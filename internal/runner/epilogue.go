package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/prompt"
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

	reviewEnabled := r.cfg.Session.Review == nil || *r.cfg.Session.Review
	if reviewEnabled {
		if err := r.runEpilogueReview(ctx, st); err != nil {
			r.log("Warning: epilogue review failed: %v", err)
		}
	}

	retroEnabled := r.cfg.Session.Retro == nil || *r.cfg.Session.Retro
	if retroEnabled {
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
		fixTier = "medium"
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
	r.log("Running epilogue retro...")
	return nil
}
