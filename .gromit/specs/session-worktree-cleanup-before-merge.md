---
id: session-worktree-cleanup-before-merge
source_ideas: []
created: 2026-02-26
epic: codebase-health
---

# Session Worktree Cleanup Before Merge Attempt

## Problem

In `runWithSessionWorktreeWithConflictSettings`, worktree removal only happens after a successful merge. If the immediate merge fails (e.g., concurrent git operations from the run loop), the session branch stays checked out in its worktree, causing `git branch -d` to fail silently. The deferred run-loop path (`lifecycle.go` `PendingBranches` loop) can then merge the branch successfully but cannot delete it, leaving orphaned worktree directories and branches that persist across runs until manual cleanup.

## Approach

- In `cmd/gromit/interactive_worktree.go`, call `git worktree remove <sessionDir>` (or equivalent) **before** the merge attempt in `runWithSessionWorktreeWithConflictSettings`, so the branch is never held in a checked-out worktree when merge and branch delete run
- Update the deferred `MergeBack` path in `internal/runner/lifecycle.go` to also clean up orphaned session worktree directories whose branch was just merged — not only delete the branch
- Add a helper in `internal/worktree/worktree.go` that removes a worktree directory by path if it exists, called after a successful branch merge
- Write a test that simulates the failure scenario: session ends, immediate merge fails, deferred path merges, verify no orphaned worktree directory or branch remains

## Files to Change

- `cmd/gromit/interactive_worktree.go` — move worktree removal before merge attempt
- `internal/runner/lifecycle.go` — add orphaned worktree cleanup in `PendingBranches` loop
- `internal/worktree/worktree.go` — add worktree directory removal helper
- `internal/runner/lifecycle_test.go` — add test for deferred-path cleanup

## Acceptance Criteria

- Session worktree directory is removed before the merge attempt in the immediate path
- After a successful deferred-path merge, the associated worktree directory is also removed
- `git branch -d` succeeds because no worktree holds the branch at merge time
- No orphaned worktree directories or branches remain after interactive session completion, whether merge succeeds immediately or via the deferred path
- Existing worktree lifecycle tests continue to pass
