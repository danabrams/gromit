package benchmark

import (
	"context"
	"fmt"
	"os/exec"
	stdstrings "strings"
)

type BaseCommitResolver interface {
	ResolveBaseCommit(ctx context.Context, baseCommitHint string) (string, error)
}

type GitRunner func(ctx context.Context, args ...string) (string, error)

type GitBaseCommitResolver struct {
	runGit GitRunner
}

func NewGitBaseCommitResolver(runGit GitRunner) *GitBaseCommitResolver {
	if runGit == nil {
		runGit = defaultGitRunner
	}
	return &GitBaseCommitResolver{runGit: runGit}
}

func (r *GitBaseCommitResolver) ResolveBaseCommit(ctx context.Context, baseCommitHint string) (string, error) {
	ref := stdstrings.TrimSpace(baseCommitHint)
	if ref == "" {
		ref = "HEAD"
	}
	out, err := r.runGit(ctx, "rev-parse", "--verify", ref)
	if err != nil {
		return "", fmt.Errorf("resolve base commit %q: %w", ref, err)
	}
	commit := stdstrings.TrimSpace(out)
	if commit == "" {
		return "", fmt.Errorf("resolve base commit %q: empty revision", ref)
	}
	return commit, nil
}

func defaultGitRunner(_ context.Context, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

type ModeWorktreeRequest struct {
	Mode          string
	BaseCommit    string
	SelectedBeads []string
	Overlay       ModeOverlay
}

type ModeWorktreeRun struct {
	Mode    string
	Cleanup func() error
}

type ModeWorktreeRunner interface {
	RunMode(ctx context.Context, req ModeWorktreeRequest) (ModeWorktreeRun, error)
}
