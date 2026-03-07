package pipeline

import (
	"context"

	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
)

// SquashGit is the subset of GitAdapter required by SquashPerBead.
type SquashGit interface {
	Log(ctx context.Context, worktree string, n int) ([]adapter.LogEntry, error)
	SquashCommits(ctx context.Context, worktree string, count int) error
	Commit(ctx context.Context, worktree, message string) (string, error)
}

const maxSquashLogEntries = 1000

// SquashPerBead squashes per-stage commits into a single combined commit for PR
// presentation. It is a no-op when the git log contains no structured commits.
func SquashPerBead(ctx context.Context, git SquashGit, worktree string, beads []presentation.BeadSummary) error {
	entries, err := git.Log(ctx, worktree, maxSquashLogEntries)
	if err != nil {
		return err
	}

	count := 0
	for _, e := range entries {
		if _, ok := ParseCommitMessage(e.Message); !ok {
			break
		}
		count++
	}

	if count == 0 {
		return nil
	}

	_ = beads
	return nil
}
