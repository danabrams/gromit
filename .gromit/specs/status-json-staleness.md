---
id: status-json-staleness
source_ideas: [idea-1770462181900]
created: 2026-02-07
---

# Status JSON Staleness Detection

## Specification

When `gromit run` starts, it writes a `status.json` file to `.gromit/` with run metadata. On clean exit, a deferred `Delete()` removes the file. If the process crashes or is killed (SIGKILL, power loss, etc.), the file is left behind with stale data.

This feature adds two things:

1. **PID tracking in `status.json`**: The `StatusWriter` includes the current process PID when writing `status.json`. This enables definitive liveness checks.

2. **`gromit status` reads and validates `status.json`**: Before showing the normal bead queue, `gromit status` checks for `status.json`. If present, it checks whether the PID is still alive:
   - **PID alive**: Display run-in-progress info (iteration, bead, model, elapsed time) before the normal queue output.
   - **PID dead**: Print a warning about the stale run (when it started, which bead, how long ago), delete the stale file, then show normal status.
   - **No `status.json`**: Show normal status (current behavior).

The PID liveness check uses `os.FindProcess(pid)` followed by sending signal 0 (`process.Signal(syscall.Signal(0))`), which is the standard Unix technique — it returns nil if the process exists and an error if it doesn't.

## Acceptance Criteria

- `status.json` includes a `pid` field set to `os.Getpid()` when written by `StatusWriter.Write()`
- `gromit status` displays run-in-progress info when `status.json` exists and the PID is alive
- `gromit status` prints a warning and deletes `status.json` when the PID is dead, then shows normal status

## Decisions

1. **PID check over timeout heuristic.** A PID check is definitive — the process is either running or it isn't. Timeout heuristics require arbitrary thresholds and always risk false positives (long-running legitimate runs) or false negatives (short crashes).

2. **Warn and clean on stale detection.** When a stale file is found, `gromit status` both warns the user (so they know a previous run crashed) and deletes the file (so it doesn't linger and confuse future invocations).

3. **No run-start guard.** `gromit run` does not check for an existing `status.json` before starting. Two concurrent runs would be a bd-level concern (bead locking), not a status file concern. This keeps the scope narrow.

## Research & Context

### Current State

- `StatusWriter` lives in `internal/runner/status.go`. It writes `status.json` with fields: `running`, `iteration`, `bead_id`, `bead_title`, `model`, `started_at`, `elapsed_s`.
- `StatusWriter.Delete()` is called via `defer` in `internal/runner/runner.go:189`. If the process is killed, the defer never runs.
- `gromit status` (`cmd/gromit/main.go:60`) calls `runner.Status()` (`internal/runner/runner.go:801`), which only queries bd for the next ready bead. It does not read `status.json` at all.
- The current `status.json` on disk is stale — shows `running: true` from a run that started hours ago and is no longer running.

### Reading `status.json`

`StatusWriter` currently only writes and deletes. A new `ReadStatus` function (or similar) will be needed to load and parse the file. This should live in `internal/runner/status.go` alongside the existing writer.
