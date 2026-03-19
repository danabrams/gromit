DONE 2026-03-19
# Spec 0003a — Infrastructure Failure Detection with Worktree Auto-Recovery

## spec_id
0003a-infrastructure-failure-detection

## Depends on
None

## Vision
The Gromit Next pipeline currently cannot distinguish between "the implementation code is wrong" and "the execution environment is broken." When a worktree loses its `.git` file, contract evaluation produces 30+ misleading "file not found" failures, and the system wastes replan cycles trying to fix code that was never broken. The original incident was caused by macOS temp-directory cleanup when worktrees lived under `/tmp/`; that root cause has since been fixed by moving worktrees to `.gromit-next/worktrees/` inside the repo. However, infrastructure detection remains valuable as defense-in-depth against filesystem corruption, manual deletion, disk errors, and other unexpected failures. In the observed incident, the broken worktree burned $9 and 9 replans before blocking. The system needs to detect infrastructure failures early, attempt self-healing, and block with a clear diagnostic when recovery fails — rather than replanning endlessly against an unfixable environment.

## Summary
Add a pre-flight infrastructure health check at the top of the Validate stage that runs before any contract or shell checks. The health check verifies the worktree is a valid git repository with a working Go module. On failure, it attempts auto-recovery by removing and recreating the worktree (using the existing GitOps interface). On successful recovery, validation proceeds normally. On failed recovery, the run blocks immediately with a clear `"infrastructure: ..."` diagnostic in the blocker summary.

## Goals
### Primary
- Detect broken worktrees before contract evaluation to prevent cascading misleading failures
- Auto-recover broken worktrees by removing and recreating them via GitOps
- Block immediately with clear infrastructure diagnostics when recovery fails
- Prevent wasted replan cycles on unfixable environment issues

### Secondary
- Provide clear, actionable blocker summaries that distinguish infrastructure from code issues

## Non-goals
- Auto-recovery for non-worktree infrastructure issues (disk full, permission denied on non-worktree files, network errors) — these block immediately
- New terminal status types (StatusInfraFailure) — use existing StatusBlocked with descriptive blocker_summary
- Recovery for Go module download failures — out of scope
- Deferred: replan context deduplication (0003b), review degradation (0003c), task escalation (0003d)

## Architecture

The validate stage (`internal/next/specloop/stages/validate.go`) gains a pre-flight health check method that runs before any contract or shell checks.

Key design decisions:
- The health check is a method on ValidateStage, not a separate stage — it's logically part of validation
- GitOps interface is injected into ValidateStage (it already exists on other stages)
- Recovery reuses existing `GitOps.RemoveWorktree()` and adds a new `GitOps.RecoverWorktree(repoDir, branch string)` method that calls `git worktree add <path> <existing-branch>` (without `-b`) because the branch already exists from the init stage. The existing `CreateWorktree()` uses `-b` which fails on an existing branch, so it cannot be used for recovery.
- The worktree branch name follows the format `gromit/spec-{SpecID}-{RunID}` (same pattern as InitStage)
- Health check runs every cycle, not just the first — a worktree can break mid-run
- The `go.mod` check is intentionally Go-specific — shell checks (`go test`, `go vet`) require it to function, so a missing `go.mod` is an infrastructure failure for this pipeline
- `RepoDir` comes from the same config source as `InitStageConfig.RepoDir`
- On health check failure + failed recovery, the validate stage sets `rs.BlockerSummary` directly before returning `Blocked`

```go
// ValidateStageConfig gains GitOps and RepoDir fields
type ValidateStageConfig struct {
    // ... existing fields ...
    GitOps  GitOps   // for worktree recovery
    RepoDir string   // main repo path for worktree recreation
}

// Health check method
func (s *ValidateStage) checkWorktreeHealth(workDir string) error {
    // 1. Check directory exists
    // 2. Check .git file exists (worktrees have a .git file, not directory)
    // 3. Check go.mod exists
    // Returns nil if healthy, error describing what's wrong if not
}

// Recovery method
func (s *ValidateStage) recoverWorktree(ctx context.Context, rs *runstore.RunState) error {
    // 1. Remove broken worktree via GitOps.RemoveWorktree() (handles missing dir gracefully)
    // 2. Recreate via GitOps.RecoverWorktree() with existing branch (no -b flag)
    // 3. Update rs.WorktreePath if path changed
    // 4. Emit WorktreeRecoveryEvent
    // Returns nil on success, error on failure
}

// Add RecoverWorktree to the EXISTING GitOps interface in
// internal/next/specloop/stages/init.go. All implementations and fakes
// must be updated to satisfy the new method:
//   RecoverWorktree(repoDir, branch string) (worktreePath string, err error)
// RecoverWorktree re-adds a worktree for an existing branch.
// Uses `git worktree add <path> <branch>` (without -b).

// WorktreeRecoveryEvent is emitted when worktree recovery is attempted.
type WorktreeRecoveryEvent struct {
    HealthCheckFailure string // what the health check found (e.g., ".git file missing")
    RecoverySucceeded  bool   // whether recovery completed successfully
    NewWorktreePath    string // path to the recovered worktree (empty on failure)
}
```

**Note:** `WorktreeRecoveryEvent` must be registered in the event unmarshal switch in `events.go` (same pattern as 0003c's `DiffUnavailableEvent`).

The Run() method changes from:
```
Run() → evaluate contracts → run shell checks → merge failures
```
To:
```
Run() → health check → (recovery if needed) → evaluate contracts → run shell checks → merge failures
```

On health check failure + recovery failure, set `rs.BlockerSummary` to the infrastructure diagnostic (e.g., `"infrastructure: worktree recovery failed: <details>"`) and return `Blocked` action. This ensures the blocker summary is persisted in RunState before the stage returns.

## Acceptance Criteria

1. When the validate stage runs, it checks worktree health (directory exists, `.git` file present, `go.mod` exists) before evaluating contracts or running shell checks
2. When the worktree health check fails, the validate stage attempts recovery by removing and recreating the worktree via GitOps
3. When worktree recovery succeeds, validation proceeds normally (contracts + shell checks run against the recovered worktree)
4. When worktree recovery fails, the validate stage returns `Blocked` with a blocker summary prefixed with `"infrastructure: "`
5. The health check runs on every validation cycle, not just the first
6. ValidateStage accepts GitOps and RepoDir via its config for worktree recovery
7. No contract or shell check failures are generated when the root cause is a broken worktree — the health check catches it first
8. All existing validate stage tests continue to pass
9. The infrastructure blocker summary includes both the health check diagnosis (e.g., ".git file missing") and the recovery error details (e.g., "git worktree add failed: disk full")
10. A `WorktreeRecoveryEvent` is emitted when recovery is attempted, recording whether recovery succeeded and the health check failure reason

## Scenarios

### Scenario: Worktree is healthy, validation proceeds normally
**Given:** A run with a valid worktree at `.gromit-next/worktrees/wt-abc123/` containing `.git` file and `go.mod`
**When:** The validate stage runs
**Then:** The health check passes silently, contracts are evaluated, shell checks run, and the stage returns its normal result (pass, ReplanFrom, or Blocked based on code issues)
**Notes:** This is the common case — health check should be invisible when things are fine

### Scenario: Worktree .git file missing, recovery succeeds
**Given:** A run with worktree at `.gromit-next/worktrees/wt-abc123/` where the `.git` file has been deleted (e.g., by macOS temp cleanup) but the directory still exists with source files
**When:** The validate stage runs
**Then:** The health check detects the missing `.git` file, calls `GitOps.RemoveWorktree()` then `GitOps.RecoverWorktree()` with the existing branch (`gromit/spec-{SpecID}-{RunID}`), updates `rs.WorktreePath` with the new path, and proceeds to evaluate contracts and shell checks against the recovered worktree. A `WorktreeRecoveryEvent` is emitted with `RecoverySucceeded: true`.
**Notes:** The branch already exists from the init stage, so `RecoverWorktree` uses `git worktree add <path> <branch>` (without `-b`). Files from the old worktree are lost — the recovered worktree is a fresh checkout of the branch, which should have committed code from prior cycles.

### Scenario: Worktree directory missing entirely, recovery succeeds
**Given:** A run with worktree path set to `.gromit-next/worktrees/wt-abc123/` but the entire directory has been deleted
**When:** The validate stage runs
**Then:** The health check detects the missing directory, calls `RemoveWorktree()` which handles the missing directory gracefully (returns nil), then calls `RecoverWorktree()` with the existing branch, updates `rs.WorktreePath`, and proceeds normally
**Notes:** `RemoveWorktree` is always called regardless of directory existence — it handles non-existent paths gracefully (returns nil via `os.Stat` check), keeping the recovery flow uniform

### Scenario: Worktree go.mod missing, recovery succeeds
**Given:** A run with worktree at `.gromit-next/worktrees/wt-abc123/` where `.git` exists but `go.mod` is missing (e.g., corrupted checkout)
**When:** The validate stage runs
**Then:** The health check detects the missing `go.mod`, attempts recovery (remove + recreate), and proceeds with validation against the recovered worktree
**Notes:** Missing `go.mod` causes `go test ./...` to fail with "directory prefix . does not contain main module" — this was observed in the incident

### Scenario: Recovery fails, run blocks with infrastructure diagnostic
**Given:** A run with a broken worktree, and `GitOps.RecoverWorktree()` returns an error (e.g., disk full, git corruption)
**When:** The validate stage runs and attempts recovery
**Then:** The stage sets `rs.BlockerSummary` and returns `Blocked` action. No contract or shell check failures are generated. No replan is triggered. A `WorktreeRecoveryEvent` is emitted with `RecoverySucceeded: false`.
**Notes:** The blocker summary must start with `"infrastructure: "` so tooling can distinguish infrastructure from code failures. The diagnostic message varies by failure type:
- `.git` file missing: `"infrastructure: worktree recovery failed: .git file missing in .gromit-next/worktrees/wt-abc123/, recovery error: <details>"`
- `go.mod` missing: `"infrastructure: worktree recovery failed: go.mod not found in .gromit-next/worktrees/wt-abc123/, recovery error: <details>"`
- Directory missing: `"infrastructure: worktree recovery failed: directory does not exist: .gromit-next/worktrees/wt-abc123/, recovery error: <details>"`

### Scenario: Health check passes on second cycle after prior success
**Given:** A run where cycle 1 completed successfully with a healthy worktree, and cycle 2 begins (after a replan triggered by code failures)
**When:** The validate stage runs on cycle 2
**Then:** The health check runs again and passes (worktree still healthy), validation proceeds normally
**Notes:** Health check is not gated by any "already checked" flag — it runs every time

### Scenario: RemoveWorktree fails during recovery
**Given:** A run with a broken worktree (`.git` file missing), and `GitOps.RemoveWorktree()` returns an error (e.g., permission denied on the worktree directory)
**When:** The validate stage runs and attempts recovery
**Then:** Recovery is aborted without calling `RecoverWorktree()`. The stage sets `rs.BlockerSummary` to `"infrastructure: worktree cleanup failed: <error details>"` and returns `Blocked`. A `WorktreeRecoveryEvent` is emitted with `RecoverySucceeded: false`.
**Notes:** If the broken worktree cannot be removed, recreation would also fail. Blocking immediately with the cleanup error gives the operator actionable information.

### Scenario: Worktree breaks between cycles, recovery on cycle 2
**Given:** A run where cycle 1 completed successfully with a healthy worktree at `.gromit-next/worktrees/wt-abc123/`. Between cycle 1 and cycle 2, the worktree's `.git` file is deleted (e.g., filesystem corruption, manual deletion).
**When:** The validate stage runs on cycle 2
**Then:** The health check detects the missing `.git` file, calls `RemoveWorktree()` then `RecoverWorktree()`, updates `rs.WorktreePath`, and proceeds with validation. A `WorktreeRecoveryEvent` is emitted with `RecoverySucceeded: true`.
**Notes:** This is the key reason the health check runs every cycle, not just on the first. The worktree was healthy on cycle 1 but broke before cycle 2 — without per-cycle checks, this would produce cascading contract failures.

## Validation
- `go test ./internal/next/specloop/stages/ -count=1`
- `go test ./cmd/gromit-next/ -count=1`
- `go vet ./...`
