---
id: phase-3-token-efficiency-cache-and-tiering
source_spec: phase-3-token-efficiency-cache-and-tiering
created: 2026-02-24
decomposed: true
decomposed_at: "2026-02-24T11:05:00Z"
---

# Token Efficiency Cache and Tiering (Phase 3) Implementation Plan

**Goal:** Implement deterministic prompt-prefix caching and utility-task tier routing to reduce repeated-workload tokens/cost while preserving run correctness and success rates.

**Architecture:** Add a provider-agnostic cache optimization layer driven by deterministic prompt preamble keys and versioned invalidation, plus explicit task-category utility routing to lower tiers with kill switches and fallback.

**Tech Stack:** Go, existing gromit config/runner/prompt/provider/logger packages, YAML config, JSONL telemetry artifacts.

**Spec:** `.gromit/specs/phase-3-token-efficiency-cache-and-tiering.md`

---

## Architecture

**Overview:**
Implement Phase 3 as a reversible extension to current prompt rendering, provider invocation, and telemetry: deterministic cache-key generation for static preambles, provider-aware cache lifecycle hooks, and task-category utility routing defaults.

**Key Components:**
1. **Prompt Canonicalization + Cache Keying (`internal/prompt`)**: Build deterministic static preamble serialization and stable cache keys for cacheable prompt classes.
2. **Token Efficiency Config (`internal/config`)**: Add `token_efficiency.cache` and `token_efficiency.routing` config blocks with default-off behavior and explicit kill switches.
3. **Provider Cache Adapter (`internal/provider`)**: Introduce provider-aware cache lifecycle methods (reuse/write/invalidate/no-op fallback) behind a common interface.
4. **Invocation Wiring (`internal/runner/execution`)**: Call cache lifecycle around provider invocation as optimization-only behavior (never correctness-critical).
5. **Telemetry Extensions (`internal/runner/runtypes`, `internal/logger`)**: Persist cache hit/miss/write/invalidation and routing decision metadata per iteration.
6. **Utility Routing Resolver (`internal/runner`)**: Map utility task categories to tier/model using config, while preserving high-fidelity build/edit execution defaults.

**Integration Points:**
- Prompt preamble and diagnostics computation in `internal/prompt/prompt.go`, `internal/prompt/render_methods.go`, and `internal/prompt/renderer_context.go`.
- Invocation and provider selection flow in `internal/runner/execution/invoker.go`.
- Provider capability wiring in `internal/provider` (interface and concrete providers).
- Config parsing/default/accessor flow in `internal/config`.
- Iteration log and runtypes schema in `internal/logger/logger.go` and `internal/runner/runtypes/types.go`.
- Utility-tier call sites in `internal/runner/process_methodology.go` and related helper paths.

**Data Flow:**
1. Render path computes canonical static preamble representation and deterministic cache key.
2. Invoker checks cache adapter with version keys and prompt class metadata.
3. Provider call executes with cache optimization when available; uncached fallback always works.
4. Adapter records hit/miss/write/invalidation outcomes.
5. Iteration result/log includes cache and routing metadata for before/after analysis.

**Files to Modify:**
- `internal/config/config_types.go`
- `internal/config/config_defaults.go`
- `internal/config/config_accessors.go`
- `internal/prompt/prompt.go`
- `internal/prompt/render_methods.go`
- `internal/prompt/renderer_context.go`
- `internal/provider/provider.go`
- `internal/provider/claude.go`
- `internal/provider/codex.go`
- `internal/runner/execution/invoker.go`
- `internal/runner/process_methodology.go`
- `internal/runner/runtypes/types.go`
- `internal/logger/logger.go`

**Files to Create:**
- `internal/prompt/cache_key.go`
- `internal/prompt/cache_key_test.go`
- `internal/provider/cache_adapter.go`
- `internal/provider/cache_adapter_test.go`
- `internal/runner/token_efficiency_routing.go`
- `internal/runner/token_efficiency_routing_test.go`

**Tradeoffs:**
- **Deterministic canonicalization over ad-hoc hashing:** avoids key churn from non-semantic ordering differences.
- **Adapter abstraction over provider-specific direct wiring:** enables no-op fallback and uniform telemetry.
- **Task-category routing over ad-hoc per-run decisions:** keeps behavior explainable, configurable, and safe to roll back.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: canonical key stability, invalidation trigger logic, config defaults/accessors, utility routing mapping.
2. **Integration Tests**: invoker + router/provider cache lifecycle behavior and telemetry emission under hit/miss/failure scenarios.
3. **Manual Benchmark Validation**: fixed workload A/B protocol with repeated runs and median comparison.

**Key Test Cases:**
- Equivalent static preambles with different field ordering generate identical cache keys.
- Key inputs version bump (rules/template/tool schema) triggers invalidation.
- Cache adapter unavailable/failing path continues run uncached without erroring the iteration.
- Cache hit/miss/write metadata is emitted in iteration logs.
- Utility categories (`summarization`, `masking_transform`, `discovery_indexing`) route to configured utility tier.
- Build/code-generation paths remain on existing tier selection unless explicit override is set.
- Kill switches independently disable cache and routing behavior.

**Mocking Strategy:**
- Use fake cache adapters for deterministic lifecycle assertions.
- Use fake provider/router for routing call-site tests.
- Reuse existing config parsing tests for backward compatibility and default-preservation verification.

**Coverage Goals:**
- Critical paths: deterministic cache keying, invalidation correctness, non-fatal fallback, routing map enforcement.
- Edge paths: unknown categories, empty preamble, nil config, provider capability mismatch.

**Test Organization:**
- New tests in cache/routing modules (`internal/prompt`, `internal/provider`, `internal/runner`).
- Targeted updates to existing suites:
  - `internal/runner/execution/invoker_test.go`
  - `internal/config/*_test.go`
  - `internal/logger/logger_test.go`
  - `internal/runner/runtypes/types_test.go`
  - `internal/runner/process_methodology_test.go`

---

## Implementation Tasks

### Task 1: Add Token-Efficiency Config Surface and Defaults

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `internal/config/config_accessors.go`
- Test: `internal/config/config_test.go` and focused config tests

**What to Do:**
Add config blocks and accessors for:
- `token_efficiency.cache.enabled`
- `token_efficiency.cache.ttl`
- `token_efficiency.cache.capacity`
- `token_efficiency.routing.utility_tier`
- task override map and kill switches

Defaults must preserve current behavior (disabled/off unless explicitly enabled).

**Acceptance Criteria:**
- New token-efficiency keys parse correctly from YAML.
- Defaults keep current execution/routing behavior unchanged.
- Independent kill switches exist for cache and routing.

**Dependencies:**
- None

**Notes:**
- Keep naming compatible with current config conventions and backward compatibility tests.

### Task 2: Implement Deterministic Prompt Preamble Keying

**Files:**
- Create: `internal/prompt/cache_key.go`
- Create: `internal/prompt/cache_key_test.go`
- Modify: `internal/prompt/render_methods.go`
- Modify: `internal/prompt/renderer_context.go`

**What to Do:**
Implement canonical serialization and key generation for cache-stable prompt preambles (system/rules/tool/static template parts first, dynamic sections excluded or clearly separated).

**Acceptance Criteria:**
- Equivalent prompts with non-semantic ordering differences produce identical keys.
- Static preamble classes and key metadata are available to invocation path.
- Key generation is deterministic across repeated runs.

**Dependencies:**
- Task 1

**Notes:**
- Avoid map iteration nondeterminism and preserve stable section ordering.

### Task 3: Add Provider Cache Adapter Abstraction and Capability Wiring

**Files:**
- Create: `internal/provider/cache_adapter.go`
- Create: `internal/provider/cache_adapter_test.go`
- Modify: `internal/provider/provider.go`
- Modify: `internal/provider/claude.go`
- Modify: `internal/provider/codex.go`

**What to Do:**
Define a cache adapter interface and wire provider-specific/no-op implementations that support:
- cache lookup/reuse
- cache write/refresh
- explicit invalidation via version keys
- TTL/capacity policy parameters

**Acceptance Criteria:**
- Adapter API supports hit/miss/write/invalidate semantics.
- Unsupported providers degrade cleanly to no-op cache behavior.
- Unit tests cover capability and no-op fallback behavior.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
- This layer must remain optimization-only and never required for correctness.

### Task 4: Wire Cache Lifecycle into Invocation Flow with Fallback Guardrails

**Files:**
- Modify: `internal/runner/execution/invoker.go`
- Test: `internal/runner/execution/invoker_test.go`

**What to Do:**
Integrate cache adapter lifecycle into invocation:
- pre-invocation lookup/reuse
- post-invocation write/update
- invalidation on configured version-key changes
- robust fallback on cache unavailability/errors

**Acceptance Criteria:**
- Cache failures do not fail the run.
- Invocation continues uncached when adapter errors.
- Lifecycle events are observable for telemetry wiring.

**Dependencies:**
- Task 3

**Notes:**
- Keep existing provider routing and usage-limit behavior intact.

### Task 5: Add Utility Task Category Routing Resolver

**Files:**
- Create: `internal/runner/token_efficiency_routing.go`
- Create: `internal/runner/token_efficiency_routing_test.go`
- Modify: `internal/runner/process_methodology.go`
- Modify: `internal/runner/callbacks_tdd.go` (if needed by call path)

**What to Do:**
Add explicit category-based routing for utility tasks (default to configured utility tier/model), including task-level overrides and kill switch behavior.

**Acceptance Criteria:**
- Utility categories route to low-cost tier by default when enabled.
- Complex execution/editing/build paths retain existing high-fidelity routing.
- Routing can be reverted via kill switch without code changes.

**Dependencies:**
- Task 1

**Notes:**
- Keep category taxonomy explicit and auditable in code/tests.

### Task 6: Extend Runtypes/Logger Schema for Cache and Routing Telemetry

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/logger/logger.go`
- Test: `internal/runner/runtypes/types_test.go`
- Test: `internal/logger/logger_test.go`

**What to Do:**
Add per-iteration fields for cache and routing outcomes (hit/miss/write, cache class/key metadata, invalidation reason/version marker, utility routing category/tier used).

**Acceptance Criteria:**
- JSONL logs include cache/routing fields when populated.
- Optional fields are omitted cleanly when disabled or unused.
- Existing consumers remain compatible with additive schema changes.

**Dependencies:**
- Task 4
- Task 5

**Notes:**
- Follow existing JSON tag patterns and omitempty semantics.

### Task 7: Add End-to-End Acceptance Coverage for Phase 3 Behavior

**Files:**
- Modify: targeted acceptance/integration tests in `internal/runner/acceptance/` and/or provider/runner integration tests

**What to Do:**
Add end-to-end test coverage for:
- deterministic cache-key stability
- non-fatal cache fallback
- utility routing category behavior
- unchanged build/codegen routing behavior

**Acceptance Criteria:**
- Phase 3 guardrail behaviors are exercised by integration-level tests.
- Regression tests fail on routing drift or cache correctness regressions.

**Dependencies:**
- Task 2
- Task 4
- Task 5
- Task 6

**Notes:**
- Keep acceptance scope focused; avoid broad unrelated suite expansion.

### Task 8: Run Measurement Protocol and Record Before/After Outcomes

**Files:**
- Modify: benchmark/report notes artifact location as applicable (`.gromit/reports/` or benchmark output path)

**What to Do:**
Execute fixed-workload measurement protocol:
- >=3 baseline runs (feature disabled)
- >=3 optimized runs (feature enabled)
- compare medians for cache hit rate, input token reduction, cost reduction, success/validation rates
- validate rollback/kill-switch behavior

**Acceptance Criteria:**
- Reported metrics include cache hit rate by prompt class.
- Token and cost median deltas are documented.
- No material success-rate regression is observed, or rollback trigger is documented.

**Dependencies:**
- Task 6
- Task 7

**Notes:**
- Use the same workload set for baseline and optimized runs.

---

## Notes

- Caching is optimization-only and must never become a correctness dependency.
- Versioned invalidation keys should include at minimum rules/template/tool-schema versions.
- Preserve progressive rollout semantics: shadow/observe first, then enforce.
- Keep implementation additive where possible to minimize risk to existing workflows.
