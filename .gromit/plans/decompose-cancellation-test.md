---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T09:15:30Z"
id: decompose-cancellation-test
source_spec: decompose-cancellation-test
---

# Decompose Cancellation-Path Test Implementation Plan

**Goal:** Add deterministic coverage for proactive decomposition cancellation handling so a pre-canceled parent context reliably produces the expected skipped-decomposition reason.

**Architecture:** Extend the proactive decomposition unit tests to directly exercise `Handler.CheckProactiveDecomposition` with a pre-canceled `ParentCtx`, asserting telemetry contract fields (especially `TimeoutDecompositionReason`) and verifying no decomposition callback execution.

**Tech Stack:** Go (`testing`, `context`, `time`), existing escalation handler/test infrastructure in `internal/runner/escalation`.

**Spec:** `.gromit/specs/decompose-cancellation-test.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add a deterministic unit test that directly exercises `Handler.CheckProactiveDecomposition` with a pre-canceled parent context, then assert telemetry fields, especially `TimeoutDecompositionReason`, reflect skipped decomposition due to parent cancellation.

**Key Components:**
1. **`CheckProactiveDecomposition` path**: Uses `firstNonNilContext(bc.ParentCtx, ctx)` and short-circuits when `Err() != nil`.
2. **Bead context test fixture**: High-risk + 60% elapsed setup to guarantee proactive decomposition branch is entered before cancellation guard.
3. **Decompose callback sentinel**: A `decomposeFn` that flips a boolean so we can assert it is never called when parent context is already canceled.

**Integration Points:**
- Extend proactive decomposition tests where behavior is currently validated.
- Reuse existing handler constructor/test setup style from current escalation tests.
- Keep assertions aligned with existing timeout decomposition telemetry contract conventions.

**Data Flow:**
Pre-canceled `ParentCtx` + eligible `BeadContext` -> `CheckProactiveDecomposition` marks attempted -> detects canceled context -> sets outcome/reason/error -> returns `false` without invoking decomposition callback.

**Files to Modify:**
- `internal/runner/escalation/proactive_decompose_test.go` - Add cancellation-path test and any needed imports/assertions.

**Files to Create:**
- None.

**Tradeoffs:**
- **Direct unit test vs full `ExecuteWithRetry` integration**: Chose direct unit test for deterministic, minimal-surface validation of the exact branch.
- **String matching `"canceled"` vs full exact message**: Prefer stable substring assertions (including `"parent context canceled"`) to avoid brittle coupling to minor text formatting while still enforcing contract intent.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Add one focused unit test for `CheckProactiveDecomposition` cancellation branch.
2. **Integration Tests**: None needed for this spec; existing `handler_test.go` already covers broader timeout/decomposition contracts.
3. **Manual Testing**: None.

**Key Test Cases:**
- `TestCheckProactiveDecomposition_CanceledParentContextSkipsDecomposition`:
  - Uses `context.WithCancel`, cancels before invocation.
  - Sets bead as high-risk and >=60% elapsed so proactive path is entered.
  - Asserts return value `false` (loop stops after attempted proactive check).
  - Asserts `decomposeFn` not called.
  - Asserts `TimeoutDecompositionAttempted == true`.
  - Asserts `TimeoutDecompositionOutcome == "skipped"`.
  - Asserts `TimeoutDecompositionSucceeded == false`.
  - Asserts `TimeoutDecompositionReason` contains `"parent context canceled"` and `"canceled"`.

**Mocking Strategy:**
- Use lightweight function stubs (`decomposeFn`, `createSubFn`), no external mocks.
- Real `Handler` instance with default config to preserve production logic.

**Coverage Goals:**
- Deterministic coverage of pre-canceled parent-context path.
- No sleep, no timing races; use fixed elapsed setup via `BeadStartTime`.
- Ensure no `t.Skip` placeholders for this case.

**Test Organization:**
- Add test in `internal/runner/escalation/proactive_decompose_test.go`.
- Follow existing naming style `TestCheckProactiveDecomposition_*`.

## Implementation Tasks

### Task 1: Add deterministic proactive-decomposition cancellation test

**Files:**
- Modify: `internal/runner/escalation/proactive_decompose_test.go`

**What to Do:**
Add `TestCheckProactiveDecomposition_CanceledParentContextSkipsDecomposition` that constructs a high-risk bead context at the proactive threshold, supplies a parent context canceled via `context.WithCancel`, calls `CheckProactiveDecomposition`, and verifies skip behavior and telemetry fields.

**Acceptance Criteria:**
- Test uses `context.WithCancel`, cancels before invocation, and enters proactive decomposition branch deterministically.
- Test asserts decomposition callback is not invoked and `CheckProactiveDecomposition` returns `false`.
- Test asserts `TimeoutDecompositionReason` includes `"parent context canceled"`/`"canceled"` and outcome is `"skipped"`.

**Dependencies:**
- None.

**Notes:**
Keep assertions substring-based for reason text to avoid brittleness while preserving behavioral contract.

### Task 2: Remove/replace any skip-based placeholder for this case

**Files:**
- Modify: `internal/runner/escalation/proactive_decompose_test.go` (if applicable)

**What to Do:**
Search for any existing `t.Skip` placeholder tied to proactive decomposition cancellation path and remove or replace it with the deterministic test coverage added in Task 1.

**Acceptance Criteria:**
- No `t.Skip` remains for proactive decomposition cancellation-path coverage.
- Coverage intent previously deferred by placeholder is now represented by executable assertions.

**Dependencies:**
- Task 1.

**Notes:**
If no placeholder exists, task is satisfied by explicit verification and no-op documentation in implementation PR notes.

### Task 3: Validate deterministic behavior under repetition

**Files:**
- Test: `internal/runner/escalation/proactive_decompose_test.go`

**What to Do:**
Run repeated tests for escalation runner package to confirm deterministic behavior and absence of timing flakes.

**Acceptance Criteria:**
- `go test -count=5 ./internal/runner/...` passes.
- New cancellation-path test remains deterministic across repeated runs.

**Dependencies:**
- Task 1.

**Notes:**
No sleep-based assertions are allowed; failures should indicate contract regressions, not timing instability.

---

## Notes

- This plan intentionally targets `internal/runner/escalation/proactive_decompose_test.go` (not `internal/runner/proactive_decompose_test.go`) because the implementation under test lives in the escalation package.
- Existing broad contract tests in `internal/runner/escalation/handler_test.go` already cover related skipped-decomposition behavior; this plan adds a focused, deterministic unit test for the proactive path specifically.
