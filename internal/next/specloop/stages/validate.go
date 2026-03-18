package stages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// FinalValidator abstracts final validation for testability.
type FinalValidator interface {
	RunFinal(ctx context.Context, alwaysRun []validator.Check, projectChecks []validator.Check, workDir string) (validator.FinalResult, error)
}

// worktreeCleanupError wraps an error from RemoveWorktree so callers can
// distinguish cleanup failures from recreation failures using errors.As.
type worktreeCleanupError struct{ err error }

func (e *worktreeCleanupError) Error() string { return fmt.Sprintf("worktree cleanup failed: %v", e.err) }
func (e *worktreeCleanupError) Unwrap() error { return e.err }

// ValidateStageConfig configures the ValidateStage.
type ValidateStageConfig struct {
	AlwaysRun     []validator.Check
	ProjectChecks []validator.Check
	WorkDir       string
	EvidenceDir   string
	RepoDir       string
}

// ValidateStage runs final validation checks.
type ValidateStage struct {
	validator         FinalValidator
	contractEvaluator contract.ContractEvaluator
	cfg               ValidateStageConfig
	eventLog          *runstore.EventLog
	gitOps            GitOps
}

// NewValidateStage creates a new ValidateStage. An optional ContractEvaluator may be
// provided; if nil, contract checking is skipped.
func NewValidateStage(v FinalValidator, cfg ValidateStageConfig, eventLog *runstore.EventLog, evaluator contract.ContractEvaluator, gitOps GitOps) *ValidateStage {
	return &ValidateStage{validator: v, cfg: cfg, eventLog: eventLog, contractEvaluator: evaluator, gitOps: gitOps}
}

// Name returns the stage name.
func (s *ValidateStage) Name() string { return "validate" }

// Run executes final validation.
func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	workDir := s.cfg.WorkDir
	if rs.WorktreePath != "" {
		workDir = rs.WorktreePath
	}

	// Health check: if using a worktree, verify it's in a healthy state.
	if rs.WorktreePath != "" {
		healthErr := s.checkWorktreeHealth(workDir)
		if healthErr != nil {
			// If GitOps is configured, attempt recovery
			if s.gitOps != nil {
				recoveryErr := s.recoverWorktree(ctx, rs, healthErr)
				if recoveryErr != nil {
					// Recovery failed; distinguish cleanup failure from recreation failure.
					var blockerMsg string
					var cleanupErr *worktreeCleanupError
					if errors.As(recoveryErr, &cleanupErr) {
						blockerMsg = fmt.Sprintf("infrastructure: %v", recoveryErr)
					} else {
						blockerMsg = fmt.Sprintf("infrastructure: worktree recovery failed: %v, recovery error: %v", healthErr, recoveryErr)
					}
					rs.BlockerSummary = blockerMsg
					return specloop.NextAction{Kind: specloop.Blocked}, nil
				}
				// Recovery succeeded; update workDir to the new worktree path
				workDir = rs.WorktreePath
			} else {
				// GitOps is nil; block with infrastructure diagnostic
				rs.BlockerSummary = fmt.Sprintf("infrastructure: worktree health check failed: %v", healthErr)
				return specloop.NextAction{Kind: specloop.Blocked}, nil
			}
		}
	}

	// Collect contract failures first (if EvidenceDir is configured and file exists).
	var failures []string
	if s.cfg.EvidenceDir != "" && s.contractEvaluator != nil {
		contractPath := filepath.Join(s.cfg.EvidenceDir, "scenario-contracts.yaml")
		data, err := os.ReadFile(contractPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return specloop.NextAction{}, fmt.Errorf("read scenario-contracts.yaml: %w", err)
		}
		if err == nil {
			sc, err := contract.ParseContractYAML(string(data))
			if err != nil {
				return specloop.NextAction{}, fmt.Errorf("parse scenario-contracts.yaml: %w", err)
			}
			contractFailures, err := s.contractEvaluator.Evaluate(ctx, &sc, workDir)
			if err != nil {
				return specloop.NextAction{}, fmt.Errorf("evaluate contracts: %w", err)
			}
			for _, f := range contractFailures {
				failures = append(failures, fmt.Sprintf("contract:%s — %s failed: %s", f.ScenarioName, f.AssertionType, f.Details))
			}
		}
	}

	// Run shell checks regardless of contract results.
	// Scenario test failures are detected via the always-run 'go test ./...' check
	// and reported through the standard go test output format.
	result, err := s.validator.RunFinal(ctx, s.cfg.AlwaysRun, s.cfg.ProjectChecks, workDir)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("final validation: %w", err)
	}

	// Collect shell check failures.
	for _, cr := range result.AlwaysRun.FailedChecks() {
		failures = append(failures, fmt.Sprintf("always-run check %q failed: %s", cr.Name, cr.Output))
	}
	for _, cr := range result.ProjectChecks.FailedChecks() {
		failures = append(failures, fmt.Sprintf("project check %q failed: %s", cr.Name, cr.Output))
	}

	// Determine final validation status after collecting ALL failures (contract + shell).
	finalPassed := len(failures) == 0 && result.Pass

	// Patch the result to reflect the actual final status (contract failures may
	// have turned a shell-passing run into an overall failure).
	result.Pass = finalPassed
	rs.LastFinalValidation = &result

	// Store validation result summary reflecting actual final status.
	validationSummary := fmt.Sprintf("pass=%v", finalPassed)
	rs.LastValidationResult = &validationSummary

	// Emit final_validation_result event after all failures are collected.
	if s.eventLog != nil {
		s.eventLog.Append(runstore.FinalValidationResultEvent{
			BaseEvent: runstore.BaseEvent{Type: "final_validation_result", Timestamp: time.Now()},
			Passed:    finalPassed,
		})
	}

	if finalPassed {
		rs.FinalValidationPassed = true
		return specloop.NextAction{Kind: specloop.Continue}, nil
	}

	if len(failures) == 0 {
		failures = []string{"validation failed"}
	}

	return specloop.NextAction{
		Kind: specloop.ReplanFrom, // validation failure replan action
		Context: &specloop.FailureContext{
			Failures: failures,
			Cycle:    rs.Cycle,
		},
	}, nil
}

// checkWorktreeHealth verifies that the worktree directory is healthy.
// Returns nil if all checks pass, or an error describing what's wrong.
func (s *ValidateStage) checkWorktreeHealth(workDir string) error {
	// Check if directory exists
	if _, err := os.Stat(workDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("directory does not exist: %s", workDir)
		}
		return err
	}

	// Check if .git file exists
	gitPath := filepath.Join(workDir, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(".git file missing in %s", workDir)
		}
		return err
	}

	// Check if go.mod exists
	goModPath := filepath.Join(workDir, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("go.mod not found in %s", workDir)
		}
		return err
	}

	return nil
}

// recoverWorktree attempts to recover a failed worktree by removing the existing
// one and creating a fresh worktree. It emits WorktreeRecoveryEvent and updates
// rs.WorktreePath on success.
func (s *ValidateStage) recoverWorktree(_ context.Context, rs *runstore.RunState, healthErr error) error {
	// Step 1: Remove the old worktree
	if err := s.gitOps.RemoveWorktree(rs.WorktreePath); err != nil {
		if s.eventLog != nil {
			s.eventLog.Append(runstore.WorktreeRecoveryEvent{
				BaseEvent:          runstore.BaseEvent{Type: "worktree_recovery", Timestamp: time.Now()},
				HealthCheckFailure: healthErr.Error(),
				RecoverySucceeded:  false,
			})
		}
		return &worktreeCleanupError{err: err}
	}

	// Step 2: Derive the branch name
	branch := fmt.Sprintf("gromit/spec-%s-%s", rs.SpecID, rs.RunID)

	// Step 3: Recover (recreate) the worktree
	newWorktreePath, err := s.gitOps.RecoverWorktree(s.cfg.RepoDir, branch)
	if err != nil {
		if s.eventLog != nil {
			s.eventLog.Append(runstore.WorktreeRecoveryEvent{
				BaseEvent:          runstore.BaseEvent{Type: "worktree_recovery", Timestamp: time.Now()},
				HealthCheckFailure: healthErr.Error(),
				RecoverySucceeded:  false,
			})
		}
		return err
	}

	// Step 4: On success, update rs.WorktreePath and emit success event
	rs.WorktreePath = newWorktreePath
	if s.eventLog != nil {
		s.eventLog.Append(runstore.WorktreeRecoveryEvent{
			BaseEvent:          runstore.BaseEvent{Type: "worktree_recovery", Timestamp: time.Now()},
			HealthCheckFailure: healthErr.Error(),
			RecoverySucceeded:  true,
			NewWorktreePath:    newWorktreePath,
		})
	}
	return nil
}
