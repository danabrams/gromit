---
id: debug-runbook
source_spec: debug-runbook
created: 2026-02-18
decomposed: true
---

# Deterministic Debug Runbook Implementation Plan

**Goal:** Automatically capture failure context when beads exhaust retries and provide a replay mechanism through `gromit debug` with picker, context injection, and optional worktree restoration.

**Architecture:** Wire the existing `internal/runbook` package into the runner's escalation failure path for capture, and into `cmd/gromit/debug.go` for picker-based replay with optional `--restore` worktree mode.

**Tech Stack:** Go, cobra CLI, git worktrees

**Spec:** `.gromit/specs/debug-runbook.md`

---

## Architecture

**Overview:**
Three integration points connect the existing `internal/runbook` package (Entry, Append, List, Cleanup) to the runner and debug command. No new packages needed.

**Key Components:**
1. **Runner Runbook Capture** (`internal/runner/runbook_capture.go`): Method on Runner that builds `runbook.Entry` from `BeadContext` and calls `runbook.Append()` best-effort after escalation exhaustion.
2. **Debug Picker** (`cmd/gromit/debug.go`): Numbered menu of recent failures from `runbook.List()`, presented when `gromit debug` is invoked with no arguments.
3. **Restore Worktree** (`cmd/gromit/debug.go`): `--restore` flag creates git worktree at `failure_commit` for true state reproduction.

**Integration Points:**
- `internal/runner/run_iteration.go` — add captureRunbookEntry call after writeIterationLog on exhausted failure
- `internal/runner/runner.go` — (minimal) no changes if capture method lives in separate file on Runner
- `cmd/gromit/debug.go` — picker flow in runDebug(), context injection in buildDebugPrompt(), --restore flag
- Environment snapshot via `runtime.Version()`, `runtime.GOOS`, `runtime.GOARCH`

**Data Flow:**
```
Bead fails → escalation exhausted → captureRunbookEntry() → runbook.Append() → .gromit/runbooks.jsonl
                                                                                        ↓
gromit debug → runbook.List() → picker menu → user selects → inject into buildDebugPrompt()
                                                    ↓ (if --restore)
                                        git worktree add → debug in worktree → cleanup
```

**Tradeoffs:**
- Separate `runbook_capture.go` keeps runner.go clean; methods still on Runner struct
- Best-effort capture: append failures log warning but don't break the loop
- Environment from runtime package (no external calls)
- Picker reads stdin directly (matches existing CLI patterns)
- Context-only replay by default; --restore uses worktree for true reproduction

## Test Strategy

**Unit Tests (capture):** `internal/runner/runbook_capture_test.go`
- Verify all Entry fields populated from BeadContext and IterationResult
- Verify best-effort: append failure logs warning, doesn't error
- Verify capture skipped for non-exhausted outcomes (success, decomposed, already-done, precheck/scope skip)
- Verify ID format `rb-<unix-timestamp>-<bead-id>`
- Verify failure output capped at 5KB

**Unit Tests (picker + prompt):** extend `cmd/gromit/debug_test.go`
- buildDebugPrompt with *runbook.Entry — Failure Context section present
- buildDebugPrompt with nil — unchanged behavior
- Picker display formatting (relative timestamps, bead ID, title, category)
- Picker selection parsing (valid index, 0 for fresh, out of range)
- Empty runbook list falls through to existing behavior
- Description argument skips picker

**Unit Tests (--restore):** extend `cmd/gromit/debug_test.go`
- Worktree creation at correct path
- Fallback when failure_commit is empty
- Cleanup on decline, keep on accept
- Mock git commands (no real worktrees)

**Mocking Strategy:**
- Function injection for runbook.List/Append in capture tests
- io.Reader injection for stdin in picker tests
- Mock git worktree commands — verify args, don't create real worktrees
- Existing BeadContext/IterationResult construction patterns from runner tests

## Implementation Tasks

### Task 1: Wire runbook capture into runner

**Files:**
- Create: `internal/runner/runbook_capture.go`
- Create: `internal/runner/runbook_capture_test.go`
- Modify: `internal/runner/run_iteration.go`

**What to Do:**
Add `captureRunbookEntry(bc *runtypes.BeadContext, result *runtypes.IterationResult)` method on Runner. Build a `runbook.Entry` from BeadContext fields: bead ID, title, spec ID (from labels), StartCommit, current HEAD as failure_commit (via `r.getHead()`), rendered build prompt (`bc.BuildPrompt`), validation commands (`r.cfg.Validation.Commands` or fast+full), failure output from `result.AcceptanceFailureOutput` (capped at 5KB by `runbook.Append`), failure category from `result.FailureCategory`, escalation chain from `r.cfg.Escalation.Chain`, and env from `runtime` package. Call `runbook.Append(r.cfg.GromitDir(), entry)` with warning on error.

In `run_iteration.go`, after `writeIterationLog` in `processSingleBead()`, add a call to `r.captureRunbookEntry(bc, result)` guarded by: result is not success, not decomposed, not already-done, not precheck/scope skip — i.e., the bead exhausted retries.

**Acceptance Criteria:**
- captureRunbookEntry populates all Entry fields from BeadContext and IterationResult
- Append failure logs warning but does not return error to caller
- Capture only fires for exhausted-retry failures, not for success/decompose/skip outcomes

**Dependencies:** None (internal/runbook package already exists)

**Notes:**
- Use `runtime.Version()`, `runtime.GOOS`, `runtime.GOARCH` for env
- Spec ID extraction: scan bead labels for `spec:` prefix, or use empty string
- Validation commands: combine fast_commands and full_commands from config, or use the commands that actually ran (check what's available in IterationResult)

---

### Task 2: Add runbook picker and context injection to gromit debug

**Files:**
- Modify: `cmd/gromit/debug.go`
- Modify: `cmd/gromit/debug_test.go`

**What to Do:**
Modify `runDebug()`: when invoked with no description argument, call `runbook.List(gromitDir, cfg.Runbook.TTLDays)` to get recent entries. If entries exist, display a numbered picker menu showing relative timestamp, bead ID, title, and failure category per the spec format. Read user selection from stdin. Selection of 0 or empty falls through to existing behavior. Selection of a valid entry passes `*runbook.Entry` to `buildDebugPrompt()`.

Modify `buildDebugPrompt()`: accept an optional `*runbook.Entry` parameter. When non-nil, inject a "Failure Context" section before the existing "Context" section containing: bead ID and title, spec ID, failure category, escalation chain attempted, start and failure commit refs with `git diff <start>..<failure>` instruction, validation commands, failure output, and the rendered build prompt. When nil, behave as before.

When `gromit debug "description"` is passed, skip the picker entirely and launch directly (existing behavior).

**Acceptance Criteria:**
- Picker displays recent failures with relative timestamps when no args given
- Selecting an entry injects full failure context into the debug prompt
- Selecting 0 or providing a description arg skips the picker
- Empty runbook list falls through to existing behavior

**Dependencies:** Task 1 (entries must exist to be picked, but picker code can be written independently against `runbook.List()`)

**Notes:**
- Relative time formatting: "2h ago", "1d ago", "3d ago" etc.
- Config loading for TTLDays: debug command already loads config
- stdin reading: use `bufio.Scanner` on `os.Stdin` or inject reader for testability

---

### Task 3: Add --restore worktree mode to gromit debug

**Files:**
- Modify: `cmd/gromit/debug.go`
- Modify: `cmd/gromit/debug_test.go`

**What to Do:**
Add `--restore` boolean flag to the debug cobra command. When set and a runbook entry is selected (from picker):

1. Create git worktree at `entry.FailureCommit` in `.gromit/worktrees/debug-<entry.ID>/` via `git worktree add <path> <commit>`
2. Launch the debug session with the working directory set to the worktree path
3. After session exits, prompt "Keep worktree? [y/N]" on stdin
4. If declined (default), remove via `git worktree remove <path>`

Fall back to context-only mode (no worktree) if: `FailureCommit` is empty, worktree creation fails, or no runbook entry was selected.

`--restore` without a selected entry (user chose 0 or provided description) is ignored with a log message.

**Acceptance Criteria:**
- `--restore` creates worktree at failure_commit when entry selected
- Debug session runs in worktree directory
- Worktree cleaned up when user declines keep prompt
- Falls back to context-only when failure_commit missing or worktree creation fails

**Dependencies:** Task 2 (picker provides the selected entry)

**Notes:**
- Git worktree commands: `git worktree add <path> <commit>`, `git worktree remove <path>`
- Worktree path: `.gromit/worktrees/debug-<runbook-id>/` under project root
- Mock git commands in tests — verify args passed, don't create real worktrees
- Consider adding `.gromit/worktrees/` to `.gitignore` if not already there

---

## Notes

- The `internal/runbook` package and `RunbookConfig` are already implemented and tested. This plan covers only the remaining integration work.
- Existing open beads map to these tasks: `gromit-zdr9` (Task 1), `gromit-5fd7` (Task 2), `gromit-sqn5v` (Task 3).
- The capture point in the runner is best-effort by design — a failure to write a runbook entry should never break the main loop.
- The picker only appears on bare `gromit debug` — pre-seeded invocations (`gromit debug "description"`) skip it entirely, preserving existing workflow.
