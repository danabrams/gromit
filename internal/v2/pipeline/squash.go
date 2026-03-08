package pipeline

import (
	"context"
	"fmt"
	"os/exec"
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
	if canRewriteHistory(ctx, worktree) {
		return squashWithHistoryRewrite(ctx, worktree, entries, titles)
	}

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

type squashSegment struct {
	beadID string
	hashes []string
}

func squashWithHistoryRewrite(ctx context.Context, worktree string, entries []adapter.LogEntry, titles map[string]string) error {
	prefix := structuredPrefix(entries)
	if len(prefix) == 0 {
		return nil
	}

	baseIdx := len(prefix)
	if baseIdx >= len(entries) {
		return fmt.Errorf("structured commits require a non-structured base commit")
	}
	baseHash := entries[baseIdx].Hash
	segments := buildSquashSegments(prefix)
	if len(segments) == 0 {
		return nil
	}

	if err := runGitInWorktree(ctx, worktree, "reset", "--hard", baseHash); err != nil {
		return fmt.Errorf("reset to base %s: %w", baseHash, err)
	}

	for _, segment := range segments {
		for _, hash := range segment.hashes {
			if err := runGitInWorktree(ctx, worktree, "cherry-pick", "--no-commit", hash); err != nil {
				_ = runGitInWorktree(ctx, worktree, "cherry-pick", "--abort")
				return fmt.Errorf("cherry-pick bead %s commit %s: %w", segment.beadID, hash, err)
			}
		}
		if err := runGitInWorktree(ctx, worktree, "commit", "-m", beadCommitMessage(segment.beadID, titles)); err != nil {
			return fmt.Errorf("commit squash for bead %s: %w", segment.beadID, err)
		}
	}

	return nil
}

func structuredPrefix(entries []adapter.LogEntry) []adapter.LogEntry {
	prefix := make([]adapter.LogEntry, 0, len(entries))
	for _, entry := range entries {
		if _, ok := ParseCommitMessage(entry.Message); !ok {
			break
		}
		prefix = append(prefix, entry)
	}
	return prefix
}

func buildSquashSegments(prefix []adapter.LogEntry) []squashSegment {
	segments := make([]squashSegment, 0, len(prefix))
	pending := make([]string, 0)
	for i := len(prefix) - 1; i >= 0; i-- {
		entry := prefix[i]
		info, _ := ParseCommitMessage(entry.Message)
		if info.BeadID == "" {
			pending = append(pending, entry.Hash)
			continue
		}
		if len(segments) > 0 && segments[len(segments)-1].beadID == info.BeadID && len(pending) == 0 {
			segments[len(segments)-1].hashes = append(segments[len(segments)-1].hashes, entry.Hash)
			continue
		}
		hashes := make([]string, 0, len(pending)+1)
		hashes = append(hashes, pending...)
		pending = pending[:0]
		hashes = append(hashes, entry.Hash)
		segments = append(segments, squashSegment{
			beadID: info.BeadID,
			hashes: hashes,
		})
	}
	return segments
}

func canRewriteHistory(ctx context.Context, worktree string) bool {
	if strings.TrimSpace(worktree) == "" {
		return false
	}
	cmd := exec.CommandContext(ctx, "git", "-C", worktree, "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

func runGitInWorktree(ctx context.Context, worktree string, args ...string) error {
	cmdArgs := append([]string{"-C", worktree}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
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
