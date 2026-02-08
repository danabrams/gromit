---
id: run-output-newlines
source_ideas: []
created: 2026-02-07
---

# Fix Missing Newlines in gromit run Output

## Specification

During `gromit run`, terminal output has missing newlines causing lines to run together. There are two root causes:

### 1. Heartbeat Overwrite / Streaming Text Race Condition

The heartbeat goroutine and `processStreamJSON` (in the main goroutine) both write to the same `io.Writer` (`r.output`) concurrently without synchronization. When `overwriteHeartbeat()` writes `\r[status]` (deliberately without a trailing `\n` for in-place updates), and Claude then outputs a text block via `fmt.Fprint(output, block.Text)`, the text appears on the same line as the heartbeat — no newline separates them.

The sequence that causes the bug:
1. `printHeartbeat` writes `[0m15s] Waiting for Claude...\n`
2. Tool call event triggers `overwriteHeartbeat` → writes `\r[0m20s] 3 tool calls, 1 file modified` (no `\n`)
3. `processStreamJSON` writes next text block → appears right after the heartbeat on the same line

The `stopHeartbeat()` cleanup does write `\n` when overwrite mode was used, but only after `StreamRun` returns — by then the damage is already done for any text blocks that arrived during overwrite mode.

### 2. Missing Phase Separators

There are no blank line separators between major output phases within an iteration, making output feel dense and hard to scan:
- Between Claude's streaming output ending and the `SUCCESS:`/`FAILED:` result line
- Between the result line and validation/review status messages
- Between setup messages (logging paths, baselines) and the first `=== Iteration 1 ===` header

## Acceptance Criteria

- When heartbeat overwrites are active, Claude's streaming text output always starts on a fresh line (never on the same line as a heartbeat status)
- All writes to `r.output` during `gromit run` are properly serialized (no interleaved partial lines from concurrent goroutines)
- A blank line separates Claude's streaming output from the subsequent `SUCCESS:`/`FAILED:` result line
- A blank line separates the setup messages from the first iteration header

## Decisions

1. **Synchronize writes with a mutex-protected writer.** Wrap `r.output` in a synchronized writer that serializes all writes. This is the simplest approach that prevents all interleaving issues. The heartbeat goroutine and the streaming output both go through the same writer, so a `sync.Mutex` around `Write()` ensures atomic line output. This is preferred over channel-based approaches because the existing code uses `fmt.Fprint` directly in many places (especially `claude.go`), and a writer wrapper is transparent to all callers.

2. **Heartbeat overwrite must write a newline before yielding to text output.** When `processStreamJSON` is about to write a text block and the last heartbeat used overwrite mode, a `\n` must be emitted first. This can be achieved either by having the synchronized writer track whether the last write was an overwrite (ended with non-`\n`) and prepend `\n` to the next non-heartbeat write, or by having the heartbeat goroutine write `\n` before the text block arrives. The synchronized writer approach is cleaner since it handles the transition automatically.

3. **Add explicit separator lines at phase boundaries.** Add `r.log("")` calls at key transition points: after setup messages before the first iteration, and after `StreamRun` returns before logging the result. This is a simple, low-risk change.

## Research & Context

### Current State

- **`r.log()` helper** (`internal/runner/runner.go:638-647`): Auto-appends `\n` if the message doesn't end with one. All runner console output goes through this except heartbeat overwrites and Claude streaming.
- **`overwriteHeartbeat()`** (`internal/runner/runner.go:786-808`): Writes `\r[status][padding]` without trailing `\n` — deliberately, for terminal in-place updates.
- **`processStreamJSON()`** (`internal/claude/claude.go:442-511`): Writes Claude's text blocks to `output` via `fmt.Fprint(output, block.Text)`. Ensures trailing `\n` at stream end.
- **`StreamRun()`** (`internal/claude/claude.go:298-395`): Writes `cmd:` and `prompt length:` debug lines directly to `output` with explicit `\n`.
- **`startHeartbeat()`** (`internal/runner/runner.go:673-764`): Launches a goroutine that writes heartbeat updates. The `stopHeartbeat()` cleanup writes `\n` if overwrite mode was used, but only after `StreamRun` returns.

### Previous Fixes

Three commits already addressed related issues:
- `8b23f7c` — Ensure trailing newline after Claude streaming output
- `399e216` / `7217b00` — Ensure trailing newline after heartbeat overwrite before next log output

These fixes addressed the post-stream and post-heartbeat transitions but did not address the concurrent write interleaving during streaming, nor the missing phase separators.

### Key Files

- `internal/runner/runner.go` — `r.log()`, heartbeat goroutine, `Run()` loop
- `internal/runner/process.go` — `executeClaudeInvocation()`, phase transitions
- `internal/claude/claude.go` — `StreamRun()`, `processStreamJSON()`
