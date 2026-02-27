---
created: 2026-02-27T00:00:00Z
decomposed: true
decomposed_at: "2026-02-27T15:57:09Z"
id: event-emission-ownership
source_spec: event-emission-ownership
---

# Event Emission Ownership Implementation Plan

**Goal:** Make phase events stage-owned (no orchestrator duplicates), wire missing event types, and fix event registry + prompt budget tests.

**Architecture:** Orchestrator emits only lifecycle events; each stage emits its own phase events with correct context and timestamps. Gate/Epilogue wire previously unused event types.

**Tech Stack:** Go, internal events/emitter system, pipeline stages, tests.

**Spec:** `.gromit/specs/event-emission-ownership.md`

---

## Architecture

**Overview:**
Shift phase-level event ownership into their stages (Build/Validate/Review/Gate/Epilogue), leaving orchestrator to emit only lifecycle events; wire missing event types and align tests.

**Key Components:**
1. **Orchestrator (internal/runner/orchestrator.go)**: Remove duplicate phase event emissions for Build/Validate/Review; keep lifecycle events.
2. **Stage Emitters (Build/Validate/Review/Gate/Epilogue)**: Ensure each stage emits its own phase events with `Time`, and includes correct fields (e.g., Review `Thorough`).
3. **Event Registry Tests (internal/events/types_test.go)**: Register new event types in type checks, uniqueness checks, and spec cases.
4. **Prompt Budget (internal/prompt/rules_phase_budget_test.go / RULES)**: Adjust budget or trim rules to resolve build-phase overage.

**Integration Points:**
- The emitter already travels in `pipeline.Input`; stages emit via `in.Emitter`.
- Orchestrator remains the only owner of run/iteration/bead lifecycle events.

**Data Flow:**
- Orchestrator builds `pipeline.Input` with `Emitter` and calls stage `Run`.
- Stage emits its start event, executes work, emits pass/fail/complete events with actual metrics, then returns `pipeline.Output`.
- Orchestrator records lifecycle results and calls Epilogue, which emits Epilogue and cleanup-related events.

**Files to Modify:**
- `internal/runner/orchestrator.go` - remove Build/Validation/Review phase event emissions.
- `internal/pipeline/execute/build.go` - add `Time` to BuildStart/Complete events.
- `internal/pipeline/review/review.go` - add `Time`, set `Thorough` on ReviewStart.
- `internal/pipeline/validate/validate.go` - emit ValidationStart/Pass/Fail.
- `internal/pipeline/validate/validate_test.go` - add Validate event emission tests.
- `internal/pipeline/prepare/gate.go` - emit `GateScopeEvent` for oversized bead decisions.
- `internal/pipeline/prepare/gate_test.go` - add GateScope event tests.
- `internal/pipeline/epilogue/epilogue.go` - emit `BeadCloseEvent` and `BeadCleanupEvent`.
- `internal/pipeline/epilogue/epilogue_test.go` - add BeadClose/BeadCleanup event tests.
- `internal/events/types_test.go` - include 8 new event types, update unique count, and add spec cases.
- `internal/prompt/rules_phase_budget_test.go` or `.gromit/RULES.md` - fix build rules budget overage.

**Tradeoffs:**
- **Wire vs remove dead event types:** Wiring preserves telemetry; removing reduces surface area. We will wire them.
- **Increase budget vs trim rules:** Increase is minimal risk but grows prompt size; trimming keeps budgets tight but risks losing content. Decide based on prompt budget philosophy.

## Test Strategy

**Test Levels:**
1. **Unit Tests**:
- Stage-level emission tests in `internal/pipeline/execute/build_test.go`, `internal/pipeline/review/review_test.go`, `internal/pipeline/validate/validate_test.go`, `internal/pipeline/prepare/gate_test.go`, `internal/pipeline/epilogue/epilogue_test.go`.
- Event registry tests in `internal/events/types_test.go`.
- Prompt budget test in `internal/prompt/rules_phase_budget_test.go`.

2. **Integration Tests**:
- Runner package tests where event ordering or emission contracts are asserted, especially `internal/runner/...` if they assumed orchestrator-emitted phase events.

3. **Manual Testing**:
- Not required; automated tests cover emission and registry behavior.

**Key Test Cases:**
- Validate stage emits `ValidationStartEvent` and `ValidationPassEvent` on success, `ValidationFailEvent` on failure.
- Gate emits `GateScopeEvent` with `Action` reflecting block vs decompose path.
- Epilogue emits `BeadCloseEvent` on close success and `BeadCleanupEvent` for `sync`, `merge`, and `worktree_cleanup` actions.
- `types_test.go` includes 8 new events in interface checks and uniqueness count.
- Prompt budget test passes for build phase after budget/rules adjustment.

**Mocking Strategy:**
- Use existing fake runners and emitters; tests subscribe to `events.Emitter` channels.
- For epilogue, reuse existing fake bead lifecycle/status/worktree fakes and assert emitted events.

**Coverage Goals:**
- Each newly wired event is emitted in its intended path.
- Orchestrator no longer emits stage-level events (tests updated where needed).
- Event registry fully includes new event types.

**Test Organization:**
- Tests live alongside existing stage tests (`*_test.go`) and event registry tests in `internal/events/types_test.go`.

---

## Implementation Tasks

### Task 1: Remove Orchestrator Phase Emissions and Align Stage Events

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Modify: `internal/pipeline/execute/build.go`
- Modify: `internal/pipeline/review/review.go`

**What to Do:**
- Remove orchestrator emissions for BuildStart/Complete and ReviewStart/Complete (and any associated test assertions).
- Add `Time: time.Now()` to BuildStart/Complete emissions in build stage.
- Add `Time: time.Now()` and `Thorough` to ReviewStart emission; add `Time` to ReviewComplete emission.

**Acceptance Criteria:**
- Orchestrator does not emit Build or Review phase events.
- Build and Review stage emissions include `Time` and correct fields.
- `go test ./internal/runner/... ./internal/pipeline/execute/... ./internal/pipeline/review/...` passes.

**Dependencies:**
- None.

**Notes:**
- Ensure Review `Thorough` matches config or the intended review mode (likely false for standard review).

---

### Task 2: Move Validation Events Into Validate Stage

**Files:**
- Modify: `internal/pipeline/validate/validate.go`
- Modify: `internal/pipeline/validate/validate_test.go`
- Modify: `internal/runner/orchestrator.go`

**What to Do:**
- Emit ValidationStart before running commands (include commands list and `Time`).
- Emit ValidationPass or ValidationFail after completion (include `Time`, duration if available).
- Remove ValidationStart/Pass/Fail emissions from orchestrator.
- Add tests in validate stage for emission behavior.

**Acceptance Criteria:**
- Validation events are emitted from Validate stage only.
- Orchestrator no longer emits validation events.
- Validate stage tests assert emission behavior.
- `go test ./internal/pipeline/validate/... ./internal/runner/...` passes.

**Dependencies:**
- Task 1 (for orchestrator event removal patterns).

**Notes:**
- Duration can be zero initially if not tracked; keep consistent with existing TODOs.

---

### Task 3: Wire GateScope and Epilogue Close/Cleanup Events

**Files:**
- Modify: `internal/pipeline/prepare/gate.go`
- Modify: `internal/pipeline/prepare/gate_test.go`
- Modify: `internal/pipeline/epilogue/epilogue.go`
- Modify: `internal/pipeline/epilogue/epilogue_test.go`

**What to Do:**
- Emit `GateScopeEvent` when scope gate detects oversized bead before deciding block/decompose.
- Emit `BeadCloseEvent` when close succeeds.
- Emit `BeadCleanupEvent` for each cleanup action (`sync`, `merge`, `worktree_cleanup`).
- Add unit tests for each emission point.

**Acceptance Criteria:**
- GateScopeEvent emitted with file counts and action for oversized beads.
- BeadCloseEvent emitted on successful close.
- BeadCleanupEvent emitted for each cleanup action.
- `go test ./internal/pipeline/prepare/... ./internal/pipeline/epilogue/...` passes.

**Dependencies:**
- None.

**Notes:**
- Decide action values for GateScopeEvent: `block` vs `decompose`.

---

### Task 4: Update Event Registry Tests for New Event Types

**Files:**
- Modify: `internal/events/types_test.go`

**What to Do:**
- Add 8 new event types to TestAllEventTypesImplementEvent.
- Add 8 new event types to TestEventTypeStringsAreUnique and update count to 35.
- Add 8 new event types to `specEventCases` with correct EventType strings and `Time` handling.

**Acceptance Criteria:**
- All new event types included in registry tests.
- Unique count updated to 35.
- `go test ./internal/events/...` passes.

**Dependencies:**
- Task 3 (event types emitted; ensure any field shape changes are reflected).

---

### Task 5: Fix Build Rules Char Budget Overages

**Files:**
- Modify: `internal/prompt/rules_phase_budget_test.go` or `.gromit/RULES.md`

**What to Do:**
- Adjust build-phase rules budget or trim rules content to resolve 60-char overage.

**Acceptance Criteria:**
- Build phase rules budget test passes.
- No other rules phase budgets regress.
- `go test ./internal/prompt/...` passes.

**Dependencies:**
- None.

**Notes:**
- Prefer minimal budget increase unless rules policy prefers trimming.

---

## Notes

- Stage-level events should include `Time` consistently for ordering and downstream consumers.
- Update any orchestrator tests that assumed phase events were emitted there.
- Keep event naming consistent with existing `EventType()` strings.
