---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T18:00:51-05:00"
id: run-output-newlines
source_spec: run-output-newlines
---

# Fix Missing Newlines in gromit run Output — Implementation Plan

**Goal:** Fix missing newlines in `gromit run` terminal output caused by heartbeat/streaming race conditions and missing phase separators.

**Architecture:** Wrap `r.output` in a mutex-protected `syncWriter` that serializes all writes and automatically handles the overwrite-to-text transition by prepending `\n` when needed. Add explicit blank line separators at phase boundaries.

**Tech Stack:** Go, sync.Mutex

**Spec:** `.gromit/specs/run-output-newlines.md`

---

## Architecture

### Overview

The heartbeat goroutine and `processStreamJSON` (main goroutine) both write to `r.output` concurrently without synchronization. When `overwriteHeartbeat()` writes `\r[status]` (no trailing `\n`), and Claude then outputs a text block, the text appears on the same line as the heartbeat.

The fix introduces a `syncWriter` — a thin `io.Writer` wrapper with a `sync.Mutex` that:
1. Serializes all writes (prevents interleaving)
2. Tracks whether the last write was an "overwrite" (no trailing `\n`)
3. Auto-prepends `\n` on the next normal write after an overwrite

### Key Components

1. **`syncWriter`** (`internal/runner/syncwriter.go`): Mutex-protected writer with `Write()` (implements `io.Writer`) and `WriteOverwrite(p []byte)` for heartbeat overwrite writes. Tracks `lastWasOverwrite bool` state.

2. **Heartbeat changes** (`internal/runner/runner.go`): `overwriteHeartbeat` uses `syncWriter.WriteOverwrite()`. `stopHeartbeat` cleanup no longer manually emits `\n` — the sync writer handles the transition automatically on the next write.

3. **Phase separators** (`internal/runner/runner.go`): `r.log("")` calls at key transition points for visual separation.

### Integration Points

- `r.output` in `Runner` becomes a `*syncWriter` wrapping the original `io.Writer`
- All existing `r.log()`, `fmt.Fprint(r.output, ...)`, and `claude.StreamRun(... r.output ...)` calls go through the sync writer transparently (it implements `io.Writer`)
- `overwriteHeartbeat` is the only caller that uses `WriteOverwrite` instead of the standard `Write`

### Data Flow

1. `Runner.Run()` wraps the provided `io.Writer` in a `syncWriter` at construction time
2. During execution, heartbeat goroutine calls `syncWriter.WriteOverwrite()` → writes `\r[status]`, sets `lastWasOverwrite = true`
3. `processStreamJSON` calls `syncWriter.Write()` → sees `lastWasOverwrite`, prepends `\n`, writes text, sets `lastWasOverwrite = false`
4. All writes are serialized by the mutex — no interleaving possible

### Files to Modify

- `internal/runner/runner.go` — Wrap `r.output` in `syncWriter`; update `overwriteHeartbeat` to use `WriteOverwrite`; simplify `stopHeartbeat` cleanup; add phase separator `r.log("")` calls
- `internal/runner/process.go` — No changes needed (writes go through `r.output` which is now a `syncWriter`)

### Files to Create

- `internal/runner/syncwriter.go` — The `syncWriter` type
- `internal/runner/syncwriter_test.go` — Unit tests

### Tradeoffs

- **Mutex wrapper vs channel-based**: Chose mutex because `fmt.Fprint(output, ...)` is used directly in many places (claude.go, startupMonitor), and a writer wrapper is transparent. A channel approach would require rewriting every call site.
- **State tracking in writer vs heartbeat**: Chose writer because it handles the transition automatically regardless of which goroutine writes next.

## Test Strategy

### Unit Tests (`syncwriter_test.go`)

- `Write` passes through to underlying writer correctly
- `WriteOverwrite` passes through and marks overwrite state
- After `WriteOverwrite`, next `Write` auto-prepends `\n`
- After normal `Write`, next `Write` does NOT prepend `\n`
- Multiple consecutive `WriteOverwrite` calls → only one `\n` on transition
- Concurrent writes don't interleave (goroutines + race detector)

### Integration Tests (existing mock infrastructure)

- Phase separator: blank line between setup messages and first iteration header (check output from `Run()` with mocks)
- Phase separator: blank line between streaming output and `SUCCESS:`/`FAILED:` result

### Manual Testing

- Run `gromit run` on a real bead, verify output formatting visually

## Implementation Tasks

### Task 1: Create syncWriter type

**Files:**
- Create: `internal/runner/syncwriter.go`
- Create: `internal/runner/syncwriter_test.go`

**What to Do:**
Create a `syncWriter` struct that wraps an `io.Writer` with a `sync.Mutex`. It implements `io.Writer` via `Write(p []byte) (int, error)` which acquires the lock, checks `lastWasOverwrite`, prepends `\n` if needed, writes `p` to the underlying writer, updates `lastWasOverwrite` based on whether `p` ends with `\n`, and returns. Also provide `WriteOverwrite(p []byte) (int, error)` which acquires the lock, writes `p`, and sets `lastWasOverwrite = true`.

Write comprehensive unit tests covering: normal writes, overwrite writes, overwrite-to-normal transitions, consecutive overwrites, and concurrent write safety.

**Acceptance Criteria:**
- `syncWriter` implements `io.Writer`
- After `WriteOverwrite`, the next `Write` prepends `\n` before the content
- All tests pass including `-race`

**Dependencies:** None

### Task 2: Integrate syncWriter into Runner and update heartbeat

**Files:**
- Modify: `internal/runner/runner.go`

**What to Do:**

1. In `NewRunner` and `NewRunnerWithDeps`, wrap the provided `output` in a `newSyncWriter(output)` before storing it in `r.output`. Store the `*syncWriter` as `r.output` (the field type stays `io.Writer` since `syncWriter` implements it — but we need to store the concrete type to call `WriteOverwrite`). Add a `syncOut *syncWriter` field to `Runner` and pass it as the `output` to all writes, while using `.WriteOverwrite()` specifically in `overwriteHeartbeat`.

2. In `overwriteHeartbeat` (line ~837): Replace `fmt.Fprintf(r.output, "\r%s%s", newLine, padding)` with `r.syncOut.WriteOverwrite([]byte(fmt.Sprintf("\r%s%s", newLine, padding)))`.

3. In `stopHeartbeat` (line ~788-794): Remove the manual `\n` write when `usedOverwrite` is true. The sync writer now handles this automatically when the next write arrives. Keep the `usedOverwrite` channel and `close(done)` for goroutine shutdown, but remove the `fmt.Fprint(r.output, "\n")` line.

**Acceptance Criteria:**
- `overwriteHeartbeat` uses `syncWriter.WriteOverwrite` instead of `fmt.Fprintf`
- `stopHeartbeat` no longer manually writes `\n`
- All existing tests pass

**Dependencies:** Task 1

### Task 3: Add phase separator blank lines

**Files:**
- Modify: `internal/runner/runner.go`

**What to Do:**

1. After the setup block (after line ~248 where review baseline is initialized, before the `for {` loop starts at line ~253): Add `r.log("")` to insert a blank line between setup messages and the first iteration header.

2. In `processBead` (line ~473), after `executeWithRetry` returns (line ~538-540): The `logResult` call is in `Run()` at line ~375, not in `processBead`. So add `r.log("")` in `Run()` at line ~374, right before `r.logResult(result)`, to separate streaming output from the SUCCESS/FAILED line.

**Acceptance Criteria:**
- A blank line appears between setup messages and the first `=== Iteration 1 ===` header
- A blank line appears between Claude's streaming output and the `SUCCESS:`/`FAILED:` result line

**Dependencies:** Task 1 (needs syncWriter to be in place so the blank line writes are also serialized)

---

## Notes

- The `syncWriter` must be transparent to `claude.go` — it receives `r.output` as an `io.Writer` parameter and writes to it via `fmt.Fprint`. No changes needed in `claude.go`.
- The `startupMonitor` in `claude.go` also writes to `output` — these writes will also be serialized through the sync writer, which is correct behavior.
- The `r.log()` helper already appends `\n` if missing, so it always produces "normal" (non-overwrite) writes. No changes needed there.
- Race detector should be run on all tests (`go test -race ./internal/runner/...`) to verify concurrent safety.
