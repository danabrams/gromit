package stages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// GitOps abstracts git worktree operations for stage implementations.
type GitOps interface {
	CreateWorktree(repoDir, branch string) (worktreePath string, err error)
	RemoveWorktree(path string) error
}

// InitStageConfig configures the InitStage.
type InitStageConfig struct {
	SpecPath   string
	PolicyPath string
	RepoDir    string
	GitOps     GitOps
}

// InitStage initializes a run: creates run dir, git worktree, copies spec and policy.
type InitStage struct {
	cfg      InitStageConfig
	store    *runstore.Store
	eventLog *runstore.EventLog
}

// NewInitStage creates a new InitStage.
func NewInitStage(cfg InitStageConfig, store *runstore.Store, eventLog *runstore.EventLog) *InitStage {
	return &InitStage{cfg: cfg, store: store, eventLog: eventLog}
}

// Name returns the stage name.
func (s *InitStage) Name() string { return "init" }

// Run executes the init stage.
func (s *InitStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	// Create run directory first so that the event log (which lives inside it)
	// can be written by cleanBlockedWorktrees when emitting blocked_worktree_cleaned.
	runDir := s.store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return specloop.NextAction{}, fmt.Errorf("create run dir: %w", err)
	}

	// Clean up prior blocked worktrees for the same spec
	if err := s.cleanBlockedWorktrees(rs); err != nil {
		return specloop.NextAction{}, fmt.Errorf("clean blocked worktrees: %w", err)
	}

	// Create git worktree
	branch := fmt.Sprintf("gromit/spec-%s-%s", rs.SpecID, rs.RunID)
	worktreePath, err := s.cfg.GitOps.CreateWorktree(s.cfg.RepoDir, branch)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("create worktree: %w", err)
	}
	rs.WorktreePath = worktreePath

	// Clean up worktree if any subsequent step fails
	cleanup := func() {
		s.cfg.GitOps.RemoveWorktree(worktreePath)
		rs.WorktreePath = ""
	}

	// Copy spec file into run dir
	specData, err := os.ReadFile(s.cfg.SpecPath)
	if err != nil {
		cleanup()
		return specloop.NextAction{}, fmt.Errorf("read spec: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "spec.md"), specData, 0o644); err != nil {
		cleanup()
		return specloop.NextAction{}, fmt.Errorf("write spec: %w", err)
	}

	// Snapshot execution policy into run dir
	policyData, err := os.ReadFile(s.cfg.PolicyPath)
	if err != nil {
		cleanup()
		return specloop.NextAction{}, fmt.Errorf("read policy: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "execution-policy.json"), policyData, 0o644); err != nil {
		cleanup()
		return specloop.NextAction{}, fmt.Errorf("write policy: %w", err)
	}

	// Save initial run state
	if err := s.store.Save(rs); err != nil {
		return specloop.NextAction{}, fmt.Errorf("save run state: %w", err)
	}

	// Emit run_started event
	if s.eventLog != nil {
		s.eventLog.Append(runstore.RunStartedEvent{
			BaseEvent: runstore.BaseEvent{Type: "run_started", Timestamp: time.Now()},
			SpecID:    rs.SpecID,
			ProjectID: rs.ProjectID,
		})
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}

// cleanBlockedWorktrees removes worktree directories for prior blocked runs
// with the same spec and project, clearing the path in the store and emitting events.
func (s *InitStage) cleanBlockedWorktrees(rs *runstore.RunState) error {
	runs, err := s.store.List(rs.ProjectID)
	if err != nil {
		return err
	}
	for _, prior := range runs {
		if prior.RunID == rs.RunID {
			continue
		}
		if prior.SpecID != rs.SpecID || prior.Status != runstore.StatusBlocked || prior.WorktreePath == "" {
			continue
		}
		// Capture path before clearing
		cleanedPath := prior.WorktreePath
		// Use GitOps.RemoveWorktree for consistency with the rest of the codebase.
		// Falls back to os.RemoveAll if GitOps is not configured.
		if s.cfg.GitOps != nil {
			if err := s.cfg.GitOps.RemoveWorktree(cleanedPath); err != nil {
				return fmt.Errorf("remove worktree %s: %w", cleanedPath, err)
			}
		} else if err := os.RemoveAll(cleanedPath); err != nil {
			return fmt.Errorf("remove worktree %s: %w", cleanedPath, err)
		}
		// Clear worktree path and save
		prior.WorktreePath = ""
		if err := s.store.Save(prior); err != nil {
			return fmt.Errorf("save run %s: %w", prior.RunID, err)
		}
		// Emit event
		if s.eventLog != nil {
			s.eventLog.Append(runstore.BlockedWorktreeCleanedEvent{
				BaseEvent:    runstore.BaseEvent{Type: "blocked_worktree_cleaned", Timestamp: time.Now()},
				PriorRunID:   prior.RunID,
				WorktreePath: cleanedPath,
			})
		}
	}
	return nil
}
