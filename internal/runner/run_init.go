package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/state"
)

type runLoopState struct {
	iteration           int
	statusWriter        *StatusWriter
	statusWritten       bool
	timeBudgetMinutes   int
	consecutiveSkips    int
	beadStats           map[string]logger.BeadStats
	skippedBeads        map[string]bool
	testsAuthoredBySpec map[string]bool
	specGateCycles      map[string]int
	sf                  *state.File
	interactiveFile     *state.InteractiveFile
	l3StopLine          bool // set when L3 stop-line halts state mutations
}

func (r *Runner) validateRunPrerequisites() error {
	if r == nil {
		return fmt.Errorf("runner is nil")
	}
	if r.cfg == nil {
		return fmt.Errorf("runner config is nil")
	}
	if r.beads == nil {
		return fmt.Errorf("runner beads client is nil")
	}
	if r.renderer == nil {
		return fmt.Errorf("runner renderer is nil")
	}
	if r.router == nil {
		return fmt.Errorf("runner router is nil")
	}
	return nil
}

func (r *Runner) resetPerRunState() {
	r.validationFailures = []string{}
	r.touchedPackages = make(map[string]bool)
	r.successfulBeads = 0
	r.successesSinceFull = 0
	if r.validationRunner != nil {
		r.validationRunner.ResetFailures()
	}
}

func (r *Runner) initRunLoopState(deadline time.Time) (*runLoopState, func(), error) {
	st := &runLoopState{
		skippedBeads:        make(map[string]bool),
		testsAuthoredBySpec: make(map[string]bool),
		specGateCycles:      make(map[string]int),
	}

	statusWriter, err := NewStatusWriter(r.gromitDir)
	if err != nil {
		r.log("Warning: could not create status writer: %v", err)
	}
	st.statusWriter = statusWriter

	if !deadline.IsZero() {
		st.timeBudgetMinutes = int(time.Until(deadline).Minutes())
	}

	if r.logger != nil {
		r.log("Logging to: %s", r.logger.FilePath())
	}

	beadStats, err := logger.ReadPerBeadStats(r.cfg.Paths.Logs)
	if err != nil {
		r.log("Warning: could not read bead stats: %v", err)
		beadStats = make(map[string]logger.BeadStats)
	}
	st.beadStats = beadStats

	if r.cfg.Loop.MaxCrossRunFailures > 0 {
		metricsDir := filepath.Join(r.gromitDir, "metrics")
		consecutiveFailures, err := logger.ReadConsecutiveFailureCounts(metricsDir)
		if err != nil {
			r.log("Warning: could not read cross-run failure counts: %v", err)
		} else {
			for beadID, count := range consecutiveFailures {
				if count >= r.cfg.Loop.MaxCrossRunFailures {
					st.skippedBeads[beadID] = true
					r.log("Warning: bead %s has %d consecutive failures across runs (threshold %d). Skipping; please decompose.",
						beadID, count, r.cfg.Loop.MaxCrossRunFailures)
				}
			}
		}
	}

	st.sf = r.stateFile
	if st.sf == nil {
		st.sf, err = state.NewFile(r.gromitDir)
		if err != nil {
			r.log("Warning: could not create state file: %v", err)
			st.sf = nil
		} else if err := st.sf.Load(); err != nil {
			r.log("Warning: could not load state: %v", err)
		}
	}

	st.interactiveFile, err = state.NewInteractiveFile(r.gromitDir)
	if err != nil {
		r.log("Warning: could not create interactive state file: %v", err)
		st.interactiveFile = nil
	} else if err := st.interactiveFile.Load(); err != nil {
		r.log("Warning: could not load interactive state: %v", err)
	}

	if st.sf != nil {
		if isStale, reason := st.sf.CheckStaleness(r.cfg.State.StaleThreshold); isStale {
			r.log("Warning: %s — auto-healing state (resetting iteration counters)", reason)
			st.sf.AutoHeal()
		}

		st.sf.SetCleanExit(false)
		if err := st.sf.Save(); err != nil {
			r.log("Warning: could not save state after setting clean_exit: %v", err)
		}
	}

	if st.interactiveFile != nil && st.interactiveFile.LastReviewCommit() == "" {
		currentCommit, err := r.getHead()
		if err == nil && currentCommit != "" {
			if err := st.interactiveFile.RecordReview(currentCommit, 0); err != nil {
				r.log("Warning: could not initialize review baseline: %v", err)
			} else {
				shortCommit := currentCommit
				if len(shortCommit) > 8 {
					shortCommit = shortCommit[:8]
				}
				r.log("Initialized review baseline at commit %s", shortCommit)
			}
		}
	}

	cleanup := func() {
		if st.statusWriter != nil && st.statusWritten {
			_ = st.statusWriter.WriteFinal(st.iteration)
		}
	}
	return st, cleanup, nil
}

func (r *Runner) shouldStopLoop(ctx context.Context, stopCh <-chan struct{}, st *runLoopState, maxIterations int, deadline time.Time) (bool, error) {
	select {
	case <-ctx.Done():
		r.log("Context cancelled, stopping")
		return true, ctx.Err()
	case <-stopCh:
		r.log("Graceful stop requested, exiting after current bead")
		return true, nil
	default:
	}

	if maxIterations > 0 && st.iteration >= maxIterations {
		r.log("Reached max iterations (%d), stopping", maxIterations)
		return true, nil
	}
	if !deadline.IsZero() && time.Now().After(deadline) {
		r.log("Time budget expired, stopping")
		return true, nil
	}
	return false, nil
}

func (r *Runner) finishRun(ctx context.Context, st *runLoopState) error {
	r.log("\nGromit loop complete. Processed %d iterations.", st.iteration)

	if err := r.maybeRunFinalFullValidation(ctx); err != nil {
		return err
	}
	if st.iteration > 0 && r.logger != nil {
		r.updateGlobalStats()
	}
	if st.sf != nil {
		st.sf.SetCleanExit(true)
		if err := st.sf.Save(); err != nil {
			r.log("Warning: could not save state after clean exit: %v", err)
		}
	}

	// Epilogue handles its own logging; we only need the retro flag here.
	epilogueRanRetro, _ := r.runSessionEpilogue(ctx, st)
	if !epilogueRanRetro {
		r.checkRetroSuggestion()
	}

	if !st.l3StopLine {
		if err := r.runSessionCompletion(); err != nil {
			return err
		}
	}

	if err := r.runEndOfLoopCommand(); err != nil {
		return err
	}

	return nil
}
