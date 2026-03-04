---
id: fix-stale-worktree-branch-checkout
spec: "debug-20260304-081500"
created: 2026-03-04
decomposed: false
---

# Fix: Stale worktree blocks spec branch checkout

## Research & Context
See investigation report: `.gromit/reports/debug-20260304-081500.md`

Session worktrees from previous `gromit run` invocations aren't cleaned up, leaving branches locked and preventing subsequent runs from checking out the same spec branches.

## Architecture

The fix has two parts that work together:

1. **Reactive recovery** in `CreateOrCheckoutSpecBranch` — detect worktree conflicts and attempt stale worktree removal before retrying checkout
2. **Proactive cleanup** at run startup — prune dead session worktrees before iteration begins

Both parts use the same stale-worktree detection logic: parse the conflicting worktree path from git error output, check process liveness, and remove if dead.

## Tasks

### Task 1: Add stale worktree detection and removal helper
**Files**: `internal/runner/specbranch/git_ops.go`
**Size**: Small (20-30 lines)

Add a helper function that:
- Parses "already used by worktree at '<path>'" from git error output
- Checks if the worktree path matches the session pattern (`*-gromit-run-*` or `*-gromit-*`)
- Checks process liveness (extract timestamp from path, look for running gromit processes)
- If stale, runs `git worktree remove --force <path>` to free the branch
- Returns whether recovery was attempted

### Task 2: Integrate recovery into CreateOrCheckoutSpecBranch
**Files**: `internal/runner/specbranch/git_ops.go`
**Size**: Small (10-15 lines)

When plain checkout fails with worktree conflict:
1. Call the stale detection helper
2. If stale worktree was removed, retry the checkout once
3. If not stale (active process), return the original error

### Task 3: Tests for worktree-conflict recovery
**Files**: `internal/runner/specbranch/git_ops_test.go`
**Size**: Medium (40-60 lines)

Test cases:
- Checkout succeeds after stale worktree removal
- Checkout still fails when worktree is active (non-stale)
- Error message format when recovery fails
- Parse function handles various git error formats

### Task 4: Proactive stale worktree pruning at run startup (optional)
**Files**: `internal/runner/orchestrator.go` or `internal/worktree/worktree.go`
**Size**: Small (15-20 lines)

Add `PruneStaleSessionWorktrees()` that lists worktrees, identifies dead sessions, and removes them. Call at the start of `Run()`.

## Dependencies
- Task 1 before Task 2
- Task 1 before Task 3
- Task 4 is independent

## Testing Strategy
- Unit tests for parse/detection/recovery logic (Task 3)
- Manual verification: remove stale worktrees, confirm branch checkout succeeds
- Existing orchestrator tests verify bead failure behavior is unchanged for non-recoverable errors
