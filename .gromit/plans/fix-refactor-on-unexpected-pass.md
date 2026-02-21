---
id: fix-refactor-on-unexpected-pass
spec: internal-debug
created: 2026-02-21
decomposed: false
---

# Fix: Skip refactor when RED phase passes unexpectedly

## Problem

When TDD is in fresh-context-per-cycle mode and the RED phase validation passes unexpectedly (implementation already exists), `runRefactorAndFinalValidation` is called. The refactor computes a diff anchored to `bc.StartCommit`, which includes test files written by the RED invocation. The "no changes" guard in `RunRefactorPhaseWithResult` does not fire, so a full refactor invocation is made. This invocation fails (exit non-zero / Success=false) because there is nothing to refactor when the implementation was pre-existing.

Reference: `.gromit/reports/debug-20260221-152000.md`

## Architecture

The fix belongs in `internal/runner/tdd/orchestrator.go`. The `runOneCycle` method has two branches after RED validation:
1. `passed == true` (unexpected pass) → currently calls `runRefactorAndFinalValidation`
2. `passed == false` (expected: tests fail) → runs GREEN phase, then `runRefactorAndFinalValidation`

When `passed == true`, the cycle completed without writing any implementation code. The correct behavior is to skip the refactor and go straight to final validation (or just mark done).

## Tasks

### Task 1: Skip refactor in unexpected-pass branch
**File**: `internal/runner/tdd/orchestrator.go`
**Change**: In `runOneCycle`, when `passed == true` after RED validation, replace `runRefactorAndFinalValidation` with just `runFinalValidation`. There is no implementation code to refactor when the RED phase tests unexpectedly passed.

Before:
```go
if passed {
    // Tests pass unexpectedly — nothing left to implement
    if err := o.runRefactorAndFinalValidation(ctx, bc); err != nil {
        return err
    }
    state.Done = true
    *state = AssembleCycleState(*state, "")
    return nil
}
```

After:
```go
if passed {
    // Tests pass unexpectedly — nothing left to implement.
    // Skip refactor: no implementation was written in this cycle.
    if err := o.runFinalValidation(ctx); err != nil {
        return err
    }
    state.Done = true
    *state = AssembleCycleState(*state, "")
    return nil
}
```

### Task 2: Update tests
**File**: `internal/runner/tdd/orchestrator_test.go`
Find tests that cover the "unexpected pass" branch and update them to expect no refactor call. Also add a test that verifies refactor is NOT called when RED passes unexpectedly.

## Testing Strategy

- Run `go test -vet=off ./internal/runner/tdd/...` to verify orchestrator tests pass
- Run `go test -vet=off ./internal/runner/...` to check runner tests pass
- Run `go build ./...` to verify compilation
