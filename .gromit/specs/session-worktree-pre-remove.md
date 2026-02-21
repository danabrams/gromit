---
id: session-worktree-pre-remove
source_ideas: []
created: 2026-02-21
---

# Fix Session Worktree Lifecycle: Remove Worktree Before Merge

## Specification

Fix two related bugs in the session worktree lifecycle that cause orphaned worktree directories, persistent `gromit/*` branches, and terminal merge-conflict errors on every run loop iteration after any interactive session (retro, review, explore, refine, debug, plan).

### Root Cause

**Bug 1 — immediate merge path** (`cmd/gromit/interactive_worktree.go`):

The current cleanup order is: attempt merge → on success, remove worktree dir via `clearMergedState`. When the immediate merge fails (e.g., run loop holds a git lock), the session worktree directory remains alive, keeping the branch registered as "checked out" in git's worktree list. Git refuses to delete a branch that is checked out in any worktree, so `git branch -d` inside `MergeBack` silently fails (errors are already discarded with `_, _`).

**Bug 2 — deferred merge path** (`internal/runner/lifecycle.go` `mergeInteractiveBranches` + `internal/worktree/worktree.go` `MergeBack`):

`PendingBranches()` lists all `gromit/*` branches via `git for-each-ref`. On each run loop iteration it finds the undeleted branch and calls `MergeBack`. The merge succeeds (changes already incorporated), but `git branch -d` silently fails again because the orphaned worktree dir is still registered. The branch and the dir persist indefinitely, repeating the cycle.

`MergeBack` never attempts to clean up the orphaned worktree directory — it only deletes the branch, and only best-effort.

### Fix 1 — Remove worktree before merge (immediate path)

In `runWithSessionWorktreeWithConflictSettings`, call `git worktree remove <sessionDir>` **before** calling `manager.MergeBack(session.BranchName)`. This detaches the branch from the worktree list so that `git branch -d` can succeed.

Revised sequence:

1. `CreateSessionWorktree` — create dir + branch, run callback.
2. `stateFile.AddPendingWorktreeBranch` — record for run loop fallback.
3. **`git worktree remove <sessionDir>`** — detach branch from worktree (new position).
4. Attempt merge:
   - Success: `clearMergedState` (remove from pending state only; worktree dir already gone).
   - Conflict: return handoff error with branch name retained in pending state; worktree dir already gone.

`clearMergedState` becomes state-only: it removes the pending branch record but no longer calls `interactiveWorktreeCleanupSessionFn` (that call moves to step 3).

If `git worktree remove` fails (e.g., uncommitted changes remain in the session dir), the operation surfaces the error and aborts before any merge attempt. The session dir is preserved for inspection.

**Conflict handoff message update:** the `mergeConflictHandoffError` message currently references `SessionDir` as a place to resolve conflicts manually. After this fix, the worktree dir is gone. The message should reference the branch name instead, instructing the user to check it out manually: `git worktree add <path> gromit/<branch-suffix>`.

### Fix 2 — Clean up orphaned worktree dir after successful deferred merge (MergeBack)

In `MergeBack` (`internal/worktree/worktree.go`), after a successful merge and before returning nil, derive the expected session worktree directory from the branch name and attempt a best-effort `git worktree remove`:

- Branch `gromit/<suffix>` → expected dir: `<m.MainDir>-gromit-<suffix>` (mirrors `sessionWorktreeDir` naming).
- Call `git worktree remove <expectedDir>` (best-effort; ignore "does not exist" or "not a worktree" errors).
- This cleans up any orphaned dir whose branch the deferred path successfully merges, including legacy dirs predating Fix 1.

Only the expected dir derived from the branch name is cleaned; no directory scanning or glob operations.

## Acceptance Criteria

- After a session callback completes, `git worktree remove <sessionDir>` is called before any merge attempt in `runWithSessionWorktreeWithConflictSettings`.
- On successful immediate merge, the branch is deleted and the worktree dir is already gone (no orphan).
- On failed immediate merge (conflict), the pending branch is retained for the run loop but the worktree dir is gone; the conflict error references the branch name, not a directory path.
- `MergeBack` attempts to remove the expected session worktree dir (derived from branch name) after each successful merge.
- After a successful deferred merge via `mergeInteractiveBranches`, the branch is deleted and any associated orphaned worktree dir is removed.
- Repeated run loop iterations do not re-surface the same branch as pending after it has been successfully merged and deleted.
- If `git worktree remove` before the merge fails (uncommitted changes), the error is surfaced and the merge is aborted; the session dir is preserved.
- `git worktree remove` failures during `MergeBack` cleanup (dir missing or not a worktree) are silently ignored.

## Decisions

1. **Remove worktree before merge, not after.** The prior design assumed successful merge was the safe moment to remove the worktree (keeping it alive for conflict resolution). In practice the worktree dir causes more harm by blocking branch deletion. Branch retention already provides the user access to their work; the dir is not needed for that.

2. **Abort merge on pre-removal failure, surface the error.** If `git worktree remove` fails (uncommitted changes), proceeding to merge would succeed without cleaning up the dir, recreating the bug. Better to fail loudly so the user can inspect and clean up.

3. **Update conflict error messages to reference branch name instead of dir.** With the dir gone, pointing users to a directory is misleading. The branch name is the stable identifier; users can check it out with `git worktree add <path> <branch>`.

4. **MergeBack derives expected dir from branch name (no scanning).** Scanning for all matching dirs would be fragile and could race. The branch-to-dir mapping is deterministic and reversible using the same naming convention in `sessionWorktreeDir`.

5. **Best-effort cleanup in MergeBack, strict cleanup before merge.** Pre-merge removal is strict (failure aborts the merge) to prevent the bug from recurring. Post-merge cleanup in `MergeBack` is best-effort (errors ignored) because the merge already succeeded and cleanup failure is a cosmetic issue.

## Research & Context

### Current State

**`cmd/gromit/interactive_worktree.go`:**
- `runWithSessionWorktreeWithConflictSettings` orchestrates the full lifecycle.
- `clearMergedState` (line 229) calls both `RemovePendingWorktreeBranch` and `interactiveWorktreeCleanupSessionFn` — these must be split: worktree removal moves before merge, state removal stays in `clearMergedState`.
- `interactiveWorktreeCleanupSessionFn` (line 38) runs `git worktree remove <sessionDir>` — reuse this function, just call it earlier.
- `mergeConflictHandoffError.Error()` (line 73) references `e.SessionDir` in manual and agent conflict messages — update to use branch name.

**`internal/worktree/worktree.go`:**
- `MergeBack` (line 197): branch deletes at lines 209 and 222 use `_, _` (already best-effort). Add worktree dir cleanup after each successful delete attempt.
- `sessionWorktreeDir` (line 258): `mainDir + "-gromit-" + command + "-" + timestamp`. Given a branch `gromit/<command>-<timestamp>`, the expected dir is `mainDir + "-gromit-" + strings.TrimPrefix(branch, "gromit/")`.
- `PendingBranches` (line 157): uses `git for-each-ref` to find all `gromit/*` branches — this is why un-deleted branches re-appear every iteration.

**`internal/runner/lifecycle.go`:**
- `mergeInteractiveBranches` (line 366): calls `r.worktreeManager.PendingBranches()` then `r.worktreeManager.MergeBack(branch)` for each. No worktree cleanup today. Fix 2 in `MergeBack` handles cleanup transparently from this call site.

### Branch-to-dir derivation

```
branch = "gromit/review-1771687158809624898"
suffix = strings.TrimPrefix(branch, "gromit/")  → "review-1771687158809624898"
expectedDir = m.MainDir + "-gromit-" + suffix   → "/home/user/project-gromit-review-1771687158809624898"
```

This matches the output of `sessionWorktreeDir(m.MainDir, command, timestamp)` exactly.
