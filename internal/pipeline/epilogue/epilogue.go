package epilogue

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

// BeadLifecycle handles bead close and sync operations after a successful iteration.
type BeadLifecycle interface {
	Close(ctx context.Context, id string) error
	Sync(ctx context.Context) error
}

// StatusWriter writes execution status after each iteration.
type StatusWriter interface {
	Write(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error
}

// WorktreeMerger merges pending interactive worktree branches back into the main working tree
// and cleans up orphaned session worktrees.
type WorktreeMerger interface {
	PendingBranches() ([]string, error)
	MergeBack(branch string) error
	DeriveSessionWorktreePath(branch string) string
	RemoveByPath(path string) error
}

// PendingBranchRemover removes successfully-merged branches from persistent state.
type PendingBranchRemover interface {
	RemovePendingWorktreeBranch(branch string) error
}

// CommandRunner executes a shell command and returns stdout, stderr, exit code, and any error.
type CommandRunner interface {
	Run(ctx context.Context, command string) (string, string, int, error)
}

// SpecGateRunner evaluates the spec gate acceptance criteria for a bead.
type SpecGateRunner interface {
	Run(ctx context.Context, beadID string, labels []string) error
}

// ThoroughReviewer runs a comprehensive code review of accumulated changes.
type ThoroughReviewer interface {
	Run(ctx context.Context, iteration int) error
}

// EpicChecker checks whether a parent bead has remaining open child beads.
type EpicChecker interface {
	HasOpenChildren(ctx context.Context, parentID string) (bool, error)
}

// FailureLearner extracts learnings from failed iterations.
// It is called unconditionally on every failure, regardless of tier or package novelty.
// failureOutput is the raw validation failure text for the current iteration.
type FailureLearner interface {
	ExtractFailureLearning(ctx context.Context, beadID, beadTitle, failureOutput string) error
}

// IterationLogWriter writes a pre-built iteration log entry to persistent storage.
// When the entry has UsageLimited=true, the JSONL record includes usage_limited:true.
type IterationLogWriter interface {
	Write(log *logger.IterationLog) error
}

// Epilogue implements pipeline.Stage for Stage 5: bead lifecycle and cleanup.
// It closes and syncs the bead on the success path, evaluates the spec gate when
// spec-level methodology is active, merges interactive worktree branches, writes
// status after every iteration, triggers thorough reviews, and runs the
// between-iterations command.
type Epilogue struct {
	events.EmitterMixin // provides Emitter field and SetEmitter method
	beads               BeadLifecycle
	status              StatusWriter
	output              io.Writer
	worktree            WorktreeMerger       // optional; nil means skip worktree merge
	branchRemover       PendingBranchRemover // optional; nil means skip branch removal from state
	cmd                 CommandRunner        // optional; nil means skip between-iterations command
	specgate            SpecGateRunner       // optional; nil means skip spec gate
	review              ThoroughReviewer     // optional; nil means skip thorough review
	epic                EpicChecker          // optional; used with review for epic completion detection
	failureLearner      FailureLearner       // optional; nil means skip failure-path learning
	logWriter           IterationLogWriter   // optional; nil means skip iteration log write
	mergeWarnings       map[string]string    // per-run de-duplication of merge warnings by branch
}

// Compile-time check: *Epilogue must implement pipeline.Stage.
var _ pipeline.Stage = (*Epilogue)(nil)

// New creates an Epilogue stage with the given dependencies.
// output receives warning messages; pass io.Discard to suppress.
func New(beads BeadLifecycle, status StatusWriter, output io.Writer) *Epilogue {
	return &Epilogue{
		beads:  beads,
		status: status,
		output: output,
	}
}

// WithEmitter attaches an EventEmitter for log events.
func (e *Epilogue) WithEmitter(emitter *events.Emitter) *Epilogue {
	e.EmitterMixin.SetEmitter(emitter)
	return e
}

// WithWorktree configures an optional WorktreeMerger for merging interactive branches.
func (e *Epilogue) WithWorktree(m WorktreeMerger) *Epilogue {
	e.worktree = m
	return e
}

// WithPendingBranchRemover configures an optional PendingBranchRemover for removing merged branches from state.
func (e *Epilogue) WithPendingBranchRemover(r PendingBranchRemover) *Epilogue {
	e.branchRemover = r
	return e
}

// WithCommandRunner configures an optional CommandRunner for the between-iterations command.
func (e *Epilogue) WithCommandRunner(r CommandRunner) *Epilogue {
	e.cmd = r
	return e
}

// WithSpecGate configures an optional SpecGateRunner for spec-level acceptance criteria.
func (e *Epilogue) WithSpecGate(g SpecGateRunner) *Epilogue {
	e.specgate = g
	return e
}

// WithThoroughReview configures an optional ThoroughReviewer and EpicChecker.
// The EpicChecker is used to detect epic completion; pass nil to skip epic-based triggering.
func (e *Epilogue) WithThoroughReview(r ThoroughReviewer, ec EpicChecker) *Epilogue {
	e.review = r
	e.epic = ec
	return e
}

// WithFailureLearner configures an optional FailureLearner for failure-path learning extraction.
// When set, it is called unconditionally on every failed iteration.
func (e *Epilogue) WithFailureLearner(fl FailureLearner) *Epilogue {
	e.failureLearner = fl
	return e
}

// WithIterationLogWriter configures an optional IterationLogWriter for persisting the
// iteration log entry. When Input.Result is non-nil and a writer is configured, the
// entry is written; UsageLimited=true is preserved in the JSONL output.
func (e *Epilogue) WithIterationLogWriter(w IterationLogWriter) *Epilogue {
	e.logWriter = w
	return e
}

// Run executes the epilogue stage.
// On the success path (in.BuildSucceeded == true), it closes the bead and syncs.
// After every iteration, it writes status.
func (e *Epilogue) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	w := e.output
	if w == nil {
		w = io.Discard
	}
	lifecycleFailure := pipeline.LifecycleFailureNone
	warningOccurred := false
	warnf := func(format string, args ...interface{}) {
		e.Log("warning", format, args...)
		warningOccurred = true
	}

	// Emit EpilogueStartEvent
	if in.Emitter != nil {
		in.Emitter.Emit(&events.EpilogueStartEvent{
			BeadID:    in.Bead.ID,
			Iteration: in.Iteration,
			Success:   in.BuildSucceeded,
			Time:      time.Now(),
		})
	}

	// 1. Bead lifecycle: close and sync on success.
	if in.BuildSucceeded {
		if err := e.beads.Close(ctx, in.Bead.ID); err != nil {
			warnf("Warning: failed to close bead: %v\n", err)
			if lifecycleFailure == pipeline.LifecycleFailureNone {
				lifecycleFailure = pipeline.LifecycleFailureClose
			}
		} else {
			// Emit BeadCloseEvent on successful close
			if in.Emitter != nil {
				in.Emitter.Emit(&events.BeadCloseEvent{
					BeadID: in.Bead.ID,
					Time:   time.Now(),
				})
			}
		}
		if err := e.beads.Sync(ctx); err != nil {
			warnf("Warning: failed to sync beads: %v\n", err)
			if lifecycleFailure == pipeline.LifecycleFailureNone {
				lifecycleFailure = pipeline.LifecycleFailureSync
			}
		} else {
			// Emit BeadCleanupEvent for sync action
			if in.Emitter != nil {
				in.Emitter.Emit(&events.BeadCleanupEvent{
					BeadID: in.Bead.ID,
					Action: "sync",
					Time:   time.Now(),
				})
			}
		}
	}

	// 1a. Failure-path learning: extract unconditionally on every failure,
	// regardless of tier or package novelty.
	if !in.BuildSucceeded && e.failureLearner != nil {
		if err := e.failureLearner.ExtractFailureLearning(ctx, in.Bead.ID, in.Bead.Title, in.FailureOutput); err != nil {
			warnf("Warning: failed to extract failure learning: %v\n", err)
		}
	}

	// 2. Spec gate: DEPRECATED - the new merge pipeline is now the only completion
	// mechanism for spec-level methodology. Legacy auto-trigger has been disabled.
	// The spec gate runner is kept wired for backward compatibility but no longer invoked.
	_ = e.specgate

	// 3. Worktree merge: merge pending interactive branches when enabled.
	if e.worktree != nil && in.Config != nil &&
		in.Config.Worktree.IsEnabled() && in.Config.Worktree.IsAutoMergeEnabled() {
		branches, err := e.worktree.PendingBranches()
		if err != nil {
			warnf("Warning: failed to list pending branches: %v\n", err)
		} else {
			seen := make(map[string]struct{}, len(branches))
			for _, branch := range branches {
				if _, ok := seen[branch]; ok {
					continue
				}
				seen[branch] = struct{}{}
				if err := e.worktree.MergeBack(branch); err != nil {
					errMsg := err.Error()
					if e.shouldEmitMergeWarning(branch, errMsg) {
						warnf("Warning: failed to merge branch %s: %v\n", branch, err)
					}
				} else {
					e.clearMergeWarning(branch)
					// Emit BeadCleanupEvent for successful merge
					if in.Emitter != nil {
						in.Emitter.Emit(&events.BeadCleanupEvent{
							BeadID: in.Bead.ID,
							Action: "merge",
							Time:   time.Now(),
						})
					}
					// Remove orphaned session worktree after successful merge
					worktreePath := e.worktree.DeriveSessionWorktreePath(branch)
					if worktreePath != "" {
						if err := e.worktree.RemoveByPath(worktreePath); err != nil {
							warnf("Warning: failed to remove worktree at %s: %v\n", worktreePath, err)
						} else {
							// Emit BeadCleanupEvent for successful worktree removal
							if in.Emitter != nil {
								in.Emitter.Emit(&events.BeadCleanupEvent{
									BeadID: in.Bead.ID,
									Action: "worktree_cleanup",
									Time:   time.Now(),
								})
							}
						}
					}
					// Remove successfully-merged branch from pending state
					if e.branchRemover != nil {
						if err := e.branchRemover.RemovePendingWorktreeBranch(branch); err != nil {
							warnf("Warning: failed to remove pending branch %s from state: %v\n", branch, err)
						}
					}
				}
			}
		}
	}

	// 4. Status: always write after each iteration.
	if e.status != nil {
		maxIter := 0
		if in.Config != nil {
			maxIter = in.Config.Loop.MaxIterations
		}
		tbm := computeTimeBudgetMinutes(in.Deadline)
		model := ""
		if in.Result != nil {
			model = in.Result.Model
		}
		if err := e.status.Write(in.Iteration, in.Bead.ID, in.Bead.Title, model, maxIter, tbm); err != nil {
			warnf("Warning: failed to write status: %v\n", err)
		}
	}

	// 5. Iteration log: write when a result and writer are both present.
	if e.logWriter != nil && in.Result != nil {
		if err := e.logWriter.Write(in.Result); err != nil {
			warnf("Warning: failed to write iteration log: %v\n", err)
		}
	}

	// 6. Thorough review: trigger by frequency or epic completion.
	if e.review != nil && in.Config != nil && in.Config.Review.Thorough.Enabled {
		shouldRun := false
		n := in.Config.Review.Thorough.EveryNIterations
		if n > 0 && in.Iteration%n == 0 {
			shouldRun = true
		}
		if !shouldRun && in.BuildSucceeded && in.Bead != nil && in.Bead.Parent != "" &&
			in.Config.Review.Thorough.ShouldRunOnEpicComplete() && e.epic != nil {
			hasChildren, err := e.epic.HasOpenChildren(ctx, in.Bead.Parent)
			if err != nil {
				warnf("Warning: failed to check epic completion: %v\n", err)
			} else if !hasChildren {
				shouldRun = true
			}
		}
		if shouldRun {
			if err := e.review.Run(ctx, in.Iteration); err != nil {
				warnf("Warning: thorough review failed: %v\n", err)
			}
		}
	}

	// 7. Between-iterations command: run when configured.
	if e.cmd != nil && in.Config != nil {
		command := in.Config.Loop.BetweenIterationsCommand
		if command != "" {
			stdout, stderr, exitCode, err := e.cmd.Run(ctx, command)
			if stdout != "" {
				fmt.Fprint(w, stdout)
			}
			if err != nil {
				warnf("Warning: between-iterations command failed: %v\n", err)
			} else if exitCode != 0 {
				warnf("Warning: between-iterations command exited with code %d: %s\n", exitCode, strings.TrimSpace(stderr))
			}
		}
	}

	// Emit EpilogueCompleteEvent
	if in.Emitter != nil {
		in.Emitter.Emit(&events.EpilogueCompleteEvent{
			BeadID:  in.Bead.ID,
			Success: lifecycleFailure == pipeline.LifecycleFailureNone && !warningOccurred,
			Time:    time.Now(),
		})
	}

	return pipeline.Output{
		Decision:         pipeline.Proceed,
		TouchedPackages:  in.TouchedPackages,
		LifecycleFailure: lifecycleFailure,
		LifecycleWarning: warningOccurred,
	}, nil
}

func computeTimeBudgetMinutes(deadline time.Time) int {
	if deadline.IsZero() {
		return 0
	}
	m := int(time.Until(deadline).Minutes())
	if m < 0 {
		return 0
	}
	return m
}

func (e *Epilogue) shouldEmitMergeWarning(branch, errMsg string) bool {
	if e == nil {
		return true
	}
	if e.mergeWarnings == nil {
		e.mergeWarnings = make(map[string]string)
	}
	last, exists := e.mergeWarnings[branch]
	if exists && last == errMsg {
		return false
	}
	e.mergeWarnings[branch] = errMsg
	return true
}

func (e *Epilogue) clearMergeWarning(branch string) {
	if e == nil || e.mergeWarnings == nil {
		return
	}
	delete(e.mergeWarnings, branch)
}

