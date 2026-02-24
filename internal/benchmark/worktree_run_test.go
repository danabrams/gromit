package benchmark

import (
	"context"
	"errors"
	stdstrings "strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
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

func TestGitBaseCommitResolver_IncludesGitOutputOnResolveFailure(t *testing.T) {
	t.Parallel()

	resolver := NewGitBaseCommitResolver(func(_ context.Context, _ ...string) (string, error) {
		return "fatal: Needed a single revision\n", errors.New("exit status 128")
	})

	_, err := resolver.ResolveBaseCommit(context.Background(), "abc123")
	if err == nil {
		t.Fatal("ResolveBaseCommit() error = nil, want non-nil")
	}
	if !stdstrings.Contains(err.Error(), `resolve base commit "abc123"`) {
		t.Fatalf("error = %q, want base commit context", err)
	}
	if !stdstrings.Contains(err.Error(), "fatal: Needed a single revision") {
		t.Fatalf("error = %q, want git stderr output", err)
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

func TestApplyBenchmarkOverlayToConfig_PinsProviderAndEnforcesTierPolicies(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"openai": {
				Models: map[string]string{
					"low":    "placeholder-low",
					"medium": "placeholder-medium",
					"high":   "placeholder-high",
				},
			},
			"other": {
				Models: map[string]string{
					"low":    "other-low",
					"medium": "other-medium",
					"high":   "other-high",
				},
			},
		},
	}
	overlay, err := BuildModeOverlay(HarnessManifest{
		Provider:        "openai",
		ModelFamily:     "gpt-5",
		LowTierModel:    "gpt-5-mini",
		MediumTierModel: "gpt-5.3-codex",
		HighTierModel:   "gpt-5.3-codex",
	}, "tdd_fresh_context")
	if err != nil {
		t.Fatalf("BuildModeOverlay() error = %v", err)
	}

	got, err := applyBenchmarkOverlayToConfig(cfg, overlay)
	if err != nil {
		t.Fatalf("applyBenchmarkOverlayToConfig() error = %v", err)
	}

	if got.Methodology.BuildStrategy != "tdd" {
		t.Fatalf("build_strategy = %q, want %q", got.Methodology.BuildStrategy, "tdd")
	}
	if !got.Methodology.FreshContextPerCycle {
		t.Fatal("fresh_context_per_cycle = false, want true")
	}
	if got.Methodology.PhaseModels.Build != "low" {
		t.Fatalf("phase_models.build = %q, want %q", got.Methodology.PhaseModels.Build, "low")
	}
	if got.Review.Tier != "high" {
		t.Fatalf("review.tier = %q, want %q", got.Review.Tier, "high")
	}
	if got.Review.Thorough.Tier != "high" {
		t.Fatalf("review.thorough.tier = %q, want %q", got.Review.Thorough.Tier, "high")
	}
	if !got.Validation.IsNonInteractive() {
		t.Fatal("validation.non_interactive = false, want true")
	}
	if got.Providers["openai"].Models["low"] != "gpt-5-mini" {
		t.Fatalf("provider low model = %q, want %q", got.Providers["openai"].Models["low"], "gpt-5-mini")
	}
	if got.Providers["openai"].Models["medium"] != "gpt-5.3-codex" {
		t.Fatalf("provider medium model = %q, want %q", got.Providers["openai"].Models["medium"], "gpt-5.3-codex")
	}
	if got.Providers["openai"].Models["high"] != "gpt-5.3-codex" {
		t.Fatalf("provider high model = %q, want %q", got.Providers["openai"].Models["high"], "gpt-5.3-codex")
	}
	if _, exists := got.Providers["other"]; exists {
		t.Fatal("unexpected non-pinned provider retained in overlay config")
	}
}
