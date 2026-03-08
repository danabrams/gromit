package present

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/pipeline"
	"github.com/danabrams/gromit/internal/v2/presentation"
)

const maxSquashLogEntries = 1000

// squashPerBeadForPresentation creates or resets a dedicated PR branch,
// squashes stage commits there, and restores the original worktree branch.
// The returned branch name should be used as the PR head branch.
func squashPerBeadForPresentation(ctx context.Context, git pipeline.SquashGit, worktree, specID string, beads []presentation.BeadSummary) (string, error) {
	if !canRewriteHistory(ctx, worktree) {
		return "", squashPerBead(ctx, git, worktree, beads)
	}

	sourceBranch, err := currentWorktreeBranch(ctx, worktree)
	if err != nil {
		return "", err
	}
	prBranch := presentation.SpecPRBranchName(specID)
	if prBranch == "" {
		return "", squashPerBead(ctx, git, worktree, beads)
	}

	if err := runGitInWorktree(ctx, worktree, "checkout", "-B", prBranch, sourceBranch); err != nil {
		return "", fmt.Errorf("checkout pr branch %s: %w", prBranch, err)
	}

	squashErr := squashPerBead(ctx, git, worktree, beads)
	restoreErr := runGitInWorktree(ctx, worktree, "checkout", sourceBranch)
	if squashErr != nil {
		if restoreErr != nil {
			return "", fmt.Errorf("squash per bead: %w (restore %s: %v)", squashErr, sourceBranch, restoreErr)
		}
		return "", squashErr
	}
	if restoreErr != nil {
		return "", fmt.Errorf("restore source branch %s: %w", sourceBranch, restoreErr)
	}
	return prBranch, nil
}

// squashPerBead squashes per-stage commits into a single combined commit for PR
// presentation. It is a no-op when the git log contains no structured commits.
func squashPerBead(ctx context.Context, git pipeline.SquashGit, worktree string, beads []presentation.BeadSummary) error {
	entries, err := git.Log(ctx, worktree, maxSquashLogEntries)
	if err != nil {
		return err
	}

	titles := beadTitleMap(beads)
	allowedBeads := beadAllowlist(titles)
	groups := collectBeadGroups(entries, allowedBeads)
	if len(groups) == 0 {
		return nil
	}

	if canRewriteHistory(ctx, worktree) {
		return squashWithHistoryRewrite(ctx, worktree, entries, titles, allowedBeads)
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

func collectBeadGroups(entries []adapter.LogEntry, allowedBeads map[string]struct{}) []beadGroup {
	var groups []beadGroup
	for _, entry := range entries {
		info, ok := pipeline.ParseCommitMessage(entry.Message)
		if !ok {
			break
		}
		if !isSquashCandidate(info, allowedBeads) {
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

func squashWithHistoryRewrite(ctx context.Context, worktree string, entries []adapter.LogEntry, titles map[string]string, allowedBeads map[string]struct{}) error {
	prefix := structuredPrefix(entries)
	if len(prefix) == 0 {
		return nil
	}

	baseIdx := len(prefix)
	if baseIdx >= len(entries) {
		return fmt.Errorf("structured commits require a non-structured base commit")
	}
	baseHash := entries[baseIdx].Hash
	segments := buildSquashSegments(prefix, allowedBeads)
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
		if _, ok := pipeline.ParseCommitMessage(entry.Message); !ok {
			break
		}
		prefix = append(prefix, entry)
	}
	return prefix
}

func buildSquashSegments(prefix []adapter.LogEntry, allowedBeads map[string]struct{}) []squashSegment {
	order := make([]string, 0, len(prefix))
	seen := make(map[string]struct{}, len(prefix))
	hashesByBead := make(map[string][]string, len(prefix))
	for i := len(prefix) - 1; i >= 0; i-- {
		entry := prefix[i]
		info, _ := pipeline.ParseCommitMessage(entry.Message)
		if !isSquashCandidate(info, allowedBeads) {
			continue
		}
		if _, ok := seen[info.BeadID]; !ok {
			seen[info.BeadID] = struct{}{}
			order = append(order, info.BeadID)
		}
		hashesByBead[info.BeadID] = append(hashesByBead[info.BeadID], entry.Hash)
	}

	if len(order) == 0 {
		return nil
	}

	segments := make([]squashSegment, 0, len(order))
	for _, beadID := range order {
		segments = append(segments, squashSegment{
			beadID: beadID,
			hashes: hashesByBead[beadID],
		})
	}
	return segments
}

func isSquashCandidate(info pipeline.CommitInfo, allowedBeads map[string]struct{}) bool {
	if strings.TrimSpace(info.BeadID) == "" {
		return false
	}
	if _, ok := allowedBeads[info.BeadID]; !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(info.StageName)) {
	case "build", "validate", "review", "present":
		return true
	default:
		return false
	}
}

func canRewriteHistory(ctx context.Context, worktree string) bool {
	if strings.TrimSpace(worktree) == "" {
		return false
	}
	cmd := exec.CommandContext(ctx, "git", "-C", worktree, "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

func currentWorktreeBranch(ctx context.Context, worktree string) (string, error) {
	branch, err := runGitOutputInWorktree(ctx, worktree, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve current branch: %w", err)
	}
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("worktree %s is not on a named branch", worktree)
	}
	return branch, nil
}

func runGitInWorktree(ctx context.Context, worktree string, args ...string) error {
	_, err := runGitOutputInWorktree(ctx, worktree, args...)
	return err
}

func runGitOutputInWorktree(ctx context.Context, worktree string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", worktree}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func beadTitleMap(beads []presentation.BeadSummary) map[string]string {
	titles := make(map[string]string, len(beads))
	for _, bead := range beads {
		titles[bead.ID] = strings.TrimSpace(bead.Title)
	}
	return titles
}

func beadAllowlist(titles map[string]string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(titles))
	for beadID := range titles {
		if strings.TrimSpace(beadID) == "" {
			continue
		}
		allowed[beadID] = struct{}{}
	}
	return allowed
}

func beadCommitMessage(beadID string, titles map[string]string) string {
	title := strings.TrimSpace(titles[beadID])
	if title == "" {
		title = beadID
	}
	return fmt.Sprintf("bead %s: %s", beadID, title)
}
