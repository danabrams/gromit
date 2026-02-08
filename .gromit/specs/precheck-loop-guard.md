---
id: precheck-loop-guard
source_ideas: []
created: 2026-02-07
---

# Precheck Loop Guard

## Specification

When `runPrecheck` returns true (acceptance criteria already met) and the subsequent `beads.Close()` call fails, the runner loop can spin indefinitely on the same bead. The iteration counter is never incremented for precheck skips, so `maxIterations` never triggers. Stuck-bead detection only tracks build failures, not precheck skips. The result is an infinite loop that only stops on Ctrl+C or a `--time-budget` deadline.

This feature adds two complementary layers of defense:

### Layer 1: Add failed-close beads to `skippedBeads`

When `Close()` fails after a precheck pass, immediately add the bead ID to the existing `skippedBeads` map. On the next loop iteration, if `Ready()` returns the same bead and it's already in `skippedBeads`, the existing "all ready beads are stuck" logic kicks in and the loop terminates cleanly. This catches the most common failure mode (Close error) on the second spin.

### Layer 2: Consecutive precheck skip limit

Track a counter of consecutive precheck skips (skips without any real build work in between). When the counter reaches `maxConsecutiveSkips` (default: 3), stop the loop with a clear error message. Any real build iteration (a bead that goes through the full build process) resets the counter to zero.

This catches the subtler case where `Close()` appears to succeed (returns nil) but doesn't actually work — `Ready()` keeps returning the same bead, and `skippedBeads` never fires because no error was returned.

### Configuration

A new field `max_consecutive_skips` is added to the `loop` section of `gromit.yaml`, alongside the existing `stuck_bead_threshold`:

```yaml
loop:
  max_iterations: 20
  stuck_bead_threshold: 3
  max_consecutive_skips: 3  # new
```

When the limit is hit, the runner stops with a non-zero exit and logs an error message explaining what happened.

## Acceptance Criteria

- When `Close()` fails after a precheck pass, the bead is added to `skippedBeads` and the loop does not revisit it
- When `maxConsecutiveSkips` consecutive precheck skips occur without any real build work, the loop stops with a clear error message
- A successful build iteration resets the consecutive skip counter to zero
- `max_consecutive_skips` defaults to 3 when not set in config
- `max_consecutive_skips` is configurable in `gromit.yaml` under `loop`

## Decisions

1. **Two layers of defense rather than one.** The `skippedBeads` fix handles the obvious case (Close returns an error) quickly — on the 2nd spin. The consecutive-skip limit handles the subtle case (Close returns nil but doesn't work) with a slightly longer fuse. Either alone would leave a gap.

2. **Precheck skips still do not count toward `maxIterations`.** The original design intent — that clearing already-done beads shouldn't consume your iteration budget — is preserved. The consecutive-skip limit is a separate, purpose-built safety valve.

3. **Stop with error, not silent exit.** When the consecutive-skip limit is hit, the runner exits with a non-zero code and a descriptive error. This is an abnormal condition (something is wrong with `bd` or the filesystem), not a graceful "no work left" situation.

4. **Default of 3 mirrors `stuck_bead_threshold`.** Consistent with the existing loop-safety default and generous enough to avoid false positives from a few legitimately pre-satisfied beads.

## Research & Context

### Current State

The main loop lives in `internal/runner/runner.go`. Key locations:

- **Iteration counter**: initialized at line 229, incremented at line 358 (after the precheck skip path)
- **`skippedBeads` map**: initialized at line 239, used for stuck-bead detection at lines 297-318
- **Precheck skip path**: lines 320-351 — Close/Sync failures are logged as warnings, then `continue` skips to the next loop iteration
- **Comment on line 336**: `"Note: we don't increment iteration counter for skipped beads"` — confirms the intentional non-increment

The `LoopConfig` struct is in `internal/config/config.go` at line 46. It already has `StuckBeadThreshold` (default 3, set at line 229-230) as a precedent for the new field.

### Existing Test Coverage

- `TestRunWithMocks_PrecheckPassed` (interfaces_test.go) tests the happy path where Close succeeds
- `TestRunWithMocks_PrecheckDoesNotCountAsIteration` validates that precheck skips don't increment the counter — the property that enables the bug
- Neither test covers Close failure after precheck pass

### Implementation Surface

Changes are confined to three files:
- `internal/config/config.go` — add `MaxConsecutiveSkips` field to `LoopConfig`, add default in `applyDefaults`
- `internal/runner/runner.go` — add consecutive skip counter, add bead to `skippedBeads` on Close failure, check limit
- Test files for both packages
