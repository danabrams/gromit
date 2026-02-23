---
id: scope-first-complexity-routing
source_spec: scope-first-complexity-routing
created: 2026-02-23
decomposed: false
---

# Scope-First Complexity Routing Implementation Plan

**Goal:** Make initial build tier selection complexity-driven (from scope-check) instead of priority-driven, while preserving escalation and priority-based ordering.

**Architecture:** The Gate stage becomes the producer of one normalized effective complexity (`low|medium|high`). Build consumes that value to choose the initial tier via a dedicated complexity selector. When scope complexity is unavailable, routing falls back deterministically to `medium` with explicit observability markers.

**Tech Stack:** Go, existing runner pipeline (`internal/pipeline/*`), config accessors, prompt scope parser, JSONL iteration logging.

**Spec:** `.gromit/specs/scope-first-complexity-routing.md`

---

## Architecture

### Overview
Enforce scope-first routing in the pipeline by computing complexity before Build and wiring that complexity through stage I/O into initial tier selection. Remove priority from initial tier selection in active build paths, keep escalation as the recovery path, and emit complexity routing telemetry for tuning.

### Key Components
1. **Scope Complexity Resolver (Prepare/Gate)**
   - Resolve effective complexity before build invocation.
   - Normalize to one of `low|medium|high`.
   - Prefer scope estimate output; optionally use explicit `complexity:*` label only when scope estimate is unavailable.
   - Apply deterministic fallback `medium` and capture fallback reason.

2. **Complexity-Based Tier Selector (Config)**
   - Add a dedicated config accessor for initial tier-from-complexity mapping.
   - Keep legacy `SelectTier(priority, labels)` for compatibility in non-initial-routing call sites.
   - Support `complexity:medium` as a first-class mapping through existing label map conventions.

3. **Pipeline Complexity Plumbing**
   - Extend stage input/output to carry:
     - effective complexity
     - complexity source
     - fallback reason
     - optional scope estimate payload (for diagnostics/future tuning)
   - Gate populates these fields; Orchestrator passes them to Build.

4. **Build Initial Tier Switch**
   - Replace `SelectTier(priority, labels)` usage in Build with complexity-based selector.
   - Preserve phase override behavior (`PhaseModelTier`) and escalation chain (`NextEscalationTier`).

5. **Observability Integration**
   - Persist complexity routing data in iteration logs:
     - `complexity`
     - `complexity_source`
     - `complexity_fallback_reason`
     - existing `original_tier` and `actual_tier`
   - Ensure log output remains backward compatible (`omitempty` where appropriate).

6. **TDD Fresh-Context Alignment**
   - Ensure TDD cycle runner starts from complexity-selected initial tier rather than fallbacking to priority-driven tier selection.
   - Avoid reintroducing priority-based initial routing in fresh-context path.

### Integration Points
- **Prepare stage** in `internal/pipeline/prepare/gate.go` becomes the canonical complexity producer for run-time routing.
- **Execute stage** in `internal/pipeline/execute/build.go` consumes normalized complexity and selects initial tier accordingly.
- **Orchestrator** in `internal/runner/orchestrator.go` carries Gate complexity output to Build and iteration logs.
- **Config accessors** in `internal/config/config_accessors.go` provide a clean selector API for complexity routing.
- **Logger model** in `internal/logger/logger.go` receives added complexity routing fields for JSONL observability.

### Data Flow
1. Orchestrator calls Gate.
2. Gate computes effective complexity:
   - scope estimate complexity (preferred) -> normalized
   - else label override (`complexity:*`) -> normalized
   - else fallback `medium` with reason marker
3. Gate returns `Proceed` plus complexity metadata.
4. Orchestrator builds Build input including complexity metadata.
5. Build resolves initial tier from effective complexity, then runs invocation.
6. Escalation may raise tier on failure according to configured chain.
7. Epilogue logs original/actual tiers and complexity metadata in iteration JSONL.

### Files to Modify
- `internal/pipeline/stage.go` - add complexity-related fields on `Input` and `Output`.
- `internal/pipeline/prepare/gate.go` - compute/normalize effective complexity and emit metadata.
- `internal/pipeline/prepare/gate_test.go` - unit coverage for complexity source precedence and fallback.
- `internal/config/config_accessors.go` - add complexity-first initial tier selector.
- `internal/config/tier_selection_test.go` - coverage for complexity mapping and fallback behavior.
- `internal/pipeline/execute/build.go` - use complexity selector for initial tier; preserve escalation.
- `internal/pipeline/execute/build_test.go` - assert complexity-driven selection and priority independence.
- `internal/runner/orchestrator.go` - propagate complexity fields from Gate to Build and log payload.
- `internal/runner/orchestrator_test.go` - assert propagation and logging.
- `internal/logger/logger.go` - add complexity routing fields to iteration log schema.
- `internal/logger/iteration_log_test.go` - JSON tag/omitempty coverage for new fields.
- `internal/runner/callbacks_tdd.go` and TDD adapter tests - ensure fresh-context start tier comes from complexity routing path.

### Files to Create
- None required; changes can be implemented in existing files.

### Tradeoffs
- **Gate-produced complexity vs Build-local complexity resolution**
  - Chose Gate-produced complexity to enforce scope-first ordering structurally.
- **Dedicated complexity selector vs mutating existing `SelectTier` semantics**
  - Chose dedicated selector to avoid broad compatibility regressions and keep migration explicit.
- **Fallback `medium`**
  - Chosen per spec recommendation for deterministic neutral behavior when scope signal is missing.

## Test Strategy

### Test Levels
1. **Unit Tests**
   - Gate complexity resolver behavior, normalization, source precedence, and deterministic fallback.
   - Config complexity-to-tier selector mappings and invalid-input fallback behavior.
   - Build tier selection from complexity only (independent of bead priority when complexity fixed).

2. **Stage Integration Tests**
   - Gate output -> Orchestrator propagation -> Build input consumption.
   - Escalation behavior retained from each initial complexity tier.
   - TDD fresh-context invocation path uses complexity-derived start tier.

3. **Observability/Schema Tests**
   - Iteration log JSON fields include complexity metadata and remain backward compatible.

### Key Test Cases
- `complexity=low` -> initial tier `low`
- `complexity=medium` -> initial tier `medium`
- `complexity=high` -> initial tier `high`
- Missing scope estimate -> fallback `medium` with explicit fallback reason
- Priority permutations do not affect initial tier when effective complexity is fixed
- Escalation remains functional from each start tier (`low->medium->high`, `medium->high`, `high` terminal)
- Label override applies only when scope estimate is unavailable
- Exactly one effective complexity value is used for routing per bead

### Mocking Strategy
- Use existing fake invokers/renderers/stage doubles in pipeline tests.
- Keep real config accessor behavior in integration-style stage tests.
- Use fake log writers to assert JSON payload values without file-system coupling.

### Coverage Goals
- Critical path: scope-first complexity production and complexity-driven initial tier.
- Critical path: deterministic fallback and explicit fallback observability.
- Edge path: invalid complexity strings and absent scope output.
- Regression path: escalation behavior and priority-based ordering unaffected.

### Test Organization
- Extend focused table-driven tests in:
  - `internal/pipeline/prepare/gate_test.go`
  - `internal/pipeline/execute/build_test.go`
  - `internal/config/tier_selection_test.go`
  - `internal/runner/orchestrator_test.go`
  - `internal/logger/iteration_log_test.go`
- Keep new tests colocated with touched behavior to minimize coupling.

## Implementation Tasks

### Task 1: Add Pipeline Complexity Routing Fields

**Files:**
- Modify: `internal/pipeline/stage.go`
- Modify: `internal/pipeline/types.go` (if additional typed surface is required)
- Test: `internal/pipeline/stage_test.go` (or nearest stage type tests)

**What to Do:**
Add explicit complexity routing fields to pipeline `Input`/`Output` so Gate can emit normalized complexity metadata and Build can consume it deterministically.

**Acceptance Criteria:**
- `pipeline.Output` can carry effective complexity metadata from Gate.
- `pipeline.Input` can receive effective complexity metadata for Build.
- Type additions compile cleanly without changing stage ordering semantics.

**Dependencies:**
- None (foundational task).

**Notes:**
- Keep new fields optional/zero-safe to avoid breaking unchanged stages.

### Task 2: Implement Scope-First Complexity Resolver in Gate

**Files:**
- Modify: `internal/pipeline/prepare/gate.go`
- Test: `internal/pipeline/prepare/gate_test.go`

**What to Do:**
Add Gate logic that resolves one effective complexity value before Build. Use scope-check output as source of truth, apply label override only when scope output is unavailable, and fallback to `medium` with a reason marker.

**Acceptance Criteria:**
- Gate emits exactly one normalized complexity (`low|medium|high`) for `Proceed` decisions.
- Missing/unusable scope estimate falls back to `medium`.
- Fallback and source reason are surfaced in Gate output for observability.

**Dependencies:**
- Task 1.

**Notes:**
- Do not alter existing block/skip scope gate behavior for oversized beads.

### Task 3: Add Dedicated Complexity-to-Tier Selector

**Files:**
- Modify: `internal/config/config_accessors.go`
- Test: `internal/config/tier_selection_test.go`

**What to Do:**
Introduce a dedicated accessor that maps normalized complexity to initial tier, with deterministic fallback behavior for unknown values.

**Acceptance Criteria:**
- `low`, `medium`, `high` map to `low`, `medium`, `high` tiers respectively.
- Unknown/empty complexity maps to `medium`.
- Existing `SelectTier(priority, labels)` remains available for compatibility but is no longer required in initial build path.

**Dependencies:**
- None (can be implemented in parallel with Task 2).

**Notes:**
- Preserve legacy model-name compatibility mapping where applicable.

### Task 4: Switch Build Initial Tier Selection to Complexity

**Files:**
- Modify: `internal/pipeline/execute/build.go`
- Test: `internal/pipeline/execute/build_test.go`

**What to Do:**
Update Build stage to use complexity metadata from pipeline input and the new config complexity selector for initial tier selection. Remove priority-driven initial routing from active build path.

**Acceptance Criteria:**
- Build no longer calls priority-based `SelectTier` for initial invocation.
- Fixed complexity yields same initial tier regardless of bead priority.
- Escalation behavior remains unchanged and fully functional.

**Dependencies:**
- Task 1
- Task 3

**Notes:**
- Keep phase overrides (`PhaseModelTier`) and existing metadata population (`OriginalTier`, `ActualTier`).

### Task 5: Propagate Complexity Through Orchestrator and Logs

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Modify: `internal/logger/logger.go`
- Test: `internal/runner/orchestrator_test.go`
- Test: `internal/logger/iteration_log_test.go`

**What to Do:**
Wire Gate complexity output into Build input and persist routing telemetry in iteration log schema.

**Acceptance Criteria:**
- Orchestrator carries complexity metadata from Gate to Build.
- Iteration logs include complexity, source, fallback reason, original tier, and actual tier.
- JSON tags and omitempty behavior are validated by tests.

**Dependencies:**
- Task 1
- Task 2
- Task 4

**Notes:**
- Keep backward compatibility for historical log readers.

### Task 6: Align Fresh-Context TDD Path with Complexity-Based Initial Tier

**Files:**
- Modify: `internal/runner/callbacks_tdd.go`
- Modify: `internal/pipeline/execute/build.go` (TDD runner invocation surface as needed)
- Test: `internal/pipeline/execute/build_test.go`
- Test: `internal/runner/tdd_pipeline_adapter_test.go` (or nearest TDD adapter tests)

**What to Do:**
Ensure the TDD cycle runner starts from the complexity-selected initial tier rather than falling back to priority-based selection when tier is unset.

**Acceptance Criteria:**
- Fresh-context TDD runs begin with complexity-driven initial tier.
- No priority-based initial tier fallback remains in active fresh-context routing.
- Escalation within TDD cycle remains unchanged.

**Dependencies:**
- Task 4

**Notes:**
- This closes the last active path that can silently reintroduce priority-driven initial tiering.

### Task 7: Final Verification Against Spec Acceptance Criteria

**Files:**
- Test-only verification across modified packages.

**What to Do:**
Run targeted and package-level tests for prepare/execute/config/runner/logger areas to validate behavior and regressions.

**Acceptance Criteria:**
- Tests cover low/medium/high mapping, missing-complexity fallback, escalation from each starting tier, and priority independence in initial tier selection.
- Scope-first ordering is validated through stage wiring tests.
- No regressions in existing gate blocking/decomposition behavior or ordering semantics.

**Dependencies:**
- Tasks 1-6.

**Notes:**
- Keep verification focused on changed surfaces before broader suite execution.

---

## Notes

- Scope-check complexity is the intended routing source of truth; label override exists as fallback compatibility only when scope estimate is unavailable.
- Priority still governs ordering and workflow prioritization; this plan changes only initial model-tier routing.
- Existing escalation chain remains the reliability safety net and should not be weakened as part of this work.
