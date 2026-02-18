package runner

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/retro"
	"github.com/danabrams/gromit/internal/state"
)

// runSessionEpilogue runs the post-loop epilogue when cfg.Session.Iterations > 0.
// It sequences three phases: test-fix loop, review, and retro.
// Returns epilogueRanRetro = true when the retro phase executed (so the caller
// can skip the interactive checkRetroSuggestion).
func (r *Runner) runSessionEpilogue(ctx context.Context, st *runLoopState) (epilogueRanRetro bool, err error) {
	if r == nil || r.cfg == nil {
		return false, nil
	}
	if r.cfg.Session.Iterations == 0 {
		return false, nil
	}

	r.log("Running session epilogue...")

	if err := r.runTestFixLoop(ctx); err != nil {
		r.log("Warning: test-fix loop failed: %v", err)
	}

	if enabledWithDefault(r.cfg.Session.Review) {
		if err := r.runEpilogueReview(ctx, st); err != nil {
			r.log("Warning: epilogue review failed: %v", err)
		}
	}

	if enabledWithDefault(r.cfg.Session.Retro) {
		if err := r.runEpilogueRetro(ctx); err != nil {
			r.log("Warning: epilogue retro failed: %v", err)
		} else {
			epilogueRanRetro = true
		}
	}

	return epilogueRanRetro, nil
}

// runTestFixLoop runs cfg.Session.TestCommand and retries with LLM-generated fixes
// up to cfg.Session.MaxFixRetries times. On exhaustion, creates P0 beads.
func (r *Runner) runTestFixLoop(ctx context.Context) error {
	if r == nil || r.cfg == nil {
		return nil
	}
	if r.cfg.Session.TestCommand == "" {
		return nil
	}

	testCmd := r.cfg.Session.TestCommand
	maxRetries := r.cfg.Session.MaxFixRetries
	fixTier := r.cfg.Session.FixTier
	if fixTier == "" {
		fixTier = provider.TierMedium
	}

	// Initial test run
	stdout, stderr, exitCode, err := r.runCmd(ctx, testCmd, "")
	if err != nil {
		return fmt.Errorf("running test command: %w", err)
	}
	if exitCode == 0 {
		r.log("Session tests passed")
		return nil
	}

	// Tests failed — attempt LLM-guided fixes
	testOutput := stdout + stderr
	r.log("Session tests failed (exit %d); attempting fixes...", exitCode)

	for attempt := 0; attempt < maxRetries; attempt++ {
		r.log("Fix attempt %d/%d...", attempt+1, maxRetries)

		if err := r.applyTestFix(ctx, testCmd, testOutput, fixTier); err != nil {
			r.log("Warning: fix attempt %d failed: %v", attempt+1, err)
		}

		// Re-run tests
		stdout, stderr, exitCode, err = r.runCmd(ctx, testCmd, "")
		if err != nil {
			return fmt.Errorf("re-running test command: %w", err)
		}
		if exitCode == 0 {
			r.log("Session tests passed after %d fix attempt(s)", attempt+1)
			return nil
		}
		testOutput = stdout + stderr
	}

	// Tests still failing after all retries — create P0 beads
	r.log("Session tests still failing after %d retries; creating residual failure bead", maxRetries)
	if r.beads != nil {
		title := "Fix residual test failures from session epilogue"
		desc := fmt.Sprintf("Session test command failed after %d fix attempts.\n\nTest command: %s\n\nFailure output:\n%s",
			maxRetries, testCmd, testOutput)
		_, err := r.beads.CreateWithParentAndDescription(title, 0, []string{"from-epilogue"}, nil, "", desc)
		if err != nil {
			r.log("Warning: failed to create residual failure bead: %v", err)
		}
	}
	return nil
}

func enabledWithDefault(flag *bool) bool {
	return flag == nil || *flag
}

// applyTestFix renders a fix prompt and calls the provider to fix failing tests.
func (r *Runner) applyTestFix(ctx context.Context, testCmd, testOutput, fixTier string) error {
	if r.renderer == nil || r.router == nil {
		return fmt.Errorf("renderer or router is nil")
	}

	claudeMD, _ := r.renderer.LoadClaudeMD()
	rules, _ := r.renderer.LoadRules()

	fixCtx := &prompt.TestFixContext{
		ClaudeMD:          claudeMD,
		Rules:             rules,
		TestCommand:       testCmd,
		TestFailureOutput: testOutput,
	}

	fixPrompt, err := r.renderer.RenderTestFix(fixCtx)
	if err != nil {
		return fmt.Errorf("rendering fix prompt: %w", err)
	}

	p, _ := r.router.Select("build", fixTier)
	if p == nil {
		return fmt.Errorf("no provider available for tier %s", fixTier)
	}

	_, err = p.StreamRun(ctx, fixPrompt, fixTier, r.output, nil, nil)
	if err != nil {
		return fmt.Errorf("running fix provider: %w", err)
	}

	return nil
}

// runEpilogueReview delegates to r.reviewer.RunThorough for a post-session review.
func (r *Runner) runEpilogueReview(ctx context.Context, st *runLoopState) error {
	if r == nil || r.reviewer == nil {
		return nil
	}
	r.log("Running epilogue review...")
	if st != nil && st.interactiveFile != nil {
		r.reviewer.RunThorough(ctx, st.interactiveFile, 0, time.Time{}, r.getHead)
	}
	return nil
}

// runEpilogueRetro selects a high-tier provider, runs retrospective analysis,
// applies the proposals, and records the retro in state.
func (r *Runner) runEpilogueRetro(ctx context.Context) error {
	if r == nil || r.cfg == nil {
		return nil
	}
	if r.router == nil {
		return nil
	}

	r.log("Running epilogue retro...")

	// Select high-tier provider
	p, _ := r.router.Select("build", provider.TierHigh)
	if p == nil {
		r.log("Warning: no provider available for retro; skipping")
		return nil
	}

	// Wrap provider in retro.ProviderRunner adapter
	adapter := &epilogueRetroAdapter{p: p}

	retroRunner, err := retro.NewRetroWithProvider(adapter, r.gromitDir)
	if err != nil {
		return fmt.Errorf("creating retro runner: %w", err)
	}

	result, err := retroRunner.Run(ctx, nil)
	if err != nil {
		return fmt.Errorf("running retro: %w", err)
	}

	// Parse and apply proposals
	proposals, err := retro.ParseProposals(result.Analysis)
	if err != nil {
		r.log("Warning: parsing retro proposals failed: %v", err)
	} else {
		lf, lfErr := learnings.NewFile(r.gromitDir)
		if lfErr == nil {
			if loadErr := lf.Load(); loadErr == nil {
				rulesPath := filepath.Join(r.gromitDir, "RULES.md")
				if applyErr := retro.ApplyProposals(proposals, lf, rulesPath); applyErr != nil {
					r.log("Warning: applying retro proposals failed: %v", applyErr)
				}
			}
		}
	}

	// Record retro in state
	sf := r.stateFile
	if sf == nil {
		sf, err = state.NewFile(r.gromitDir)
		if err != nil {
			r.log("Warning: creating state file for retro recording: %v", err)
			return nil
		}
		if err := sf.Load(); err != nil {
			r.log("Warning: loading state for retro recording: %v", err)
		}
	}
	if err := sf.RecordRetro(); err != nil {
		r.log("Warning: recording retro in state failed: %v", err)
	}

	return nil
}

// epilogueRetroAdapter wraps a provider.Provider to satisfy retro.ProviderRunner.
type epilogueRetroAdapter struct {
	p provider.Provider
}

func (a *epilogueRetroAdapter) Run(ctx context.Context, promptText string, tier string) (*provider.Result, error) {
	return a.p.Run(ctx, promptText, tier)
}

func (a *epilogueRetroAdapter) StreamRun(ctx context.Context, promptText string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return a.p.StreamRun(ctx, promptText, tier, output, handler, onToolCall)
}
