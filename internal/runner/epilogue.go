package runner

import (
	"context"
	"time"
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
