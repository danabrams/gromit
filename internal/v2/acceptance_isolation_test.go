//go:build acceptance
// +build acceptance

package v2

import (
    "context"
    "testing"
)

func TestSpecExecutionCreatesIsolatedWorktree(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    git := &spyGitAdapter{}
    executor := NewSpecExecutor(git)

    _, err := executor.Execute(ctx, "spec-123", nil)
    if err != nil {
        t.Fatalf("Execute() returned error: %v", err)
    }

    if git.calledCreateWorktree == 0 {
        t.Fatalf("expected GitAdapter to be asked for a worktree")
    }

    if got, want := git.lastSpecID, "spec-123"; got != want {
        t.Fatalf("last spec ID = %q, want %q", got, want)
    }
}

type spyGitAdapter struct {
    calledCreateWorktree int
    lastSpecID           string
}

func (s *spyGitAdapter) CreateIsolatedWorktree(ctx context.Context, specID string) (string, error) {
    s.calledCreateWorktree++
    s.lastSpecID = specID
    return "/tmp/spec-worktree", nil
}
