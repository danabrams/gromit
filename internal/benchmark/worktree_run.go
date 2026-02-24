package benchmark

import (
	"context"
	"fmt"
	"os/exec"
	stdstrings "strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/worktree"
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

type SessionModeWorktreeRunnerOptions struct {
	MainDir string

	CreateSessionWorktree func(command string) (*worktree.SessionWorktree, error)
	CheckoutBaseCommitInWorktree func(ctx context.Context, worktreeDir, baseCommit string) error
	RunModeInWorktree     func(ctx context.Context, worktreeDir string, req ModeWorktreeRequest) error
	CleanupSession        func(mainDir, sessionDir string) error
}

type SessionModeWorktreeRunner struct {
	mainDir               string
	createSessionWorktree func(command string) (*worktree.SessionWorktree, error)
	checkoutBaseCommitInWorktree func(ctx context.Context, worktreeDir, baseCommit string) error
	runModeInWorktree     func(ctx context.Context, worktreeDir string, req ModeWorktreeRequest) error
	cleanupSession        func(mainDir, sessionDir string) error
}

func NewSessionModeWorktreeRunner(opts SessionModeWorktreeRunnerOptions) *SessionModeWorktreeRunner {
	r := &SessionModeWorktreeRunner{
		mainDir:               opts.MainDir,
		createSessionWorktree: opts.CreateSessionWorktree,
		checkoutBaseCommitInWorktree: opts.CheckoutBaseCommitInWorktree,
		runModeInWorktree:     opts.RunModeInWorktree,
		cleanupSession:        opts.CleanupSession,
	}
	if r.createSessionWorktree == nil {
		r.createSessionWorktree = func(command string) (*worktree.SessionWorktree, error) {
			manager, err := worktree.NewManager(r.mainDir)
			if err != nil {
				return nil, err
			}
			return manager.CreateSessionWorktree(command)
		}
	}
	if r.runModeInWorktree == nil {
		r.runModeInWorktree = func(_ context.Context, _ string, _ ModeWorktreeRequest) error { return nil }
	}
	if r.checkoutBaseCommitInWorktree == nil {
		r.checkoutBaseCommitInWorktree = defaultCheckoutBaseCommitInWorktree
	}
	if r.cleanupSession == nil {
		r.cleanupSession = defaultSessionCleanup
	}
	return r
}

func (r *SessionModeWorktreeRunner) RunMode(ctx context.Context, req ModeWorktreeRequest) (ModeWorktreeRun, error) {
	session, err := r.createSessionWorktree("benchmark-" + req.Mode)
	if err != nil {
		return ModeWorktreeRun{}, fmt.Errorf("create session worktree for mode %q: %w", req.Mode, err)
	}
	if err := r.checkoutBaseCommitInWorktree(ctx, session.WorktreeDir, req.BaseCommit); err != nil {
		return ModeWorktreeRun{
			Mode: req.Mode,
			Cleanup: func() error {
				return r.cleanupSession(r.mainDir, session.WorktreeDir)
			},
		}, fmt.Errorf("checkout base commit for mode %q: %w", req.Mode, err)
	}

	if err := r.runModeInWorktree(ctx, session.WorktreeDir, req); err != nil {
		return ModeWorktreeRun{
			Mode: req.Mode,
			Cleanup: func() error {
				return r.cleanupSession(r.mainDir, session.WorktreeDir)
			},
		}, fmt.Errorf("run mode %q in worktree: %w", req.Mode, err)
	}

	return ModeWorktreeRun{
		Mode: req.Mode,
		Cleanup: func() error {
			return r.cleanupSession(r.mainDir, session.WorktreeDir)
		},
	}, nil
}

func defaultCheckoutBaseCommitInWorktree(ctx context.Context, worktreeDir, baseCommit string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", "--detach", baseCommit)
	cmd.Dir = worktreeDir
	if output, err := cmd.CombinedOutput(); err != nil {
		msg := stdstrings.TrimSpace(string(output))
		if msg == "" {
			return fmt.Errorf("checkout base commit %q in %q: %w", baseCommit, worktreeDir, err)
		}
		return fmt.Errorf("checkout base commit %q in %q: %w: %s", baseCommit, worktreeDir, err, msg)
	}
	return nil
}

func defaultSessionCleanup(mainDir, sessionDir string) error {
	cmd := exec.Command("git", "worktree", "remove", sessionDir)
	cmd.Dir = mainDir
	if output, err := cmd.CombinedOutput(); err != nil {
		msg := stdstrings.TrimSpace(string(output))
		if msg == "" {
			return fmt.Errorf("remove session worktree %q: %w", sessionDir, err)
		}
		return fmt.Errorf("remove session worktree %q: %w: %s", sessionDir, err, msg)
	}
	return nil
}

func applyBenchmarkOverlayToConfig(cfg *config.Config, overlay ModeOverlay) (*config.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	cloned := *cfg
	cloned.Methodology.BuildStrategy = overlay.BuildStrategy
	cloned.Methodology.FreshContextPerCycle = overlay.FreshContextPerCycle
	cloned.Methodology.PhaseModels.Build = overlay.BuildTierDefault
	cloned.Methodology.PhaseModels.Red = overlay.BuildTierDefault
	cloned.Methodology.PhaseModels.Green = overlay.BuildTierDefault
	cloned.Methodology.PhaseModels.Refactor = overlay.BuildTierDefault

	cloned.Review.Enabled = overlay.FinalReview.Enabled
	cloned.Review.Tier = overlay.FinalReview.Tier
	cloned.Review.Thorough.Enabled = overlay.FinalReview.Enabled
	cloned.Review.Thorough.Tier = overlay.FinalReview.Tier

	validationNonInteractive := true
	cloned.Validation.NonInteractive = &validationNonInteractive
	runFinalFullGate := true
	cloned.Validation.RunFinalFullGate = &runFinalFullGate

	// Pin to one provider and manifest tier models.
	pinned := config.ProviderDef{
		Models: map[string]string{
			"low":    overlay.TierModels.Low,
			"medium": overlay.TierModels.Medium,
			"high":   overlay.TierModels.High,
		},
	}
	if existing, ok := cfg.Providers[overlay.Provider]; ok {
		pinned = existing
		if pinned.Models == nil {
			pinned.Models = map[string]string{}
		}
		pinned.Models["low"] = overlay.TierModels.Low
		pinned.Models["medium"] = overlay.TierModels.Medium
		pinned.Models["high"] = overlay.TierModels.High
	}
	cloned.Providers = map[string]config.ProviderDef{
		overlay.Provider: pinned,
	}
	if cloned.Routing.PhasePreferences == nil {
		cloned.Routing.PhasePreferences = map[string]string{}
	}
	for _, phase := range []string{"build", "review", "thorough_review", "decompose"} {
		cloned.Routing.PhasePreferences[phase] = overlay.Provider
	}
	cloned.Routing.Ratio = map[string]int{overlay.Provider: 100}

	return &cloned, nil
}
