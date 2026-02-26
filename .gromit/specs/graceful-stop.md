---
id: graceful-stop
source_ideas: []
created: 2026-02-13
updated: 2026-02-18
epic: run-loop-reliability
---

# Graceful Stop via SIGQUIT

## Specification

Pressing Ctrl+\ (SIGQUIT) during `gromit run` finishes the current bead, then exits the loop. Ctrl+C (SIGINT) keeps its current behavior: cancel the context and stop immediately.

A `stopCh chan struct{}` carries the graceful-stop signal from the signal handler in `main.go` to the runner loop. The channel decouples signal handling (CLI concern) from loop control (runner concern), keeping OS signals out of the runner package and making the behavior easy to test.

**Ctrl+\ (SIGQUIT):** Closes `stopCh`. Prints `Graceful stop requested — will exit after current bead completes` to stderr. The context stays live, so the in-flight bead runs to completion through all its phases. Repeated SIGQUIT signals are ignored (the channel is already closed).

**Ctrl+C (SIGINT) / SIGTERM:** Cancels the context and kills the in-flight Claude process immediately. No change from current behavior.

## Acceptance Criteria

- SIGQUIT during a running bead prints the graceful-stop message and lets the bead finish all phases
- After the bead finishes, the loop exits through the normal cleanup path (stats, clean-exit marker, retro suggestion)
- SIGINT/SIGTERM cancel the context and stop immediately, unchanged from today
- Repeated SIGQUIT after the first has no effect
- When no signal is sent, behavior is identical to today

## Decisions

1. **Separate signal (SIGQUIT), not two-stage Ctrl+C** — Using a distinct signal avoids changing the meaning of the first Ctrl+C. Users who press Ctrl+C expect immediate cancellation; redefining it as "graceful stop" violates that expectation. Ctrl+\ is a standard Unix signal that users can discover, and it keeps the two intents — "stop soon" and "stop now" — on separate keys.

2. **Stop channel, not atomic flag** — A channel integrates with Go's `select` and matches the existing `ctx.Done()` pattern. It also keeps the runner independent of the signal handler's state.

3. **Signal handling stays in main.go** — The runner should not know about OS signals. The channel abstraction lets tests close `stopCh` directly without signal machinery.

4. **Finish the entire bead, not just the current phase** — Stopping between phases (e.g., after build but before validate) leaves the bead in an ambiguous state. Completing all phases ensures each bead is either fully done or fully not started.

5. **Ignore repeated SIGQUIT** — Once `stopCh` is closed, the stop is already pending. A second Ctrl+\ needs no action. Users who want immediate termination can press Ctrl+C.

## Research & Context

### Current State

The signal handler lives in `cmd/gromit/main.go:142-148`. It listens for SIGINT/SIGTERM, prints a message, and calls `cancel()` on the context.

`Run()` already accepts a `stopCh <-chan struct{}` parameter, and `shouldStopLoop()` already checks it with a non-blocking select. However, `main.go` passes `nil` for this parameter today.

### Changes Required

**`cmd/gromit/main.go`** — Add a second `signal.Notify` for `syscall.SIGQUIT`. Create `stopCh := make(chan struct{})`. In the SIGQUIT handler goroutine, close the channel and print the message. Pass `stopCh` to `r.Run()` instead of `nil`. SIGINT/SIGTERM handling stays unchanged.

No changes needed in the runner package — the `stopCh` plumbing already works.
