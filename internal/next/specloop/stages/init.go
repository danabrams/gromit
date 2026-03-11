package stages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
	cfg   InitStageConfig
	store *runstore.Store
}

// NewInitStage creates a new InitStage.
func NewInitStage(cfg InitStageConfig, store *runstore.Store) *InitStage {
	return &InitStage{cfg: cfg, store: store}
}

// Name returns the stage name.
func (s *InitStage) Name() string { return "init" }

// Run executes the init stage.
func (s *InitStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	// Create run directory
	runDir := s.store.RunDir(rs.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return specloop.NextAction{}, fmt.Errorf("create run dir: %w", err)
	}

	// Create git worktree
	branch := fmt.Sprintf("gromit/spec-%s-%s", rs.SpecID, rs.RunID)
	worktreePath, err := s.cfg.GitOps.CreateWorktree(s.cfg.RepoDir, branch)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("create worktree: %w", err)
	}
	rs.WorktreePath = worktreePath

	// Copy spec file into run dir
	specData, err := os.ReadFile(s.cfg.SpecPath)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("read spec: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "spec.md"), specData, 0o644); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write spec: %w", err)
	}

	// Snapshot execution policy into run dir
	policyData, err := os.ReadFile(s.cfg.PolicyPath)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("read policy: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "execution-policy.json"), policyData, 0o644); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write policy: %w", err)
	}

	// Save initial run state
	if err := s.store.Save(rs); err != nil {
		return specloop.NextAction{}, fmt.Errorf("save run state: %w", err)
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}
