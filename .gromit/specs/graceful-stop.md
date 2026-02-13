---
id: graceful-stop
source_ideas: []
created: 2026-02-13
---

# Graceful Stop: Two-Stage Ctrl+C

## Specification

Pressing Ctrl+C during `gromit run` stops the loop after the current bead finishes. A second Ctrl+C forces an immediate stop. This replaces the current behavior where the first Ctrl+C cancels the context and interrupts the in-flight Claude invocation.

A `stopCh chan struct{}` carries the graceful stop signal from the signal handler in `main.go` to the runner loop. The channel decouples signal handling (CLI concern) from loop control (runner concern), keeping OS signals out of the runner package and making the behavior easy to test.

**First Ctrl+C:** Closes `stopCh`. Prints `Finishing current bead then stopping (Ctrl+C again to force quit)` to stderr. The context stays live, so the in-flight bead runs to completion — build, validate, review, and all phases.

**Second Ctrl+C:** Cancels the context. Kills the in-flight Claude process immediately (current behavior).

## Acceptance Criteria

- First Ctrl+C during a running bead prints the graceful-stop message and lets the bead finish
- After the bead finishes, the loop exits through the normal cleanup path (stats, clean-exit marker, retro suggestion)
- Second Ctrl+C cancels the context and stops immediately
- When no Ctrl+C is pressed, behavior is identical to today

## Decisions

1. **Two-stage Ctrl+C, not a separate signal** — Users already reach for Ctrl+C. SIGUSR1 requires a second terminal and knowledge of the PID. A sentinel file requires manual cleanup. A stdin keypress listener conflicts with Claude's terminal usage.

2. **Stop channel, not atomic flag** — A channel integrates naturally with Go's `select` statement and matches the existing `ctx.Done()` pattern in the loop. It also prevents the runner from needing a reference to the signal handler's state.

3. **Signal handling stays in main.go** — The runner shouldn't know about OS signals. The channel abstraction lets tests close `stopCh` directly without signal machinery.

4. **Finish the entire bead, not just the current phase** — Stopping between phases (e.g., after build but before validate) leaves the bead in an ambiguous state. Completing all phases ensures the bead is either fully done or fully not started.

## Research & Context

### Current State

The signal handler lives in `cmd/gromit/main.go:134-143`. It listens for one SIGINT/SIGTERM, prints a message, and calls `cancel()` on the context. This cancellation propagates through the entire context chain — including the per-bead timeout context — so it interrupts the current Claude invocation rather than waiting for it.

The main loop in `internal/runner/runner.go:438-694` checks `ctx.Done()` at the top of each iteration (line 439-445). Five stop conditions are checked between iterations: context cancellation, max iterations, time budget, no work, and stuck beads.

`Run()` signature is `func (r *Runner) Run(ctx context.Context, maxIterations int, deadline time.Time, dryRun bool) error`.

### Changes Required

**`cmd/gromit/main.go`** — Replace the signal handler goroutine. Create `stopCh`, pass it to `Run()`. First SIGINT closes the channel; second SIGINT calls `cancel()`.

**`internal/runner/runner.go`** — Add `stopCh <-chan struct{}` parameter to `Run()`. Add a `select` case for `stopCh` alongside the existing `ctx.Done()` check at the top of the loop. Label the for-loop so `break` exits it from inside the select.

**Tests calling `Run()`** — Pass `nil` or a fresh channel for the new parameter. A `nil` channel never fires, preserving current behavior.
