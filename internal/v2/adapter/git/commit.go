package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

const eventsLogPath = ".gromit/v2/events.jsonl"

// CommitWorktree stages tracked changes (including the events log) and creates a git commit with the provided message.
func CommitWorktree(ctx context.Context, worktree, message string) (string, error) {
	trimmed := strings.TrimSpace(worktree)
	if trimmed == "" {
		return "", fmt.Errorf("worktree required")
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return "", fmt.Errorf("commit message required")
	}

	if out, err := runGitCommand(ctx, trimmed, "add", "-A"); err != nil {
		return "", fmt.Errorf("git add: %s: %w", out, err)
	}

	eventPath := filepath.Join(trimmed, eventsLogPath)
	if _, statErr := os.Stat(eventPath); statErr == nil {
		if out, err := runGitCommand(ctx, trimmed, "add", "-f", "--", eventsLogPath); err != nil {
			return "", fmt.Errorf("git add events log: %s: %w", out, err)
		}
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("stat events log: %w", statErr)
	}

	if out, err := runGitCommand(ctx, trimmed, "commit", "-m", msg); err != nil {
		return "", fmt.Errorf("git commit: %s: %w", out, err)
	}
	out, err := runGitCommand(ctx, trimmed, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %s: %w", out, err)
	}
	return strings.TrimSpace(string(out)), nil
}
