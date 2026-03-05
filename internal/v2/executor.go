package v2

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/config"
)

// GitAdapter encapsulates git operations required for v2 spec execution.
type GitAdapter interface {
	CreateIsolatedWorktree(ctx context.Context, specID string) (string, error)
}

// SpecExecutor coordinates execution tasks for a single spec.
type SpecExecutor struct {
	git GitAdapter
}

// NewSpecExecutor constructs an executor with the provided git adapter.
func NewSpecExecutor(git GitAdapter) *SpecExecutor {
	return &SpecExecutor{git: git}
}

// Execute prepares an isolated worktree and runs the spec hooks.
func (s *SpecExecutor) Execute(ctx context.Context, specID string, cfg *config.Config) (string, error) {
	if s.git == nil {
		return "", fmt.Errorf("git adapter required")
	}

	worktree, err := s.git.CreateIsolatedWorktree(ctx, specID)
	if err != nil {
		return "", err
	}

	return worktree, nil
}
