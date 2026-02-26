---
id: session-worktree-cleanup-before-merge
source_spec: session-worktree-cleanup-before-merge
created: 2026-02-26
decomposed: false
---

# Session Worktree Cleanup Before Merge Attempt Implementation Plan

**Goal:** Ensure interactive session branches and worktree directories are always cleaned up after session completion, whether merge succeeds immediately or in deferred auto-merge.

**Architecture:** Remove session worktrees before immediate merge attempts, and extend deferred merge epilogue flow to remove orphaned session worktree directories after successful branch merge.

**Tech Stack:** Go, git worktree plumbing, existing gromit runner/epilogue pipeline and state tracking.

**Spec:** `.gromit/specs/session-worktree-cleanup-before-merge.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Remove session worktrees before immediate merge attempts so branch deletion is never blocked by a checked-out worktree, and add deferred-path orphan cleanup right after successful merge-back.

**Key Components:**
1. **Immediate session flow (`cmd/gromit`)**: reorder cleanup/merge so `git worktree remove <sessionDir>` runs before `MergeBack`.
2. **Worktree manager helper (`internal/worktree`)**: add a path-based cleanup helper that safely removes an existing worktree directory.
3. **Deferred merge epilogue (`internal/pipeline/epilogue`)**: after successful `MergeBack`, invoke orphaned session worktree cleanup for that merged branch.

**Integration Points:**
- Update immediate path sequencing in `runWithSessionWorktreeWithConflictSettings` and/or `attemptMergeWithConflictPolicy`.
- Extend deferred path behavior in epilogue’s `PendingBranches` loop after successful merge.
- Keep pending-branch state removal behavior unchanged, but perform it after both merge and orphan cleanup succeed (or log cleanup warning and still remove state, based on desired strictness).

**Data Flow:**
- Immediate path:
  1. Create session worktree + branch
  2. Run callback
  3. Record pending branch
  4. Remove session worktree
  5. Merge branch back
  6. Delete branch (inside `MergeBack`)
  7. Remove pending branch from state
- Deferred path:
  1. Enumerate pending branches
  2. Merge branch back
  3. On success, cleanup orphaned session worktree dir for merged branch
  4. Remove pending branch from state

**Files to Modify:**
- `cmd/gromit/interactive_worktree.go` - move session worktree removal to pre-merge path and simplify checked-out-branch retry logic.
- `internal/worktree/worktree.go` - add helper to remove a worktree directory by path when present.
- `internal/pipeline/epilogue/epilogue.go` - cleanup orphaned session worktree dirs after successful deferred merge.
- `internal/runner/constructor_adapters.go` (if needed) - expose helper through adapter interface used by epilogue.
- `cmd/gromit/interactive_worktree_test.go` - update event-order expectations.
- `internal/worktree/worktree_test.go` - add helper-focused tests.
- `internal/pipeline/epilogue/epilogue_test.go` - add deferred cleanup scenario test.

**Files to Create:**
- None expected.

**Tradeoffs:**
- **Deterministic pre-merge cleanup vs retry-on-delete-error recovery**: pre-cleanup is simpler and avoids relying on fragile error-string detection.
- **Cleanup in epilogue via interface extension vs type assertion**: interface extension is clearer/safer for tests and compile-time guarantees, but touches adapter/test doubles.

## Test Strategy

**Test Levels:**
1. **Unit tests (`cmd/gromit`)**: verify immediate path now removes session worktree before any merge attempt, and preserves conflict-handoff behavior.
2. **Unit tests (`internal/pipeline/epilogue`)**: verify deferred path performs orphaned worktree cleanup after successful `MergeBack`.
3. **Unit tests (`internal/worktree`)**: verify new worktree-by-path removal helper is safe/idempotent and executes correct git command shape.
4. **Targeted integration-style behavior tests**: simulate immediate merge failure + deferred merge success, asserting no branch/worktree leftovers through mocks (no full git fixture required unless existing test pattern already uses one nearby).

**Key Test Cases:**
- Immediate success path order: `add pending -> cleanup session worktree -> merge -> clear pending`.
- Immediate cleanup failure: returns wrapped error with branch context and does not attempt merge.
- Conflict/manual handoff path: session worktree is not pre-removed when handoff is intended (if policy requires retaining workspace).
- Deferred success: `PendingBranches` branch merged, orphan cleanup called, pending branch removed.
- Deferred merge failure: no orphan cleanup call, warning emitted once per unique error.
- Helper behavior: no-op when path absent; calls `git worktree remove <path>` when present; surfaces command stderr on failure.

**Mocking Strategy:**
- Keep using existing function-injection seams in `interactive_worktree_test.go`.
- Extend epilogue fake manager interface with cleanup method and assert call order via recorded events.
- For `worktree.Manager` helper tests, continue `WithGitRunFn` mock pattern to assert arguments without shelling out.

**Coverage Goals:**
- Critical path: eliminate checked-out-branch delete failure class by reordering cleanup before merge.
- Deferred reconciliation: branch merged via epilogue leaves no orphaned worktree directory.
- Regression guard: existing merge conflict and pending-branch state behaviors remain unchanged.

**Test Organization:**
- Add/adjust tests in:
  - `cmd/gromit/interactive_worktree_test.go`
  - `internal/pipeline/epilogue/epilogue_test.go`
  - `internal/worktree/worktree_test.go`
- Follow existing `Test<Subject>_<Condition>_<Outcome>` style and event-list assertions already used in these files.

## Implementation Tasks

### Task 1: Reorder Immediate Session Cleanup Before Merge

**Files:**
- Modify: `cmd/gromit/interactive_worktree.go`
- Test: `cmd/gromit/interactive_worktree_test.go`

**What to Do:**
Refactor immediate merge flow so session worktree cleanup runs before `MergeBack` on normal completion paths. Remove or simplify checked-out-branch delete retry logic that becomes obsolete once worktree is removed pre-merge. Preserve conflict-policy behavior and pending-state semantics.

**Acceptance Criteria:**
- Immediate path removes session worktree before first merge attempt.
- Pending branch is cleared only after successful merge.
- Existing conflict handoff behavior remains intact.

**Dependencies:**
- None.

**Notes:**
Keep error wrapping branch-specific and ensure cleanup failures are clearly surfaced.

### Task 2: Add Worktree Manager Helper for Path-Based Session Worktree Removal

**Files:**
- Modify: `internal/worktree/worktree.go`
- Test: `internal/worktree/worktree_test.go`

**What to Do:**
Add a helper on `Manager` that removes a worktree directory by explicit path if it exists and belongs to tracked git worktrees. Reuse existing `runGit` abstraction and keep behavior deterministic for tests.

**Acceptance Criteria:**
- Helper can remove an existing session worktree path via `git worktree remove`.
- Helper is safe/no-op when the path is absent or not registered.
- Failures include actionable context (path and git error output).

**Dependencies:**
- None.

**Notes:**
Use `worktree list --porcelain` parsing to avoid false-positive removals.

### Task 3: Extend Deferred Epilogue Merge Loop to Cleanup Orphaned Session Worktrees

**Files:**
- Modify: `internal/pipeline/epilogue/epilogue.go`
- Modify: `internal/runner/constructor_adapters.go` (if adapter/interface wiring required)
- Test: `internal/pipeline/epilogue/epilogue_test.go`

**What to Do:**
After successful `MergeBack` in epilogue’s pending-branch loop, call the new worktree cleanup helper for that branch’s derived session worktree path, then remove branch from pending state. Add warnings for cleanup failures without regressing merge progress.

**Acceptance Criteria:**
- Deferred successful merge triggers orphaned worktree directory cleanup.
- Pending state removal still occurs for successfully merged branches.
- Merge-failure path behavior and warning de-dup remain unchanged.

**Dependencies:**
- Task 2 (cleanup helper availability).

**Notes:**
If interface expansion is needed, update all test doubles in epilogue tests accordingly.

### Task 4: Add Regression Test for Immediate-Fail Then Deferred-Success Cleanup Scenario

**Files:**
- Modify: `internal/pipeline/epilogue/epilogue_test.go`
- Modify: `cmd/gromit/interactive_worktree_test.go` (event/order updates if needed)

**What to Do:**
Add a scenario-level regression test that models immediate merge not finalizing cleanup and deferred epilogue merge completing successfully, then assert no orphan branch/worktree remains through mock/event assertions.

**Acceptance Criteria:**
- Test reproduces failure class described in spec.
- Test asserts deferred path performs merge + cleanup + pending-state removal.
- Test fails on previous behavior and passes with new behavior.

**Dependencies:**
- Task 1
- Task 3

**Notes:**
Prefer deterministic mock sequencing over timing-sensitive concurrency.

### Task 5: Run Targeted and Full Worktree Lifecycle Test Suites

**Files:**
- Test: `cmd/gromit/interactive_worktree_test.go`
- Test: `internal/worktree/worktree_test.go`
- Test: `internal/pipeline/epilogue/epilogue_test.go`

**What to Do:**
Execute focused tests for touched packages, then broader suite (`go test ./...`) if practical, confirming no lifecycle regressions.

**Acceptance Criteria:**
- New tests pass.
- Existing worktree lifecycle tests continue to pass.
- No regressions in epilogue merge behavior.

**Dependencies:**
- Task 1
- Task 2
- Task 3
- Task 4

**Notes:**
If full suite is expensive, record at least package-level test command results.

---

## Notes

- The spec references `internal/runner/lifecycle.go`, but in this codebase the deferred merge loop currently lives in `internal/pipeline/epilogue/epilogue.go`; implementation should target the active path.
- Keep branch-name and worktree-path derivation centralized to reduce mismatch risk between immediate and deferred cleanup paths.
- Preserve current warning behavior: deferred cleanup should not abort the loop, but must emit clear diagnostics.
