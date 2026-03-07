package pipeline

import (
	"context"
	"fmt"
	"strings"

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

	msg := squashMessage(beads)
	if err := git.SquashCommits(ctx, worktree, count); err != nil {
		return fmt.Errorf("squash commits: %w", err)
	}
	if _, err := git.Commit(ctx, worktree, msg); err != nil {
		return fmt.Errorf("commit squash: %w", err)
	}
	return nil
}

func squashMessage(beads []presentation.BeadSummary) string {
	if len(beads) == 1 {
		return fmt.Sprintf("bead %s: %s", beads[0].ID, beads[0].Title)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("squash %d beads", len(beads)))
	for _, bead := range beads {
		b.WriteString(fmt.Sprintf("\n- bead %s: %s", bead.ID, bead.Title))
	}
	return b.String()
}
