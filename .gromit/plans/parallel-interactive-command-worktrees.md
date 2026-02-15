---
created: 2026-02-15T00:00:00Z
decomposed: true
decomposed_at: "2026-02-15T20:17:40Z"
id: parallel-interactive-command-worktrees
source_spec: parallel-interactive-command-worktrees
---

# Parallel Interactive Command Worktrees Implementation Plan

**Goal:** Run interactive/authoring commands in isolated per-session worktrees so they can execute concurrently and merge safely back to the main branch.

**Architecture:** Add a shared session worktree launcher for CLI commands, track pending session branches in interactive state, and perform immediate merge-back with configurable conflict handling (`agent` or `manual`) plus runner merge-back as a safety net.

**Tech Stack:** Go, Cobra CLI, git worktree/merge operations, JSON state persistence.

**Spec:** `.gromit/specs/parallel-interactive-command-worktrees.md`

---

## Architecture

**Overview:**
Use per-session worktrees for `refine`, `plan`, `explore`, `debug`, interactive `review`, `retro`, and `decompose --review`. Each invocation gets a unique worktree path and branch. On completion, merge immediately; if conflicts happen, either run agent-assisted resolution or preserve for manual resolution.

**Key Components:**
1. **Worktree Session Manager (`internal/worktree`)**: Create unique session worktree paths and branch names per invocation, and expose cleanup operations for merged sessions.
2. **Interactive Session State (`internal/state/interactive_state.go`)**: Persist pending interactive branches and bounded recent session metadata for deterministic merge bookkeeping.
3. **Config Extensions (`internal/config/config.go`)**: Add conflict-resolution controls and defaults for always-on command isolation behavior.
4. **Shared Command Launcher (`cmd/gromit/interactive_worktree.go`)**: Centralize session creation, launch-in-dir execution, immediate merge, conflict handling, state updates, and user-facing status/errors.
5. **Pipeline Agent Launch-in-Dir (`internal/pipeline`)**: Extend pipeline agent interface and workflow code to support `LaunchInDir` so existing refine/explore/review interactive paths run in session worktrees without global chdir.
6. **Runner Merge Safety Net (`internal/runner`)**: Continue merge-back during run iterations; prefer interactive-state pending branch list with fallback behavior for compatibility.

**Integration Points:**
- Existing command entrypoints under `cmd/gromit` currently call `Launch(...)`; they will route through a shared isolated launcher and `LaunchInDir(...)`.
- `decompose --review` remains semantically the same from a user perspective but executes in its session worktree and participates in the same merge/conflict flow.
- Existing run-loop merge settings (`worktree.auto_merge`, `worktree.merge_failure`) remain authoritative for runner-side merge behavior.

**Data Flow:**
1. CLI command starts.
2. Determine project root and decide isolation policy (for this feature set, default to isolated sessions when worktrees are enabled).
3. Create session worktree and branch.
4. Record pending branch/session in `interactive-state.json`.
5. Execute command workflow in session directory.
6. Attempt immediate merge-back.
7. On merge conflict:
   - `agent` mode: run agent-assisted conflict resolution (bounded retries), then retry merge.
   - `manual` mode or exhausted retries: keep worktree/branch and print explicit manual steps.
8. On successful merge: remove pending branch, clean up session worktree/branch.
9. Runner merge pass can still merge pending branches not handled yet.

**Files to Modify:**
- `internal/worktree/worktree.go`
- `internal/worktree/worktree_test.go`
- `internal/state/interactive_state.go`
- `internal/state/interactive_state_test.go`
- `internal/config/config.go`
- `internal/config/worktree_config_test.go`
- `internal/runner/lifecycle.go`
- `cmd/gromit/plan.go`
- `cmd/gromit/refine.go`
- `cmd/gromit/explore.go`
- `cmd/gromit/debug.go`
- `cmd/gromit/review.go`
- `cmd/gromit/main.go`
- `cmd/gromit/decompose.go`
- `internal/pipeline/pipeline.go`
- `internal/pipeline/refine.go`
- `internal/pipeline/explore.go`
- `internal/pipeline/review_test.go` (and related pipeline test mocks)

**Files to Create:**
- `cmd/gromit/interactive_worktree.go`
- `cmd/gromit/interactive_worktree_test.go`

**Tradeoffs:**
- **Per-session vs shared worktree:** per-session chosen to eliminate index/worktree contention and support true parallel invocations.
- **Immediate merge + runner safety net vs runner-only merge:** immediate merge reduces stale branches and shortens feedback loops; runner merge remains fallback protection.
- **Agent conflict resolution + manual fallback vs manual-only:** agent path reduces user burden for routine conflicts while preserving deterministic manual recovery.
- **State-backed pending branch tracking vs branch-scan only:** explicit pending list is more reliable than heuristic branch scans and supports cleanup/reporting.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: worktree session naming/branch uniqueness, state bookkeeping, conflict policy selection/retry bounds, config defaults/parsing.
2. **Integration Tests**: command launch path isolation, immediate merge and cleanup behavior, conflict branches preserved when unresolved, run-loop pending branch merge fallback.
3. **Manual Testing**: concurrent command runs with distinct worktrees, forced conflict paths, agent-assisted and manual conflict outcomes.

**Key Test Cases:**
- Two concurrent interactive commands produce different worktree paths and branches.
- `refine`, `plan`, `explore`, `debug`, interactive `review`, `retro`, and `decompose --review` launch from session worktree directories.
- Successful immediate merge removes pending state and cleans worktree/branch.
- Conflict in `agent` mode triggers bounded resolution attempts and retries merge.
- Conflict in `manual` mode preserves worktree/branch and prints clear next actions.
- `worktree.enabled=false` preserves legacy in-place behavior.
- Runner merge pass can merge pending interactive branches from state.

**Mocking Strategy:**
- Mock worktree manager/git operations for deterministic merge/conflict tests.
- Mock agent launch outcomes for conflict-resolution retry flows.
- Use real `interactive-state.json` serialization in state migration/compatibility tests.
- Keep command wiring tests focused on launch dir behavior and state transitions.

**Coverage Goals:**
- Critical path: session creation -> launch -> pending-state update -> merge/conflict handling -> cleanup.
- Edge cases: corrupted/missing state file, duplicate pending branches, cleanup errors after successful merge, retry exhaustion.

**Test Organization:**
- `internal/worktree/*_test.go`: session creation and merge mechanics.
- `internal/state/*_test.go`: new pending/session fields and backward compatibility.
- `cmd/gromit/interactive_worktree_test.go`: orchestration and policy behavior.
- Existing command tests: verify worktree launch integration.
- `internal/runner/*_test.go`: pending branch merge behavior.

## Implementation Tasks

### Task 1: Extend Worktree and Config Primitives

**Files:**
- Modify: `internal/worktree/worktree.go`
- Modify: `internal/config/config.go`
- Test: `internal/worktree/worktree_test.go`
- Test: `internal/config/worktree_config_test.go`

**What to Do:**
Implement per-session worktree creation primitives and new config fields for conflict handling and session behavior defaults. Ensure naming is unique and deterministic enough for debugging.

**Acceptance Criteria:**
- Worktree manager can create unique per-session worktrees and branches for a command invocation.
- Config supports conflict-resolution mode and retry cap with documented defaults.
- Existing worktree defaults remain backward-compatible when new fields are unset.

**Dependencies:**
- None

**Notes:**
- Keep branch/worktree naming aligned with spec shape (`gromit/<command>-<timestamp>-<rand>` and `<project>-gromit-<command>-<session-id>`).

### Task 2: Add Interactive Pending Branch Session Bookkeeping

**Files:**
- Modify: `internal/state/interactive_state.go`
- Test: `internal/state/interactive_state_test.go`

**What to Do:**
Add additive interactive-state fields and helper methods for pending branch add/remove/list, plus optional bounded recent session tracking.

**Acceptance Criteria:**
- `interactive-state.json` persists `pending_worktree_branches` and loads cleanly when fields are absent.
- Add/remove helpers are deduplicated and safe under file-locking.
- Existing state tests still pass with backward compatibility preserved.

**Dependencies:**
- Task 1

**Notes:**
- Keep fields additive and nil-normalized.

### Task 3: Build Shared CLI Session Launcher with Merge/Conflict Orchestration

**Files:**
- Create: `cmd/gromit/interactive_worktree.go`
- Test: `cmd/gromit/interactive_worktree_test.go`

**What to Do:**
Implement reusable command-side orchestration: choose isolation, create session worktree/branch, run callback in session dir, record pending branch, merge immediately, handle conflicts via `agent`/`manual` policy, and cleanup/report.

**Acceptance Criteria:**
- Helper supports command callback execution in session directory.
- Successful path merges immediately and cleans pending state/worktree artifacts.
- Conflict path supports agent-assisted retry and manual fallback with clear actionable output.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
- Keep helper API small so each command can adopt it with minimal changes.

### Task 4: Wire Plan/Refine/Explore/Debug to Session Launcher

**Files:**
- Modify: `cmd/gromit/plan.go`
- Modify: `cmd/gromit/refine.go`
- Modify: `cmd/gromit/explore.go`
- Modify: `cmd/gromit/debug.go`
- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/pipeline/refine.go`
- Modify: `internal/pipeline/explore.go`

**What to Do:**
Route command execution through the shared session launcher. Upgrade pipeline agent interface and implementations to use `LaunchInDir` so these workflows execute from the session worktree.

**Acceptance Criteria:**
- Commands launch their agent workflows in assigned session dirs when worktree isolation is enabled.
- Pipeline agent interface supports `LaunchInDir` without breaking existing behavior.
- Existing non-isolated behavior remains when worktree is disabled.

**Dependencies:**
- Task 3

**Notes:**
- Update associated pipeline and command tests as part of this task’s implementation.

### Task 5: Wire Review (Interactive), Retro, and Decompose --Review

**Files:**
- Modify: `cmd/gromit/review.go`
- Modify: `cmd/gromit/main.go`
- Modify: `cmd/gromit/decompose.go`
- Modify: command tests in corresponding `*_test.go` files

**What to Do:**
Apply the same session launcher to interactive review and retro command flows, and run `decompose --review` execution in a session worktree with the same merge/conflict behavior.

**Acceptance Criteria:**
- Interactive review and retro execute from session worktrees and participate in immediate merge flow.
- `decompose --review` uses session worktree execution and same conflict handling policy.
- Conflicts in these commands honor `agent` vs `manual` modes.

**Dependencies:**
- Task 3

**Notes:**
- Preserve command semantics and user prompts; only execution directory/isolation/merge handling should change.

### Task 6: Integrate Runner Pending-Branch Consumption with Interactive State

**Files:**
- Modify: `internal/runner/lifecycle.go`
- Test: runner worktree merge tests (`internal/runner/*worktree*_test.go`, related)

**What to Do:**
Update runner merge pass to consume pending branches from interactive state first, with compatibility fallback behavior, and keep merge failure policy handling unchanged.

**Acceptance Criteria:**
- Runner attempts merge for pending interactive branches recorded in state.
- Successful merges remove pending branch state.
- Merge failures preserve pending state and honor `merge_failure` behavior.

**Dependencies:**
- Task 2
- Task 5

**Notes:**
- This task is a safety-net integration; command immediate merge remains primary.

### Task 7: End-to-End Verification and Cleanup Guardrails

**Files:**
- Modify: targeted tests across `cmd/gromit`, `internal/pipeline`, `internal/runner`, `internal/state`, `internal/worktree` as needed

**What to Do:**
Close gaps, stabilize flaky paths, and verify concurrent command isolation + merge/conflict handling end-to-end with focused test coverage.

**Acceptance Criteria:**
- Targeted and impacted test suites pass for modified areas.
- Concurrent session behavior is validated (distinct branches/worktrees, no shared index contention).
- Manual fallback instructions are accurate and sufficient for conflict resolution.

**Dependencies:**
- Task 4
- Task 5
- Task 6

**Notes:**
- Keep coverage focused on regression-prone orchestration paths.

---

## Notes

- This plan intentionally treats these commands as isolated authoring sessions, not as “special interactive” one-offs.
- Immediate merge-back is primary; runner merge-back remains a secondary recovery path.
- Conflict policy is dual-mode by design: agent-assisted first when configured, manual preservation always available.
