---
id: session-worktree-pre-remove
source_spec: session-worktree-pre-remove
created: 2026-02-23
decomposed: false
---

# Session Worktree Pre-Remove Implementation Plan

**Goal:** Ensure interactive session branches and worktree directories are cleaned up reliably by detaching the session worktree before merge and cleaning orphaned session dirs after deferred merge success.

**Architecture:** Move `git worktree remove <sessionDir>` to execute immediately after pending-branch recording and before any merge attempt in the interactive path, then keep merged-state cleanup state-only. Extend `MergeBack` to derive the expected session dir from branch suffix and best-effort remove it after successful merges.

**Tech Stack:** Go, git CLI invocation via existing manager abstractions, existing unit-test harnesses in `cmd/gromit` and `internal/worktree`.

**Spec:** `.gromit/specs/session-worktree-pre-remove.md`

---

## Architecture

## Architecture Proposal

**Overview:**  
Implement strict pre-merge worktree detachment in the interactive session flow, then add best-effort orphan cleanup inside `MergeBack` so both immediate and deferred merge paths can reliably delete `gromit/*` branches and stop repeat merge loops.

**Key Components:**
1. **Interactive session lifecycle (`cmd/gromit/interactive_worktree.go`)**: Move `git worktree remove` to run after pending-branch recording and before first merge attempt.
2. **Merged-state cleanup (`cmd/gromit/interactive_worktree.go`)**: Make `clearMergedState` state-only (remove pending branch only).
3. **Conflict handoff messaging (`cmd/gromit/interactive_worktree.go`)**: Remove dependency on `SessionDir` guidance; instruct via branch-centric recovery (`git worktree add <path> <branch>`).
4. **Deferred/orphan cleanup (`internal/worktree/worktree.go`)**: After successful merge, derive expected session dir from branch suffix and run best-effort `git worktree remove`.
5. **Deferred merge caller (`internal/pipeline/epilogue/epilogue.go`)**: No behavioral changes needed; it already loops `PendingBranches()` + `MergeBack()` and will inherit cleanup automatically.

**Integration Points:**
- Immediate path currently merges first and cleans up later; invert to clean up first.
- Deferred path currently relies on `MergeBack` branch deletion; extend `MergeBack` to also clear derived orphan dir.
- Existing run-loop integration remains in `internal/pipeline/epilogue/epilogue.go:174`.

**Data Flow:**
- Session command finishes callback.
- Record pending branch in interactive state.
- Strict detach: `git worktree remove <sessionDir>` (abort on failure).
- Merge attempt(s):
  - Success: remove pending branch record only.
  - Conflict/exhausted retries: return handoff error; pending branch remains; worktree dir already detached.
- Later epilogue run: `PendingBranches` returns branch; `MergeBack` merges, deletes branch, then best-effort removes derived worktree dir.

**Files to Modify:**
- `cmd/gromit/interactive_worktree.go` - reorder lifecycle, update conflict text, keep state cleanup separate from filesystem cleanup.
- `cmd/gromit/interactive_worktree_test.go` - update call-order expectations and conflict-handoff assertions.
- `internal/worktree/worktree.go` - add derived-dir cleanup helper invoked after successful merges.
- `internal/worktree/worktree_test.go` - add/adjust tests for post-merge worktree removal behavior and ignored cleanup errors.
- `internal/pipeline/epilogue/epilogue_test.go` - optional assertion pass to confirm no API change needed (likely unchanged).

**Files to Create:**
- None expected.

**Tradeoffs:**
- **Strict pre-merge remove vs preserve dir for manual conflict resolution:** choose strict remove to guarantee branch deletion viability and prevent recurring deferred merges.
- **Best-effort orphan cleanup in `MergeBack` vs hard-fail:** choose best-effort to avoid turning successful merges into failures due to cosmetic cleanup issues.
- **Deterministic branch→dir derivation vs scanning filesystem/worktree list:** choose deterministic derivation to avoid broad matching and reduce accidental cleanup risk.

## Test Strategy

## Test Strategy

**Test Levels:**
1. **Unit tests (`cmd/gromit/interactive_worktree_test.go`)**: Verify lifecycle ordering and conflict-handoff behavior in immediate path.
2. **Unit tests (`internal/worktree/worktree_test.go`)**: Verify `MergeBack` performs derived worktree cleanup only after successful merge paths.
3. **Integration-through-epilogue confidence (`internal/pipeline/epilogue/epilogue_test.go`)**: Keep existing coverage; optional targeted assertion that successful `MergeBack` calls are still invoked once per unique branch.

**Key Test Cases:**
- Immediate success path calls cleanup before first `MergeBack`.
- Immediate path aborts and surfaces error when pre-merge `git worktree remove` fails; merge is not attempted.
- Manual conflict handoff retains pending branch but no longer claims conflict resolution occurs in existing session dir.
- Agent retry conflict flow still retries merges correctly, but uses branch-based handoff instructions when exhausted.
- `clearMergedState` only removes pending branch and does not invoke filesystem cleanup.
- `MergeBack` fast-forward success performs:
  - merge
  - branch delete (best effort)
  - derived `git worktree remove` (best effort)
- `MergeBack` regular-merge success does same cleanup sequence.
- `MergeBack` conflict path does not attempt derived worktree remove.
- `MergeBack` ignores derived cleanup errors such as missing/non-worktree path and still returns success.

**Mocking Strategy:**
- Use existing factory overrides in `interactive_worktree_test.go` (`interactiveWorktreeCleanupSessionFn`, mock manager/state recorder) to enforce call ordering and failure behavior.
- Use existing `WithGitRunFn` mock in `worktree_test.go` to assert exact git command sequence and error handling without real git operations.
- No new end-to-end git fixture needed for this scope.

**Coverage Goals:**
- Critical lifecycle invariant: detach worktree before any merge attempt.
- Critical deferred invariant: successful merge should clean up orphan dir opportunistically.
- Edge cases: pre-merge remove failure (strict), post-merge cleanup failures (lenient), conflict message correctness after dir removal.

**Test Organization:**
- Extend existing test files only:
  - `cmd/gromit/interactive_worktree_test.go`
  - `internal/worktree/worktree_test.go`
- Keep naming pattern `Test<Function>_<Behavior>` and table style where already present.

## Implementation Tasks

### Task 1: Reorder Interactive Lifecycle to Pre-Remove Session Worktree

**Files:**
- Modify: `cmd/gromit/interactive_worktree.go`
- Test: `cmd/gromit/interactive_worktree_test.go`

**What to Do:**
- In `runWithSessionWorktreeWithConflictSettings`, invoke `interactiveWorktreeCleanupSessionFn(mainDir, session.WorktreeDir)` immediately after `AddPendingWorktreeBranch` succeeds and before `attemptMergeWithConflictPolicy`.
- Ensure cleanup failure is returned with context and aborts all merge attempts.
- Keep pending branch recorded when pre-removal fails to preserve run-loop fallback and inspection path.

**Acceptance Criteria:**
- `git worktree remove` is attempted before any call to `MergeBack`.
- When pre-removal fails, the function returns an error and `MergeBack` is never called.
- Existing callback and pending-branch recording order remains intact.

**Dependencies:**
- None.

**Notes:**
- Preserve existing error-wrapping style for actionable diagnostics.

### Task 2: Make Merged-State Cleanup State-Only and Update Handoff Messaging

**Files:**
- Modify: `cmd/gromit/interactive_worktree.go`
- Test: `cmd/gromit/interactive_worktree_test.go`

**What to Do:**
- Update `clearMergedState` to only call `RemovePendingWorktreeBranch`; remove session-dir cleanup from this helper.
- Revise `mergeConflictHandoffError.Error()` for manual and agent policies so instructions reference branch-based recovery rather than operating in `SessionDir`.
- Include explicit guidance to re-create a worktree manually when needed (`git worktree add <path> <branch>`), keeping branch name central.

**Acceptance Criteria:**
- Successful immediate merge path clears pending branch without invoking filesystem cleanup from `clearMergedState`.
- Conflict handoff messages no longer direct users to resolve in `SessionDir`.
- Handoff message includes branch-specific, actionable recovery commands.

**Dependencies:**
- Task 1.

**Notes:**
- Keep `mergeConflictHandoffError` fields backward-compatible unless field removal is required and fully covered by tests.

### Task 3: Add Best-Effort Derived Worktree Cleanup to MergeBack

**Files:**
- Modify: `internal/worktree/worktree.go`
- Test: `internal/worktree/worktree_test.go`

**What to Do:**
- Add helper logic to derive expected session worktree dir from a branch name:
  - only for `gromit/<suffix>` branches
  - expected dir: `<MainDir>-gromit-<suffix>`
- In both successful merge branches (ff-only and regular merge), after branch-delete attempt, call best-effort `git worktree remove <expectedDir>`.
- Ignore cleanup failures (including missing/non-worktree cases) and still return success.

**Acceptance Criteria:**
- Successful `MergeBack` calls invoke derived `worktree remove` after merge success.
- Conflict path does not call derived cleanup.
- Cleanup errors do not change successful merge return value.

**Dependencies:**
- None.

**Notes:**
- Keep branch deletion best-effort behavior unchanged.

### Task 4: Validate Deferred Merge Loop Behavior with Updated MergeBack Semantics

**Files:**
- Modify: `internal/pipeline/epilogue/epilogue_test.go` (only if needed)
- Test: `internal/worktree/worktree_test.go`, `cmd/gromit/interactive_worktree_test.go`

**What to Do:**
- Confirm existing epilogue tests still pass with updated `MergeBack` semantics.
- Add or adjust one epilogue-level assertion only if current tests don’t adequately verify merge-loop usage (unique branch merge invocation remains expected behavior).
- Ensure no repeated pending-branch resurfacing behavior remains attributable to undeleted branch/worktree in unit-level simulation.

**Acceptance Criteria:**
- Epilogue merge loop behavior remains unchanged at interface level (`PendingBranches` + `MergeBack`).
- Test suite reflects that deferred successful merges can now clean orphan session dirs indirectly via `MergeBack`.
- No regressions in warning/merge iteration behavior.

**Dependencies:**
- Task 3.

**Notes:**
- This is primarily a verification task; avoid unnecessary production edits if coverage is already sufficient.

### Task 5: Run Focused Quality Gates for Lifecycle and Worktree Packages

**Files:**
- Test: `cmd/gromit/interactive_worktree_test.go`
- Test: `internal/worktree/worktree_test.go`
- Optional test: `internal/pipeline/epilogue/epilogue_test.go`

**What to Do:**
- Run targeted Go tests for modified packages.
- Confirm acceptance criteria coverage for strict pre-removal failures and lenient post-merge cleanup failures.
- Capture any residual risk if full-suite execution is deferred.

**Acceptance Criteria:**
- All modified-package tests pass locally.
- New/updated tests assert call order and cleanup behavior explicitly.
- Any unrun broader checks are documented in handoff notes.

**Dependencies:**
- Tasks 1-4.

**Notes:**
- Keep test scope targeted first for fast feedback, then expand only if needed.

---

## Notes

- Repository architecture has evolved from `internal/runner/lifecycle.go` to pipeline epilogue orchestration (`internal/pipeline/epilogue/epilogue.go`); this plan targets current code locations while preserving spec intent.
- The branch-to-dir mapping is deterministic and must remain aligned with `sessionWorktreeDir` naming conventions to avoid accidental cleanup drift.
- Strict vs best-effort semantics are intentional and asymmetric:
  - Pre-merge remove in interactive flow: strict, blocking.
  - Post-merge derived cleanup in `MergeBack`: best-effort, non-blocking.
