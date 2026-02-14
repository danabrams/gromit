# Review Fix Progress

## Context
Running `gromit review` for the last 15 iterations (commits 0815983..8ac8338, 35 commits, ~4,500 lines).
The review found 6 issues. All 6 fixed.

## Status — ALL DONE

### Task 1: Fix silent swallowing of Codex usage-limit errors - DONE
**File: `internal/provider/codex.go`**
- Added error event emission in `turn.completed` case when `event.ErrorInfo != nil`
- Changed `processCodexStream` return signature from `(string, *codexUsage, error)` to `(string, *codexUsage, *codexErrorInfo, error)`
- Updated `StreamRun` caller to check `streamErrInfo` and return `Success: false`
- Made `_, _ = io.WriteString(stdin, prompt)` explicit in all 3 goroutines

**File: `internal/provider/codex_event_parsing_test.go`** - Updated all calls to 4-return
**File: `internal/provider/codex_streaming_acceptance_test.go`** - Updated call to 4-return

### Task 2: Add file locking to InteractiveFile - DONE
**File: `internal/state/interactive_state.go`**
- Added `withFileLock` helper using `syscall.Flock` with `.lock` file
- Wrapped `Load()` and `Save()` internals in `withFileLock`

### Task 3: Fix containsPath edge case - DONE
**File: `internal/worktree/worktree.go`**
- Replaced single-pass substring check with line-by-line split approach

### Task 4: Handle branch-already-exists in EnsureWorktree - DONE
**File: `internal/worktree/worktree.go`**
- Added retry without `-b` when first `worktree add -b gromit/interactive` fails

### Task 5: Handle stdin write errors in CodexProvider - DONE
**File: `internal/provider/codex.go`**
- Changed all 3 goroutines from `io.WriteString(stdin, prompt)` to `_, _ = io.WriteString(stdin, prompt)`

### Task 6: Clean up stale ATDD "Expected failure" comments - DONE
Removed all "Expected failure:" and "Red:" comment lines from:
- `internal/worktree/worktree_test.go`
- `internal/worktree/detect_test.go`
- `internal/state/interactive_state_test.go`
- `internal/config/worktree_config_test.go`
- `internal/provider/codex_events_test.go`
- `internal/provider/codex_event_parsing_test.go`

## Verification
- `go build ./cmd/gromit` — pass
- `go test ./internal/provider/... ./internal/state/... ./internal/worktree/... ./internal/config/...` — all pass
- `go test -tags acceptance ./internal/provider/...` — pass
