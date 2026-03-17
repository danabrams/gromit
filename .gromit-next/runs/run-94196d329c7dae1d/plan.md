# Plan (Cycle 1)

## t-001

Add RecoverWorktree method to the GitOps interface in init.go. This method takes repoDir and branch strings, returns (worktreePath string, err error). It uses 'git worktree add <path> <branch>' without -b since the branch already exists.

## t-002

Update fakeGitOps in init_test.go to implement the new RecoverWorktree method. Add recoverBranch, recoverWorktreePath, and recoverErr fields. The method should record the branch and return the configured path/error.

## t-003

Add WorktreeRecoveryEvent struct to events.go with fields: HealthCheckFailure string, RecoverySucceeded bool, NewWorktreePath string. Register 'worktree_recovery' in the unmarshalEvent switch.

## t-004

Add GitOps and RepoDir fields to ValidateStageConfig. Update NewValidateStage to accept these via the config struct. No signature change needed — they come through the config.

## t-005

Write tests for the checkWorktreeHealth method in validate_test.go. Test cases: (1) healthy worktree with .git and go.mod passes, (2) missing directory returns error, (3) missing .git file returns error, (4) missing go.mod returns error. Use t.TempDir() to create real filesystem fixtures.

## t-006

Implement checkWorktreeHealth method on ValidateStage. Checks: (1) directory exists via os.Stat, (2) .git file exists in directory, (3) go.mod exists in directory. Returns nil if healthy, descriptive error if not.

## t-007

Write tests for the full Run() integration with health check and recovery. Test cases: (1) healthy worktree proceeds to normal validation (existing tests still pass), (2) broken worktree + successful recovery proceeds to validation, (3) broken worktree + failed recovery returns Blocked with 'infrastructure: ' prefix in BlockerSummary, (4) WorktreeRecoveryEvent emitted on recovery attempt with correct fields, (5) RemoveWorktree failure during recovery returns Blocked with cleanup error, (6) health check runs every cycle (not gated by prior success). Tests use fakeGitOps for GitOps and fakeValidator for validation.

## t-008

Implement recoverWorktree method on ValidateStage. Steps: (1) call GitOps.RemoveWorktree — if it fails, return error with 'worktree cleanup failed' message, (2) call GitOps.RecoverWorktree with repoDir and branch — if it fails, return error, (3) update rs.WorktreePath with new path. Also construct the branch name as 'gromit/spec-{SpecID}-{RunID}' matching InitStage pattern.

## t-009

Wire health check into the Run() method of ValidateStage. Before contract evaluation: (1) run checkWorktreeHealth, (2) on failure, attempt recoverWorktree, (3) emit WorktreeRecoveryEvent with result, (4) on recovery success, update workDir and proceed, (5) on recovery failure, set rs.BlockerSummary with 'infrastructure: ' prefix and return Blocked. Ensure no contract/shell checks run when infrastructure fails.

## t-010

Update the stage_provider.go caller in cmd/gromit-next/ to pass GitOps and RepoDir in ValidateStageConfig. GitOps comes from the same gitOps used by InitStage; RepoDir comes from p.cfg.RepoDir or equivalent config field.

## t-011

Update any other callers of NewValidateStage that pass ValidateStageConfig (in contract_integration_test.go, write_contracts_integration_test.go, write_contracts_scenario_test.go) to still compile — the new GitOps/RepoDir fields are optional (zero-value means no health check recovery possible, which is fine for existing tests that don't test infrastructure recovery). Verify all tests pass.

