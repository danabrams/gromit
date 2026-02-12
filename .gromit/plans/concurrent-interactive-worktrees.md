---
created: 2026-02-12T00:00:00Z
decomposed: true
decomposed_at: "2026-02-12T16:38:33-05:00"
id: concurrent-interactive-worktrees
source_spec: concurrent-interactive-worktrees
---

# Concurrent Interactive Sessions via Git Worktrees — Implementation Plan

**Goal:** Enable `gromit retro` and `gromit review` to run concurrently with `gromit run` by isolating interactive sessions in a separate git worktree.

**Architecture:** A new `internal/worktree/` package manages a persistent sibling worktree. Interactive commands detect an active run loop (via status.json PID liveness) and launch Claude Code in the worktree. State is split into run-loop-owned and interactive-owned files. Completed interactive branches merge back between run loop iterations.

**Tech Stack:** Go, git worktrees, existing cobra CLI, existing agent/provider/state infrastructure

**Spec:** `.gromit/specs/concurrent-interactive-worktrees.md`

---

## Architecture

**Overview:** Three layers — worktree manager (lifecycle + merge), command integration (retro/review detect and use worktree), and run loop coordination (merge-back between iterations). State splits into two files to prevent concurrent-write corruption.

**Key Components:**

1. **`internal/worktree/` package** — `Manager` struct with lifecycle methods (`EnsureWorktree`, `CreateBranch`, `Cleanup`) and merge methods (`MergeBack`, `PendingBranches`). `IsRunLoopActive(gromitDir)` function for detection.

2. **`internal/state/interactive_state.go`** — `InteractiveState` struct and `InteractiveFile` type owning `last_retro`, `last_review_commit`, `last_review_iteration`, `filtered_learning_hashes`. Loaded from `interactive-state.json`.

3. **`internal/config/config.go` WorktreeConfig** — `Enabled`, `AutoMerge`, `MergeFailure` fields with defaults (true, true, "warn").

4. **Agent `LaunchInDir`** — New method on `Agent` interface allowing retro/review to launch Claude Code in the worktree directory.

**Integration Points:**

- `retro.LaunchClaudeCode()` gains a `dir` parameter; `cmd/gromit/retro.go` checks `IsRunLoopActive` and passes worktree path when active
- `cmd/gromit/review.go` interactive path: resolves agent, calls `LaunchInDir` with worktree path when run loop active
- `internal/runner/runner.go` line ~599: new `mergeInteractiveBranches()` call after `runGitAutoPush()`, before `runBetweenIterationsCommand()`
- All callers of state interactive fields migrate from `state.File` to `state.InteractiveFile`

**Data Flow (interactive session):**

1. User runs `gromit retro` → reads `status.json` → PID alive → run loop active
2. `worktree.Manager.EnsureWorktree()` creates/verifies `<project>-gromit-interactive`
3. `Manager.CreateBranch("retro")` creates `gromit/retro-<timestamp>` in worktree
4. Claude Code launched with CWD = worktree path
5. User edits files, session ends, changes committed to branch
6. Between iterations, runner calls `Manager.PendingBranches()` + `MergeBack()`

**Files to Modify:**

- `internal/agent/agent.go` — Add `LaunchInDir` to interface + `cliAgent` implementation
- `internal/config/config.go` — Add `WorktreeConfig` struct, field, defaults
- `internal/state/state.go` — Remove interactive fields from `State`, update `NormalizeNilFields`
- `internal/runner/runner.go` — Add merge-back call, load InteractiveFile for review baseline/retro suggestion
- `internal/runner/interfaces.go` — Add `WorktreeManager` interface if needed
- `cmd/gromit/retro.go` — Add worktree detection + pass dir to LaunchClaudeCode
- `cmd/gromit/review.go` — Add worktree detection + LaunchInDir
- `internal/retro/retro.go` — `LaunchClaudeCode` gains `dir` parameter, sets `cmd.Dir`
- `gromit.yaml` — Add `worktree:` config section with comments

**Files to Create:**

- `internal/worktree/worktree.go` — Manager struct + lifecycle + merge methods
- `internal/worktree/detect.go` — `IsRunLoopActive` function
- `internal/worktree/worktree_test.go` — Unit tests for Manager
- `internal/worktree/detect_test.go` — Detection tests
- `internal/state/interactive_state.go` — InteractiveState struct + InteractiveFile type
- `internal/state/interactive_state_test.go` — Tests

**Tradeoffs:**

- **`LaunchInDir` on Agent interface vs external `cmd.Dir` manipulation**: Adding to the interface is cleaner than having callers get `*exec.Cmd` via `Command()` and set Dir themselves. The interface change is small and well-scoped.
- **State split vs file locking**: Two files with clear ownership is simpler than flock. Cross-worktree flock semantics can be surprising.
- **Sibling directory vs nested**: Sibling avoids `.gitignore` pollution but requires parent directory to be writable.
- **Persistent worktree vs ephemeral**: Persistent avoids repeated git operations and preserves worktree state.

## Test Strategy

**Unit Tests:**
- All `worktree.Manager` methods via mocked git runner (happy path + error cases)
- `IsRunLoopActive` with various status.json states (active, stale PID, missing file, not running)
- `InteractiveState` load/save independently from run-loop `State`
- `WorktreeConfig` defaults and YAML parsing
- Agent `LaunchInDir` sets `cmd.Dir`

**Integration Tests:**
- Real git worktree operations in temp directories (create worktree, create branch, merge back, handle conflicts)
- Runner merge-back orchestration with mock `WorktreeManager`

**Mocking Strategy:**
- `Manager` wraps git via a `gitRunner` function field — tests verify command construction without real git
- Runner tests mock `WorktreeManager` interface via `Fn` field pattern
- Detection tests create temp status.json files with various PID/running states

**Key Test Cases:**
- `EnsureWorktree` creates at correct sibling path, reuses healthy existing, recovers unhealthy
- `CreateBranch` uses `gromit/<cmd>-<timestamp>` naming
- `MergeBack` fast-forwards when possible, merge commit otherwise, errors on conflict
- `PendingBranches` returns only unmerged `gromit/*` branches
- `IsRunLoopActive` correctly distinguishes active/stale/missing/stopped
- Runner merge-back: success, conflict-warn, conflict-stop, no-pending, disabled
- State fields removed from `State` → compile-time verification

---

## Implementation Tasks

### Task 1: Add WorktreeConfig to config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `gromit.yaml`

**What to Do:**
Add `WorktreeConfig` struct with three fields: `Enabled *bool` (default true), `AutoMerge *bool` (default true), `MergeFailure string` (default "warn", valid values "warn"/"stop"). Add `Worktree WorktreeConfig` field to `Config` struct with `yaml:"worktree"` tag. Add convenience methods `IsEnabled()` and `IsAutoMergeEnabled()` handling nil-means-true. Add defaults in `SetDefaults()`. Add commented-out `worktree:` section to `gromit.yaml`.

**Acceptance Criteria:**
- `WorktreeConfig` defaults to enabled=true, auto_merge=true, merge_failure="warn" when section absent from YAML
- YAML with `worktree: { enabled: false }` correctly disables worktrees
- Convenience methods handle nil pointers correctly

**Dependencies:** None

### Task 2: Create InteractiveState type and file

**Files:**
- Create: `internal/state/interactive_state.go`
- Create: `internal/state/interactive_state_test.go`

**What to Do:**
Create `InteractiveState` struct with fields: `LastRetro time.Time`, `LastReviewCommit string`, `LastReviewIteration int`, `FilteredLearningHashes []string`, `UpdatedAt time.Time` (all with json tags matching current state.json field names). Create `InteractiveFile` type with `path string` and `state InteractiveState` fields. Implement `NewInteractiveFile(gromitDir)` (path = `interactive-state.json`), `Load()`, `Save()`, and accessor/mutator methods mirroring the current `state.File` methods for these fields: `LastRetro()`, `RecordRetro()`, `LastReviewCommit()`, `RecordReview()`, `LastReviewIteration()`, `GetFilteredHashes()`, `AddFilteredHashes()`, `ReconcileFilteredHashes()`. Add `NormalizeNilFields()` for the slice.

**Acceptance Criteria:**
- `InteractiveFile` loads/saves `interactive-state.json` independently from `state.json`
- All accessor/mutator methods work correctly (round-trip through save/load)
- `NormalizeNilFields` initializes nil slice to empty

**Dependencies:** None

### Task 3: Create worktree.Manager with lifecycle methods

**Files:**
- Create: `internal/worktree/worktree.go`
- Create: `internal/worktree/worktree_test.go`

**What to Do:**
Create `Manager` struct with `MainDir string`, `WorktreeDir string` (computed as `<MainDir>-gromit-interactive`), and a `gitRunFn func(dir string, args ...string) (string, error)` for testability. Implement:
- `NewManager(mainDir string) *Manager` — sets `WorktreeDir` to sibling path, wires real git runner
- `EnsureWorktree() (string, error)` — if worktree dir doesn't exist, runs `git worktree add <path> -b gromit/interactive --detach` from MainDir. If it exists, verifies `.git` file is present (healthy check). Returns worktree path.
- `CreateBranch(command string) (string, error)` — generates branch name `gromit/<command>-<timestamp>`, runs `git checkout -b <branch>` in worktree dir. Returns branch name.
- `Cleanup() error` — runs `git worktree remove <path>` from MainDir, then `git worktree prune`.

Wire `gitRunFn` as a field so tests can mock git commands and verify arguments without real git repos.

**Acceptance Criteria:**
- `EnsureWorktree` creates worktree at `<project>-gromit-interactive` sibling path
- `EnsureWorktree` reuses existing healthy worktree without recreating
- `CreateBranch` creates correctly-named branch in the worktree
- `Cleanup` removes worktree and prunes

**Dependencies:** None

### Task 4: Add merge operations to worktree.Manager

**Files:**
- Modify: `internal/worktree/worktree.go`
- Modify: `internal/worktree/worktree_test.go`

**What to Do:**
Add two methods to `Manager`:
- `PendingBranches() ([]string, error)` — runs `git branch --list gromit/*` from MainDir, filters to unmerged branches (checks `git branch --merged` to exclude already-merged ones). Returns branch names.
- `MergeBack(branch string) error` — from MainDir, attempts `git merge --ff-only <branch>`. If fast-forward fails, falls back to `git merge --no-edit <branch>`. If merge conflicts, runs `git merge --abort` and returns an error describing the conflict. On success, deletes the branch with `git branch -d <branch>`.

**Acceptance Criteria:**
- `PendingBranches` returns only unmerged `gromit/*` branches, empty list when none
- `MergeBack` fast-forwards when possible, creates merge commit otherwise
- `MergeBack` aborts and returns error on conflict without corrupting working tree

**Dependencies:** Task 3

### Task 5: Add IsRunLoopActive detection

**Files:**
- Create: `internal/worktree/detect.go`
- Create: `internal/worktree/detect_test.go`

**What to Do:**
Create `IsRunLoopActive(gromitDir string) (bool, error)` function. It reads `status.json` from gromitDir, unmarshals to check `Running` field and `PID` field. If `Running == true` and PID is alive (use `os.FindProcess` + `process.Signal(syscall.Signal(0))` — same pattern as `runner.IsProcessAlive`), return true. Return false for: file missing, `Running == false`, PID dead. Errors in file reading/parsing return false with nil error (non-fatal).

**Acceptance Criteria:**
- Returns true when status.json has `running: true` and PID is alive
- Returns false when status.json has `running: true` but PID is dead (stale)
- Returns false when status.json missing or `running: false`

**Dependencies:** None

**Notes:** Reuse the status JSON struct from `runner/status.go` or define a minimal local struct with just the fields needed (`Running bool`, `PID int`). Avoid importing the runner package to prevent circular dependencies.

### Task 6: Add LaunchInDir to Agent interface

**Files:**
- Modify: `internal/agent/agent.go`
- Modify: `internal/agent/resolve.go` (if test file exists, update mocks)

**What to Do:**
Add `LaunchInDir(promptPath string, dir string) error` method to the `Agent` interface. Implement on `cliAgent`: same as `Launch` but sets `cmd.Dir = dir` before running. If `dir` is empty, behaves identically to `Launch` (no Dir set). Update `retro.LaunchClaudeCode` signature to accept an optional `dir string` parameter and set `cmd.Dir` when non-empty.

**Acceptance Criteria:**
- `LaunchInDir` with non-empty dir sets `cmd.Dir` on the subprocess
- `LaunchInDir` with empty dir behaves identically to `Launch`
- `retro.LaunchClaudeCode` accepts dir parameter and passes it through

**Dependencies:** None

**Notes:** Also update `retro.LaunchClaudeCode` in `internal/retro/retro.go` (line 518: `cmd := exec.Command("claude")`) to accept and use a `dir` parameter since retro doesn't go through the agent system.

### Task 7: Migrate state callers to InteractiveState

**Files:**
- Modify: `internal/state/state.go`
- Modify: `internal/state/state_test.go`
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/interfaces.go` (if StateFile-like interface exists in runner)
- Modify: `internal/runner/interfaces_test.go`
- Modify: `cmd/gromit/retro.go` (calls `sf.RecordRetro()`)
- Modify: `cmd/gromit/review.go` (calls `sf.LastReviewCommit()`, wraps in `cliStateManager`)
- Modify: `internal/pipeline/pipeline.go` (StateManager interface)
- Modify: `internal/pipeline/mocks_test.go`
- Modify: `internal/retro/retro.go` (reads FilteredLearningHashes via state)

**What to Do:**
Remove `LastRetro`, `LastReviewCommit`, `LastReviewIteration`, `FilteredLearningHashes` from `State` struct and all their accessor/mutator methods from `state.File`. Update all callers to load and use `state.InteractiveFile` instead:

- **Runner** (`runner.go`): Load `InteractiveFile` alongside `state.File` at loop start. Use it for `LastReviewCommit()` (lines 358, 2038), `RecordReview()`, `LastRetro()` (line 1169), `RecordRetro()` is not called by runner.
- **Retro command** (`cmd/gromit/retro.go`): Create `InteractiveFile`, call `RecordRetro()` on it instead of `state.File`.
- **Review command** (`cmd/gromit/review.go`): Update `cliStateManager` to wrap `InteractiveFile` for `GetLastReviewCommit`/`SetLastReviewCommit`.
- **Pipeline** (`pipeline.go`): `StateManager` interface stays the same (it only has `GetLastReviewCommit`/`SetLastReviewCommit`), but the CLI adapter in `review.go` wraps `InteractiveFile` now.
- **Retro** (`retro.go`): Update `FilteredLearningHashes` access to use `InteractiveFile`.

Add backward-compatible migration: if `interactive-state.json` doesn't exist but `state.json` has the old fields, copy them over on first load. This handles upgrades gracefully.

**Acceptance Criteria:**
- Interactive fields removed from `State` struct (compile-time verification)
- All callers use `InteractiveFile` for interactive fields
- Backward migration copies old fields to new file on first load
- All existing tests pass (with mock updates)

**Dependencies:** Task 2

**Notes:** The runner needs both files — `state.File` for `CleanExit`/`IterationsSinceReview`/provider state, and `InteractiveFile` for `LastReviewCommit`/`LastRetro`. This is fine since concurrent writes are eliminated — the run loop only writes interactive state during its own iteration cycle (thorough review, retro suggestion check), not while an interactive command is simultaneously running.

### Task 8: Add merge-back to runner between-iterations

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/interfaces.go`
- Modify: `internal/runner/interfaces_test.go`
- Modify: `internal/runner/runner_test.go`

**What to Do:**
Add `WorktreeManager` interface to `interfaces.go`:
```go
type WorktreeManager interface {
    PendingBranches() ([]string, error)
    MergeBack(branch string) error
}
```
Add `worktreeMgr WorktreeManager` field to `Runner` struct. Accept it in `Deps` (optional — nil means disabled). In the main loop, after `runGitAutoPush()` (line 594) and before `runBetweenIterationsCommand()` (line 599), add `r.mergeInteractiveBranches()` call. The method:
1. Returns early if `r.worktreeMgr == nil` or `!r.cfg.Worktree.IsAutoMergeEnabled()`
2. Calls `PendingBranches()`
3. For each branch, calls `MergeBack(branch)`
4. On success: log "Merged interactive branch: <branch>"
5. On error: if `cfg.Worktree.MergeFailure == "stop"`, return error (halts loop). Otherwise log warning and continue.

Wire `worktree.NewManager()` in `NewRunner()` when `cfg.Worktree.IsEnabled()`.

**Acceptance Criteria:**
- Merge-back runs between iterations when worktree enabled and auto_merge true
- Merge-back skips when auto_merge false or worktree manager nil
- Merge conflict with merge_failure="warn" logs warning and continues
- Merge conflict with merge_failure="stop" halts the run loop

**Dependencies:** Tasks 1, 3, 4

### Task 9: Wire worktree into retro command

**Files:**
- Modify: `cmd/gromit/retro.go`
- Modify: `internal/retro/retro.go`

**What to Do:**
In `runRetro` (main.go line 177), after loading config and resolving gromitDir:
1. Call `worktree.IsRunLoopActive(gromitDir)`
2. If active AND `cfg.Worktree.IsEnabled()` AND not `--non-interactive`:
   a. Create `worktree.NewManager(projectDir)` (projectDir = parent of gromitDir)
   b. Call `mgr.EnsureWorktree()` to get worktree path
   c. Call `mgr.CreateBranch("retro")` to get branch name
   d. Pass worktree path as `dir` to `retro.LaunchClaudeCode()`
   e. Log that retro is running in worktree
3. If not active: run as today (no behavior change)

Update `retro.LaunchClaudeCode` to set `cmd.Dir = dir` when dir is non-empty (from Task 6).

Set `bead.Client.Dir` to worktree path if bd commands need to resolve in the worktree context.

**Acceptance Criteria:**
- `gromit retro` runs in worktree when run loop active and worktree enabled
- `gromit retro` runs in main worktree when run loop not active (no regression)
- `gromit retro --non-interactive` never uses worktree (it doesn't launch Claude Code)

**Dependencies:** Tasks 1, 3, 5, 6

### Task 10: Wire worktree into review command

**Files:**
- Modify: `cmd/gromit/review.go`

**What to Do:**
In `runReviewInteractive` (review.go line 293), before resolving the agent:
1. Call `worktree.IsRunLoopActive(gromitDir)`
2. If active AND `cfg.Worktree.IsEnabled()`:
   a. Create `worktree.NewManager(projectDir)`
   b. Call `mgr.EnsureWorktree()`
   c. Call `mgr.CreateBranch("review")`
   d. Use `agent.LaunchInDir(promptPath, worktreePath)` instead of `agent.Launch(promptPath)`
   e. Log that review is running in worktree
3. If not active: run as today

Non-interactive review (`runReviewNonInteractive`) is called by the runner itself and runs in the main worktree — no change needed there.

**Acceptance Criteria:**
- Interactive `gromit review` runs in worktree when run loop active
- Interactive `gromit review` runs in main worktree when run loop not active (no regression)
- Non-interactive review is unchanged

**Dependencies:** Tasks 1, 3, 5, 6

---

## Notes

- **bd redirect mechanism**: The spec mentions bd's redirect file for bead sharing across worktrees. The `bead.Client.Dir` field already exists. When launching in a worktree, set `Dir` to the worktree path. bd's own redirect mechanism (`.beads/redirect` file) handles pointing back to the main repo's beads — this may need to be set up during `EnsureWorktree`.

- **State concurrency safety**: The state split eliminates concurrent writes. The runner reads interactive state during its own iteration cycle (not while interactive commands are writing). If timing is very tight (interactive command saves while runner reads), the worst case is a stale read — which is benign (e.g., slightly old `LastReviewCommit` baseline).

- **Cleanup story**: The persistent worktree at `<project>-gromit-interactive` accumulates over time. Consider adding `gromit cleanup` or `gromit worktree cleanup` in a follow-up. For now, `Manager.Cleanup()` is available for programmatic use.

- **Prompt file paths**: When retro/review render prompts to `.gromit/tmp/`, the prompt file path must be accessible from the worktree. Since `.gromit/` is in the main worktree, the path is absolute and works from either directory.

- **LEARNINGS.md and RULES.md paths**: These live in `.gromit/` in the main worktree. When Claude Code runs in the worktree, it sees a different working tree. The prompt should reference absolute paths or the worktree's copy. The worktree shares the git database so committed files are visible, but uncommitted changes in `.gromit/` (like state files) are not. This needs care — either symlink `.gromit/` into the worktree or use absolute paths in prompts.
