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

	groups := collectBeadGroups(entries)
	if len(groups) == 0 {
		return nil
	}

	titles := beadTitleMap(beads)
	for _, group := range groups {
		msg := beadCommitMessage(group.beadID, titles)
		if err := git.SquashCommits(ctx, worktree, group.count); err != nil {
			return fmt.Errorf("squash commits for bead %s: %w", group.beadID, err)
		}
		if _, err := git.Commit(ctx, worktree, msg); err != nil {
			return fmt.Errorf("commit squash for bead %s: %w", group.beadID, err)
		}
	}
	return nil
}

type beadGroup struct {
	beadID string
	count  int
}

func collectBeadGroups(entries []adapter.LogEntry) []beadGroup {
	var groups []beadGroup
	for _, entry := range entries {
		info, ok := ParseCommitMessage(entry.Message)
		if !ok {
			break
		}
		if info.BeadID == "" {
			continue
		}
		if len(groups) == 0 || groups[len(groups)-1].beadID != info.BeadID {
			groups = append(groups, beadGroup{beadID: info.BeadID, count: 1})
			continue
		}
		groups[len(groups)-1].count++
	}
	return groups
}

func beadTitleMap(beads []presentation.BeadSummary) map[string]string {
	titles := make(map[string]string, len(beads))
	for _, bead := range beads {
		titles[bead.ID] = strings.TrimSpace(bead.Title)
	}
	return titles
}

func beadCommitMessage(beadID string, titles map[string]string) string {
	title := strings.TrimSpace(titles[beadID])
	if title == "" {
		title = beadID
	}
	return fmt.Sprintf("bead %s: %s", beadID, title)
}
