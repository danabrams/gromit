package benchmark

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/worktree"
)

func TestGitBaseCommitResolver_ResolvesHintWithRevParse(t *testing.T) {
	t.Parallel()

	calls := make([][]string, 0, 1)
	resolver := NewGitBaseCommitResolver(func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return "abc123\n", nil
	})

	got, err := resolver.ResolveBaseCommit(context.Background(), "feature-branch")
	if err != nil {
		t.Fatalf("ResolveBaseCommit() error = %v", err)
	}
	if got != "abc123" {
		t.Fatalf("ResolveBaseCommit() = %q, want %q", got, "abc123")
	}
	if len(calls) != 1 {
		t.Fatalf("git call count = %d, want 1", len(calls))
	}
	if len(calls[0]) != 3 || calls[0][0] != "rev-parse" || calls[0][1] != "--verify" || calls[0][2] != "feature-branch" {
		t.Fatalf("git args = %v, want [rev-parse --verify feature-branch]", calls[0])
	}
}

func TestSessionModeWorktreeRunner_RunModeExecutesInSessionAndReturnsCleanup(t *testing.T) {
	t.Parallel()

	session := &worktree.SessionWorktree{
		BranchName:  "gromit/benchmark-single_pass-1",
		WorktreeDir: "/tmp/repo-wt",
	}
	var executedReq ModeWorktreeRequest
	executedDir := ""
	cleaned := false

	runner := NewSessionModeWorktreeRunner(SessionModeWorktreeRunnerOptions{
		MainDir: "/tmp/repo",
		CreateSessionWorktree: func(command string) (*worktree.SessionWorktree, error) {
			if command != "benchmark-single_pass" {
				t.Fatalf("command = %q, want %q", command, "benchmark-single_pass")
			}
			return session, nil
		},
		CheckoutBaseCommitInWorktree: func(_ context.Context, _, _ string) error { return nil },
		RunModeInWorktree: func(_ context.Context, worktreeDir string, req ModeWorktreeRequest) error {
			executedDir = worktreeDir
			executedReq = req
			return nil
		},
		CleanupSession: func(mainDir, sessionDir string) error {
			if mainDir != "/tmp/repo" {
				t.Fatalf("cleanup mainDir = %q, want %q", mainDir, "/tmp/repo")
			}
			if sessionDir != session.WorktreeDir {
				t.Fatalf("cleanup sessionDir = %q, want %q", sessionDir, session.WorktreeDir)
			}
			cleaned = true
			return nil
		},
	})

	req := ModeWorktreeRequest{
		Mode:          "single_pass",
		BaseCommit:    "abc123",
		SelectedBeads: []string{"gromit-1", "gromit-2"},
	}
	run, err := runner.RunMode(context.Background(), req)
	if err != nil {
		t.Fatalf("RunMode() error = %v", err)
	}
	if executedDir != session.WorktreeDir {
		t.Fatalf("executed worktreeDir = %q, want %q", executedDir, session.WorktreeDir)
	}
	if executedReq.Mode != req.Mode || executedReq.BaseCommit != req.BaseCommit {
		t.Fatalf("executed req = %+v, want mode/base commit from request", executedReq)
	}
	if len(executedReq.SelectedBeads) != 2 || executedReq.SelectedBeads[0] != "gromit-1" || executedReq.SelectedBeads[1] != "gromit-2" {
		t.Fatalf("executed selected beads = %v, want [gromit-1 gromit-2]", executedReq.SelectedBeads)
	}
	if cleaned {
		t.Fatal("cleanup called before cleanup callback execution")
	}
	if run.Cleanup == nil {
		t.Fatal("Cleanup callback = nil, want non-nil")
	}
	if err := run.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if !cleaned {
		t.Fatal("cleanup callback was not invoked")
	}
}

func TestSessionModeWorktreeRunner_RunModeChecksOutBaseCommitBeforeExecution(t *testing.T) {
	t.Parallel()

	session := &worktree.SessionWorktree{
		BranchName:  "gromit/benchmark-single_pass-2",
		WorktreeDir: "/tmp/repo-wt",
	}
	sequence := make([]string, 0, 2)

	runner := NewSessionModeWorktreeRunner(SessionModeWorktreeRunnerOptions{
		MainDir: "/tmp/repo",
		CreateSessionWorktree: func(command string) (*worktree.SessionWorktree, error) {
			return session, nil
		},
		CheckoutBaseCommitInWorktree: func(_ context.Context, worktreeDir, baseCommit string) error {
			if worktreeDir != session.WorktreeDir {
				t.Fatalf("checkout worktreeDir = %q, want %q", worktreeDir, session.WorktreeDir)
			}
			if baseCommit != "abc123" {
				t.Fatalf("checkout baseCommit = %q, want %q", baseCommit, "abc123")
			}
			sequence = append(sequence, "checkout")
			return nil
		},
		RunModeInWorktree: func(_ context.Context, _ string, _ ModeWorktreeRequest) error {
			sequence = append(sequence, "run")
			return nil
		},
		CleanupSession: func(_, _ string) error { return nil },
	})

	_, err := runner.RunMode(context.Background(), ModeWorktreeRequest{
		Mode:       "single_pass",
		BaseCommit: "abc123",
	})
	if err != nil {
		t.Fatalf("RunMode() error = %v", err)
	}
	if len(sequence) != 2 || sequence[0] != "checkout" || sequence[1] != "run" {
		t.Fatalf("execution sequence = %v, want [checkout run]", sequence)
	}
}
