---
created: 2026-02-13T00:00:00Z
decomposed: false
id: graceful-stop
source_spec: graceful-stop
---

# Graceful Stop Implementation Plan

**Goal:** Pressing Ctrl+C during `gromit run` finishes the current bead then stops; a second Ctrl+C kills immediately.

**Architecture:** A `stopCh chan struct{}` carries the graceful-stop signal from the signal handler in `main.go` to the runner loop. First SIGINT closes the channel; second SIGINT cancels the context. The runner checks `stopCh` between iterations and breaks into the normal cleanup path.

**Tech Stack:** Go (channels, signal handling, context)

**Spec:** `.gromit/specs/graceful-stop.md`

---

## Architecture

**Overview:**
Add a two-stage Ctrl+C mechanism where the first signal closes a `stopCh` channel (graceful stop — finish current bead), and the second signal cancels the context (immediate kill). The stop channel is created in `main.go` and passed to `Run()` as a new parameter.

**Key Components:**
1. **Signal handler in `cmd/gromit/main.go`**: Replace the current single-signal goroutine with a two-stage handler. First SIGINT closes `stopCh` and prints the graceful-stop message. Second SIGINT calls `cancel()`.
2. **Stop channel check in `runner.Run()`**: New `stopCh <-chan struct{}` parameter. Checked via `select` between iterations alongside `ctx.Done()`. When fired, breaks out of loop into existing cleanup path.

**Integration Points:**
- `cmd/gromit/main.go:runLoop` — creates `stopCh`, rewrites signal goroutine, passes channel to `r.Run()`
- `internal/runner/runner.go:Run()` — adds parameter, adds `select` case in loop head
- All existing callers of `Run()` in tests — pass `nil` (nil channel never fires, preserving current behavior)

**Data Flow:**
1. User presses Ctrl+C → signal handler receives SIGINT
2. First time: closes `stopCh`, prints message. Context stays live, in-flight bead continues.
3. Runner loop finishes `processBead()`, returns to top of loop, `select` fires on `stopCh` → `break`
4. Normal cleanup path runs (stats, clean-exit marker, retro suggestion)
5. Second Ctrl+C: `cancel()` fires, context propagates, Claude process killed immediately

**Files to Modify:**
- `internal/runner/runner.go` — add `stopCh` param to `Run()`, add select case in loop
- `cmd/gromit/main.go` — rewrite signal handler, create stopCh, pass to `Run()`

**Tradeoffs:**
- Channel over atomic flag: composes with `select`, matches `ctx.Done()` pattern
- `nil` channel for backward compat: blocks forever in `select`, zero test changes needed beyond adding the parameter
- `break` over `return`: cleanup path (global stats, clean-exit, retro suggestion) runs normally

## Test Strategy

**Unit Tests** (in `internal/runner/runner_test.go`):
- `TestRun_StopChClosedBeforeStart_ExitsImmediately`: stopCh closed before Run(), assert 0 beads processed, no error
- `TestRun_StopChClosedDuringIteration_FinishesCurrentBead`: mock provider closes stopCh as side effect, assert exactly 1 bead processed, clean exit
- `TestRun_NilStopCh_RunsNormally`: nil stopCh, 2 beads available, both processed

**Mocking Strategy:**
- Existing `NewRunnerWithDeps` + mock `BeadClient`/`Router` patterns
- Mock provider closes `stopCh` as side effect for the "during iteration" test

**Manual Testing:**
- Run `gromit run`, press Ctrl+C during a bead, confirm message + bead finishes
- Press Ctrl+C twice quickly, confirm immediate kill

## Implementation Tasks

### Task 1: Add stopCh parameter to Run() and unit tests

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

**What to Do:**
Add `stopCh <-chan struct{}` as a new parameter to `Run()`, between `deadline` and `dryRun`. In the loop head, replace the simple `ctx.Done()` select with a two-case select that also checks `stopCh`. When `stopCh` fires, log "Graceful stop requested, exiting after current bead" and `break` out of the for-loop (use a labeled loop so `break` exits the outer `for`, not just the `select`). When `stopCh` is nil, the case is never selected (nil channel blocks forever).

Update all existing `r.Run(ctx, ...)` call sites across the runner test files to pass `nil` as the `stopCh` argument. This is a mechanical find-and-replace.

Add three new tests:
1. **StopChClosedBeforeStart**: Create a closed channel, pass to Run(). Mock bead client returns 1 bead. Assert Run() returns nil and the bead client's Close was never called (0 beads processed).
2. **StopChClosedDuringIteration**: Create an open channel. Mock provider closes it as a side effect during invocation. Mock bead client returns 2 beads. Assert Run() returns nil, exactly 1 bead closed.
3. **NilStopCh**: Pass nil. Mock bead client returns 2 beads then nil. Assert both beads processed and closed.

**Acceptance Criteria:**
- `Run()` accepts `stopCh <-chan struct{}` and exits the loop when the channel is closed
- Passing `nil` preserves current behavior (loop runs until no work / max iterations)
- All existing tests pass with `nil` stopCh

**Dependencies:**
- None

**Notes:**
The labeled for-loop pattern: `loop: for { select { case <-stopCh: break loop ... } }`. This is necessary because a bare `break` inside a `select` only breaks the select, not the for-loop.

### Task 2: Wire two-stage signal handler in main.go

**Files:**
- Modify: `cmd/gromit/main.go`

**What to Do:**
In `runLoop()`, replace the signal handler goroutine (lines 137-143) with a two-stage handler:

1. Create `stopCh := make(chan struct{})`.
2. In the goroutine, wait for first signal. On receive: close `stopCh`, print `Finishing current bead then stopping (Ctrl+C again to force quit)` to stderr.
3. Wait for second signal. On receive: call `cancel()`.
4. Pass `stopCh` to `r.Run(ctx, cfg.Loop.MaxIterations, deadline, stopCh, dryRun)`.

The channel buffer on `sigCh` should be 2 (not 1) to avoid dropping a second signal that arrives while the goroutine is between receives.

**Acceptance Criteria:**
- First Ctrl+C prints the graceful-stop message and does not cancel the context
- Second Ctrl+C cancels the context (same as current single-Ctrl+C behavior)
- `go build ./cmd/gromit` succeeds

**Dependencies:**
- Task 1 (Run() signature must have stopCh parameter)

---

## Notes

- The signal handler in `runRetro` (lines 194-203) is NOT changed — retro doesn't have a bead loop, so single-Ctrl+C-to-cancel is the right behavior there.
- The `stopCh` parameter position between `deadline` and `dryRun` keeps related loop-control parameters adjacent.
