---
id: cost-optimized-routing
source_spec: cost-optimized-routing
created: 2026-02-26
decomposed: false
---

# Cost-Optimized Routing Implementation Plan

**Goal:** Add an opt-in `cost_optimized` routing strategy that defaults implementation work to low tier, uses decomposition before model escalation, and preserves existing `priority_based` behavior.

**Architecture:** Introduce strategy-aware routing config and accessors, add atomic bead classification, and wire a `DecomposeFirstHandler` that retries low-tier first, decomposes non-atomic failures, and escalates only atomic beads.

**Tech Stack:** Go, YAML config parsing/validation, existing runner/escalation pipeline, prompt templates, bd bead runtime integration.

**Spec:** `.gromit/specs/cost-optimized-routing.md`

---

## Architecture

**Overview:**
Add an additive routing mode (`routing.strategy: cost_optimized`) that keeps normal behavior as default, but changes build-time escalation logic to retry/decompose first on low tier and only escalate for atomic beads.

**Key Components:**
1. **`internal/config` strategy + policy config**: extend `RoutingConfig` with `Strategy` and `CostOptimized` fields (`build_tier`, `decompose_tier`, `escalation_tier`, `max_decomposition_depth`, `max_retries_before_decompose`) with defaults/validation.
2. **Phase-aware tier/model selection**: add strategy-aware selection helpers so build always starts low-tier under `cost_optimized`, while decompose/review/planning use `decompose_tier`.
3. **`internal/bead` atomicity helper**: add `IsAtomic(bead, depth, maxDepth)` using depth limit, `atomic:true` label, and single-target heuristics.
4. **`internal/runner/escalation/` decompose-first flow**: add `DecomposeFirstHandler` that retries low tier, then decomposes non-atomic beads, escalating only atomic beads.
5. **Runner wiring**: choose escalation handler implementation by routing strategy in constructor wiring, reusing existing decomposition/sub-bead creation paths.
6. **Decompose targeting mode**: add `decomposition.target: single_concern` and propagate to prompt context/templates to produce finer-grained beads.

**Integration Points:**
- `internal/config/` for schema/defaults/validation/accessors
- `internal/runner/escalation/` for retry/decompose/escalate policy
- `internal/runner/constructor*.go` for strategy-based handler wiring
- `internal/bead/` for atomic classification helper
- `internal/prompt/` + `.gromit/templates/PROMPT_decompose.md` for `single_concern` instructions

**Data Flow:**
1. Build starts at `build_tier` (default low) when strategy is `cost_optimized`.
2. Failures retry on same tier until `max_retries_before_decompose`.
3. On retry cap: non-atomic beads decompose and enqueue children; atomic beads escalate to `escalation_tier` chain.
4. Child beads re-enter queue and start on low tier under the same strategy.

**Tradeoffs:**
- Wrapper-style handler integration minimizes risk to existing escalation behavior.
- Atomicity heuristic is pragmatic and deterministic, with explicit `atomic:true` override.
- Routing logic remains centralized in config accessors to avoid scattered conditionals.

## Test Strategy

**Test Levels:**
1. **Unit tests** for config strategy parsing/validation, tier/model selection, atomic classification, and decompose-first decision logic.
2. **Integration tests** for constructor wiring and runtime decomposition/escalation flow in runner paths.
3. **Acceptance/regression tests** for decompose prompt `single_concern` mode and unchanged default `priority_based` behavior.

**Key Test Cases:**
- Default strategy remains `priority_based` when omitted.
- `cost_optimized` build phase always starts low tier regardless of priority.
- Retry threshold triggers decomposition for non-atomic beads.
- Atomic beads (label/depth/single-target) skip decomposition and escalate.
- Max decomposition depth boundary is enforced.
- Decompose prompt includes `single_concern` targeting guidance.
- Existing `priority_based` tests pass unchanged.
- `gromit stats` continues to surface `cost_per_spec` metric.

**Mocking Strategy:**
- Use injected `decomposeFn`, `createSubFn`, and invocation stubs in escalation tests.
- Use existing fake provider/router harnesses for constructor and pipeline integration tests.
- Keep stats tests fixture-based with no live provider dependency.

**Coverage Goals:**
- Critical branching for strategy switch and decompose-before-escalate flow.
- Edge conditions for invalid config, depth boundaries, and nil-safe defaults.
- Regression coverage ensuring default routing semantics do not change.

## Implementation Tasks

### Task 1: Add Routing Strategy and Cost-Optimized Config Schema

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `routing.strategy` with default `priority_based`, define `routing.cost_optimized` config struct and defaults, and validate tier/range fields.

**Acceptance Criteria:**
- `routing.strategy` parses with default `priority_based` when omitted.
- `routing.cost_optimized` fields parse with defaults matching spec.
- Invalid tier names or invalid retry/depth values return validation errors.

**Dependencies:**
- None

**Notes:**
- Keep additive schema changes; do not break existing routing/fallback/circuit-breaker fields.

### Task 2: Implement Strategy-Aware Tier/Model Selection Accessors

**Files:**
- Modify: `internal/config/config_accessors.go`
- Test: `internal/config/tier_selection_test.go`
- Test: `internal/config/phase_model_tier_test.go`

**What to Do:**
Add strategy-aware selection helpers that make `cost_optimized` build phase always low tier, while decompose/review/planning use configured decompose tier; preserve legacy selection behavior for `priority_based`.

**Acceptance Criteria:**
- Under `cost_optimized`, build tier selection ignores priority and returns low/build_tier.
- Under `cost_optimized`, decompose/review/planning return decompose_tier.
- Under `priority_based`, current priority+label behavior remains unchanged.

**Dependencies:**
- Task 1

**Notes:**
- Prefer introducing explicit strategy-aware methods over changing existing method contracts where possible.

### Task 3: Add Atomic Bead Classification Helper

**Files:**
- Create: `internal/bead/atomic.go`
- Test: `internal/bead/atomic_test.go`

**What to Do:**
Implement `IsAtomic(bead, depth, maxDepth)` with depth-based, label-based (`atomic:true`), and single-target heuristics aligned to spec.

**Acceptance Criteria:**
- Beads with `atomic:true` are always classified atomic.
- Beads at or beyond max depth are classified atomic.
- Single-function/method/file-target beads are classified atomic by helper heuristics.

**Dependencies:**
- Task 1

**Notes:**
- Keep heuristics deterministic and documented in tests.

### Task 4: Implement Decompose-First Escalation Handler

**Files:**
- Create: `internal/runner/escalation/decompose_first_handler.go`
- Test: `internal/runner/escalation/decompose_first_handler_test.go`
- Modify: `internal/runner/escalation/handler.go` (shared hooks only if needed)

**What to Do:**
Add a strategy-specific escalation handler that retries at low tier up to max retries, decomposes non-atomic failures, and escalates atomic failures.

**Acceptance Criteria:**
- Retries stop at `max_retries_before_decompose` for same-tier attempts.
- Non-atomic failure at retry cap invokes decomposition path and marks parent decomposed on success.
- Atomic failure at retry cap escalates tier/model instead of decomposing.

**Dependencies:**
- Task 2
- Task 3

**Notes:**
- Reuse existing typed decomposition/sub-bead callbacks to avoid duplicating creation logic.

### Task 5: Wire Strategy-Specific Handler in Runner Construction

**Files:**
- Modify: `internal/runner/constructor.go`
- Modify: `internal/runner/constructor_adapters.go`
- Test: `internal/runner/constructor_test.go`

**What to Do:**
Select default handler for `priority_based` and `DecomposeFirstHandler` for `cost_optimized`, with existing dependencies (decomposer, bead client, router) passed through.

**Acceptance Criteria:**
- `priority_based` routes through existing handler path unchanged.
- `cost_optimized` routes through decompose-first handler path.
- Runtime decomposition continues creating sub-beads and closing parent bead as today.

**Dependencies:**
- Task 4

**Notes:**
- Keep constructor API stable for existing tests and call sites.

### Task 6: Add `decomposition.target: single_concern` Config and Prompt Context

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `internal/prompt/context_types.go`
- Modify: `internal/prompt/render_methods.go`
- Test: `internal/config/decompose_config_test.go`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Introduce decomposition target mode config and propagate target mode into decompose prompt context rendering.

**Acceptance Criteria:**
- `decomposition.target` parses and defaults compatibly with existing behavior.
- `single_concern` is accepted as a valid target value.
- Prompt render context receives and exposes target mode for template logic.

**Dependencies:**
- Task 1

**Notes:**
- Maintain backward compatibility for current `narrow_scope` default semantics.

### Task 7: Update Decompose Prompt Guidance for `single_concern`

**Files:**
- Modify: `.gromit/templates/PROMPT_decompose.md`
- Test: `internal/prompt/decompose_guidelines_acceptance_test.go`

**What to Do:**
Add single-concern guidance (one function/method/test/config change per bead) while preserving existing never-split constraints and overlap checks.

**Acceptance Criteria:**
- Template explicitly distinguishes `single_concern` targeting mode guidance.
- Guidance preserves never-split natural units and overlap-prevention checks.
- Acceptance tests verify presence/ordering of updated guidance sections.

**Dependencies:**
- Task 6

**Notes:**
- Keep output format contract (`expected_outputs`, JSON array) unchanged.

### Task 8: Ensure Phase Routing Hooks Use Strategy-Aware Selection

**Files:**
- Modify: `internal/pipeline/execute/build.go`
- Modify: `internal/runner/callbacks_tdd.go`
- Modify: `internal/runner/policy/escalation.go`
- Test: `internal/pipeline/execute/build_test.go`
- Test: `internal/runner/callbacks_tdd_test.go`

**What to Do:**
Update build/TDD/refactor routing call sites to use strategy-aware phase tier resolution where needed, ensuring cost-optimized phase behavior is consistently applied.

**Acceptance Criteria:**
- Build invocations under `cost_optimized` start from configured low tier path.
- TDD phase transitions respect configured phase tier mapping without regressions.
- Existing tests for legacy behavior continue passing.

**Dependencies:**
- Task 2
- Task 5

**Notes:**
- Minimize call-site churn by using centralized config helper methods.

### Task 9: Add Cost-Optimized End-to-End Escalation/Decomposition Tests

**Files:**
- Modify: `internal/runner/escalation/handler_test.go`
- Modify: `internal/runner/escalation/scope_gate_integration_test.go`
- Modify: `internal/runner/token_efficiency_routing_test.go` (if needed for regression)

**What to Do:**
Add integration-style tests covering retry cap, decompose-first behavior, atomic escalation path, and max depth atomic enforcement.

**Acceptance Criteria:**
- Tests verify decompose-before-escalate for decomposable beads after retry cap.
- Tests verify atomic beads escalate directly after retry cap.
- Tests verify depth-limited beads are treated atomic and skip decomposition.

**Dependencies:**
- Task 4
- Task 8

**Notes:**
- Reuse existing fixture/stub patterns to keep tests deterministic.

### Task 10: Final Verification and Metrics Regression Guard

**Files:**
- Modify: `cmd/gromit/stats_test.go` (only if new assertions needed)
- Run tests: `go test ./internal/config ./internal/bead ./internal/prompt ./internal/runner/... ./internal/pipeline/... ./cmd/gromit/...`

**What to Do:**
Run focused suites and add/adjust regression assertions to ensure `cost_per_spec` remains available and default routing behavior remains unchanged.

**Acceptance Criteria:**
- Targeted test suites pass with new strategy enabled/disabled cases.
- `cost_per_spec` remains present in stats output/JSON.
- No behavioral regressions in `priority_based` routing tests.

**Dependencies:**
- Tasks 1-9

**Notes:**
- Keep verification focused on acceptance criteria and routing regressions.

---

## Notes

- This plan is additive: default behavior should remain `priority_based` unless `routing.strategy: cost_optimized` is explicitly configured.
- Existing decomposition contracts and runtime sub-bead safety checks (including partial-state protections) should be reused, not replaced.
- If atomicity heuristics are ambiguous for a bead, prefer decomposable unless explicit depth/label constraints mark it atomic.
