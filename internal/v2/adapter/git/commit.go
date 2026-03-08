package git

import (
	"context"
	"fmt"
	"strings"

	commitmsg "github.com/danabrams/gromit/internal/v2/commit"
)

// StageCommitterGit is the git interface required by StageCommitter.
type StageCommitterGit interface {
	Status(ctx context.Context, worktree string) (string, error)
	Commit(ctx context.Context, worktree, message string) (string, error)
}

// StageCommitter creates a structured git commit after each stage completes.
// It is a no-op when the worktree has no uncommitted changes.
type StageCommitter struct {
	Git StageCommitterGit
}

// CommitStage checks for uncommitted changes and creates a structured commit
// if changes are present.
func (sc *StageCommitter) CommitStage(ctx context.Context, worktree, beadID, stageName string, iteration int, decision string) error {
	if sc == nil || sc.Git == nil {
		return fmt.Errorf("git adapter required")
	}

	status, err := sc.Git.Status(ctx, worktree)
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		return nil
	}

	normalizedBeadID := strings.TrimSpace(beadID)
	if normalizedBeadID == "" {
		normalizedBeadID = "spec"
	}
	message := commitmsg.FormatMessage(normalizedBeadID, strings.TrimSpace(stageName), iteration, strings.TrimSpace(decision))
	_, err = sc.Git.Commit(ctx, worktree, message)
	return err
}
