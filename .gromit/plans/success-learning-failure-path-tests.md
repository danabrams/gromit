---
id: success-learning-failure-path-tests
source_spec: success-learning-failure-path-tests
created: 2026-02-26
decomposed: false
---

# Success-Learning Failure-Path Tests Implementation Plan

**Goal:** Add explicit coverage for the success-learning extraction failure branch so failed provider results are handled cleanly and observably.

**Architecture:** Extend success-learning extraction tests to exercise `result.IsSuccess()==false` and verify clean short-circuit behavior, while ensuring failure-path observability via logging callback (or explicitly documenting short-circuit-only behavior if no log is emitted).

**Tech Stack:** Go, `testing` package, existing runner escalation mocks, learnings file helper (`internal/learnings`).

**Spec:** `.gromit/specs/success-learning-failure-path-tests.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add a focused failure-path unit test for `ExtractSuccessLearning` in `internal/runner/escalation/success_learning_test.go` that drives the provider result to `Success=false` and verifies the function exits cleanly while still emitting a log callback (telemetry/logging signal).

**Key Components:**
1. **`ExtractSuccessLearning` failure branch**: exercise `result != nil && !result.IsSuccess()` path.
2. **Mock provider/result in tests**: return a `mockSuccessResult{success:false,...}`.
3. **Log capture hook (`logFn`)**: collect log calls to verify failure-path telemetry/logging behavior.

**Integration Points:**
- Modify success-learning test coverage in `internal/runner/escalation/success_learning_test.go`.
- If needed to satisfy observability expectations, perform a minimal production change in `internal/runner/escalation/learning.go` to emit a failure-path log before returning on `!result.IsSuccess()`.

**Data Flow:**
`BeadContext` is constructed in test -> `ExtractSuccessLearning` selects provider -> provider returns unsuccessful result -> function returns without panic -> assertions validate clean return and expected logging/telemetry signal (or explicitly documented short-circuit contract).

**Files to Modify:**
- `internal/runner/escalation/success_learning_test.go` - add failure-path test coverage.
- `internal/runner/escalation/learning.go` - optional minimal logging callback in the unsuccessful-result branch.

**Files to Create:**
- None.

**Tradeoffs:**
- **Test-only vs code+test**: test-only documents current short-circuit behavior; code+test enforces observability guarantees for failed extraction attempts.
- **Strict logging assertion vs permissive assertion**: strict assertion catches observability regressions but may require a production tweak; permissive assertion keeps behavior unchanged but gives weaker guarantees.

## Test Strategy

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Add a focused unit test for failed provider result handling in `ExtractSuccessLearning`.
2. **Integration Tests**: Not required; behavior is isolated and mockable.
3. **Manual Testing**: Not required.

**Key Test Cases:**
- `TestExtractSuccessLearning_FailedIteration`:
  - Construct a valid medium/high-tier `BeadContext`.
  - Router returns a provider whose result has `IsSuccess()==false`.
  - Assert the call returns without panic.
  - Assert failure-path behavior:
    - If observability path is intended: at least one `logFn` call is recorded.
    - If short-circuit-only behavior is intended: no learning persisted and this behavior is asserted/documented.
- Preserve existing success-learning tests to ensure no regressions in happy path and other skip paths.

**Mocking Strategy:**
- Reuse existing `mockSuccessRouter`, `mockSuccessProvider`, and `mockSuccessResult` helpers.
- Capture logging via an in-memory `logFn` closure.
- Use temp-dir `learnings.NewFile` to verify persistence side effects.

**Coverage Goals:**
- Cover untested branch `result != nil && !result.IsSuccess()`.
- Verify no panic and deterministic return behavior.
- Verify observability or explicit short-circuit semantics for failure path.

**Test Organization:**
- Add to `internal/runner/escalation/success_learning_test.go`.
- Use naming pattern `TestExtractSuccessLearning_FailedIteration`.
- Validate with `go test ./internal/runner/...`.

## Implementation Tasks

### Task 1: Add Failure-Path Success-Learning Test

**Files:**
- Modify: `internal/runner/escalation/success_learning_test.go`

**What to Do:**
Add `TestExtractSuccessLearning_FailedIteration` that sets up a non-low-tier bead context, routes to a mock provider, and returns a `SuccessLearningResult` where `IsSuccess()` is false. Capture log callback output and assert clean behavior without persisted learning.

**Acceptance Criteria:**
- New test invokes `ExtractSuccessLearning` with `result.IsSuccess()==false`.
- Test asserts no panic and verifies expected return-path side effects.
- Test asserts either failure-path log emission or explicitly documented short-circuit-without-log behavior.

**Dependencies:**
- None.

**Notes:**
- Prefer table-driven style only if it improves clarity; single focused case is acceptable.

### Task 2: Align Production Failure-Branch Behavior with Observability Contract

**Files:**
- Modify: `internal/runner/escalation/learning.go`
- Test: `internal/runner/escalation/success_learning_test.go`

**What to Do:**
If Task 1 exposes a mismatch between intended observability and current behavior, add a minimal `logFn` callback invocation in the `!result.IsSuccess()` branch before returning. Update test expectations to lock this behavior.

**Acceptance Criteria:**
- Failure branch remains non-panicking and non-persisting.
- Failure branch behavior is explicit and asserted by tests.
- Logging behavior for unsuccessful provider results is deterministic and covered.

**Dependencies:**
- Task 1.

**Notes:**
- Keep change minimal; do not alter tier/package-skip logic or success parsing flow.

### Task 3: Runner Escalation Quality Gate Validation

**Files:**
- Test run only: `internal/runner/...`

**What to Do:**
Run runner package tests with focus on escalation/success-learning coverage to confirm the new failure-path test passes and no regressions are introduced.

**Acceptance Criteria:**
- `go test ./internal/runner/...` passes.
- New failure-path test is included and passing.
- No unrelated production behavior changes are required beyond Task 2 (if applied).

**Dependencies:**
- Task 1.
- Task 2 (conditional, if production tweak is made).

**Notes:**
- If full package runtime is high, start with targeted escalation test invocation, then run full runner suite.

---

## Notes

This plan intentionally keeps scope narrow: verify and codify behavior for unsuccessful success-learning extraction provider results. The main decision is whether observability should include explicit failure-branch logging. Either way, the branch must be tested and documented by assertions.
