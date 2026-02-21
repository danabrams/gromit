# Fix: TDD Coverage Tracker Loops Without Progress

**Date:** 2026-02-21
**Branch:** gromit/debug-1771700141758600921
**Commit:** 8d3b761c (merged to main)

## Problem

The TDD fresh-context orchestrator repeatedly cycles without making forward progress on coverage criteria. Logs show "TDD coverage tracker reports unchecked criteria after cycle pass N; injecting additional cycles" repeating until `maxOrchestratorPasses` is exhausted, then the bead fails. Total: up to 30 wasted Claude invocations, ~60+ minutes, zero progress.

## Root Cause

Two interacting bugs in the red-passes early-exit path:

### Bug 1: Coverage never updated when red validation passes
**File:** `internal/runner/callbacks_tdd.go` (ValidateFn closure)

When the red phase writes a test and it PASSES (implementation already exists), `lastRenderedPhase == "red"`. The coverage validation guard at line ~141 checks `lastRenderedPhase != tddPhaseGreen` and early-returns, so the criterion stays `Unchecked` forever.

### Bug 2: `state.Done = true` exits inner loop after one cycle
**File:** `internal/runner/tdd/orchestrator.go` (runOneCycle)

When red validation passes, `runOneCycle` sets `state.Done = true`, which causes `RunCycles` to return after just ONE cycle per outer pass. Combined with Bug 1 (no coverage update), the outer loop sees incomplete tracker and retries forever.

## Fix Applied

### Change 1: `internal/runner/callbacks_tdd.go`
Added coverage marking BEFORE the early-return guard in ValidateFn:
```go
if passed && tracker != nil && lastRenderedPhase == tddPhaseRed && pendingCoverageCriterion != nil {
    tracker.MarkCovered(pendingCoverageCriterion.Number)
    updateIterationCoverageMetrics(activeBC.Result, tracker)
    pendingCoverageCriterion = nil
    lastRenderedPhase = ""
}
```

### Change 2: `internal/runner/tdd/orchestrator.go`
Simplified red-passes branch in `runOneCycle` — removed `state.Done = true` and `runFinalValidation`. Now just calls `AssembleCycleState` which naturally sets `Done=true` when `Remaining` is empty.

## How to Detect Recurrence

Look for these log patterns:
- "TDD coverage tracker reports unchecked criteria after cycle pass N" repeating
- Bead running 10+ TDD cycles with 0 coverage progress
- "tdd fresh-context stopped with unchecked coverage criteria" error

## How to Resume Investigation

```
Read the investigation report at /home/dabrams/gromit/.gromit/reports/debug-20260221-190000.md
and this fix report at /home/dabrams/gromit/.gromit/reports/fix-tdd-coverage-loop-20260221.md.

The fix was applied in commit 8d3b761c. Key files:
- internal/runner/callbacks_tdd.go (ValidateFn closure, coverage marking on red-pass)
- internal/runner/tdd/orchestrator.go (runOneCycle, red-passes branch)
- internal/runner/callbacks_tdd_test.go (red-pass coverage advancement tests)
- internal/runner/tdd/orchestrator_test.go (inner loop continuation tests)
```

## Tests Added

- `TestRedPassCoverageAdvancement_*` — 4 tests in callbacks_tdd_test.go
- `TestRunCycles_RedPass_ContinuesToNextCriterion` — verifies inner loop processes all criteria
- `TestRunCycles_RedPass_MixedWithFullCycles` — mixed red-pass and full TDD cycles
- `TestRunCycles_RedPass_RespectsMaxCycles` — MaxCycles still honored
- Updated 5 existing tests for new behavior (no final validation on red-pass)
