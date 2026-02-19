---
id: session-completion-timeouts
source_ideas: []
created: 2026-02-19
---

# Session Completion Timeouts

## Specification

`runSessionCompletion()` uses `context.Background()` for `git pull --rebase` and `git push`. If the remote is unreachable, these calls block indefinitely. The user's only recourse is SIGKILL.

Add a configurable timeout to the two network-facing git operations in `runSessionCompletion()`. Replace `context.Background()` with `context.WithTimeout(context.Background(), timeout)` for the `git pull --rebase` and `git push` calls. Default to 60 seconds. Expose the value as `git.push_timeout` in `gromit.yaml`.

Local-only operations (`git status`, `git add`, `git commit`) and `runBetweenIterationsCommand()` keep `context.Background()` — they complete in milliseconds and gain nothing from a timeout.

```yaml
git:
  push_timeout: 60  # seconds; 0 disables the timeout
```

## Acceptance Criteria

- `git pull --rebase` and `git push` in `runSessionCompletion()` use a timeout context instead of `context.Background()`
- Default timeout is 60 seconds
- `git.push_timeout: 0` disables the timeout (uses `context.Background()`)
- Timeout expiry follows the existing `push_failure` policy: returns an error when `"stop"`, logs a warning otherwise
- Local git operations (`commitGeneratedMetrics`, `commitGeneratedState`) remain on `context.Background()`

## Decisions

1. **Timeout, not context propagation** — The parent context may already be cancelled (user pressed Ctrl+C to stop the loop). Propagating it would skip the push entirely. A fresh timeout context lets the push attempt complete on its own terms.

2. **60-second default** — Generous enough for slow remotes, short enough that a hung connection resolves within a minute. Git's own `GIT_HTTP_TIMEOUT` is unbounded by default, so this provides a tighter bound.

3. **Reuse `push_failure` policy** — Timeout is just another push failure mode. The existing `"stop"` / `"warn"` policy already handles this; no new config knob needed for error disposition.

4. **Local ops excluded** — `git status`, `git add`, and `git commit` operate on the local index and complete in milliseconds. Adding timeouts to them adds complexity with no practical benefit.

## Research & Context

### Current State

`runSessionCompletion()` in `internal/runner/lifecycle.go` calls `r.runCmd(context.Background(), ...)` for `git pull --rebase` (line 215), `git push` (line 259), and `git status --short --branch` (line 275). `commitGeneratedMetrics()` and `commitGeneratedState()` each make three additional `context.Background()` calls for local git operations.

The `runCmd` method in `internal/runner/helpers.go:114` delegates to an injectable `cmdRunnerFn`, which calls `exec.CommandContext` — so the timeout context will propagate correctly to the subprocess.

### Changes Required

**`internal/config/config.go`** — Add `PushTimeout int` to `GitConfig`. Default to 60 in the accessor method.

**`internal/runner/lifecycle.go`** — In `runSessionCompletion()`, build a timeout context for the `git pull --rebase` and `git push` calls. The verification `git status --short --branch` call can keep `context.Background()` since it is local and best-effort.
