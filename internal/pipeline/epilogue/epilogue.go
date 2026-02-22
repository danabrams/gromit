package epilogue

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

// BeadLifecycle handles bead close and sync operations after a successful iteration.
type BeadLifecycle interface {
	Close(id string) error
	Sync() error
}

// StatusWriter writes execution status after each iteration.
type StatusWriter interface {
	Write(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error
}

// WorktreeMerger merges pending interactive worktree branches back into the main working tree.
type WorktreeMerger interface {
	PendingBranches() ([]string, error)
	MergeBack(branch string) error
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
	HasOpenChildren(parentID string) (bool, error)
}

// FailureLearner extracts learnings from failed iterations.
// It is called unconditionally on every failure, regardless of tier or package novelty.
type FailureLearner interface {
	ExtractFailureLearning(ctx context.Context, beadID, beadTitle string) error
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
	beads          BeadLifecycle
	status         StatusWriter
	output         io.Writer
	worktree       WorktreeMerger     // optional; nil means skip worktree merge
	cmd            CommandRunner      // optional; nil means skip between-iterations command
	specgate       SpecGateRunner     // optional; nil means skip spec gate
	review         ThoroughReviewer   // optional; nil means skip thorough review
	epic           EpicChecker        // optional; used with review for epic completion detection
	failureLearner FailureLearner     // optional; nil means skip failure-path learning
	logWriter      IterationLogWriter // optional; nil means skip iteration log write
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

// WithWorktree configures an optional WorktreeMerger for merging interactive branches.
func (e *Epilogue) WithWorktree(m WorktreeMerger) *Epilogue {
	e.worktree = m
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

	// 1. Bead lifecycle: close and sync on success.
	if in.BuildSucceeded {
		if err := e.beads.Close(in.Bead.ID); err != nil {
			fmt.Fprintf(w, "Warning: failed to close bead: %v\n", err)
		}
		if err := e.beads.Sync(); err != nil {
			fmt.Fprintf(w, "Warning: failed to sync beads: %v\n", err)
		}
	}

	// 1a. Failure-path learning: extract unconditionally on every failure,
	// regardless of tier or package novelty.
	if !in.BuildSucceeded && e.failureLearner != nil {
		if err := e.failureLearner.ExtractFailureLearning(ctx, in.Bead.ID, in.Bead.Title); err != nil {
			fmt.Fprintf(w, "Warning: failed to extract failure learning: %v\n", err)
		}
	}

	// 2. Spec gate: evaluate acceptance criteria when spec-level methodology is active.
	if in.BuildSucceeded && in.Config != nil && e.specgate != nil {
		if in.Config.Methodology.Granularity == config.MethodologyGranularitySpec &&
			in.Config.SpecGate.IsEnabled() && in.Config.SpecGate.IsAutoTrigger() {
			if err := e.specgate.Run(ctx, in.Bead.ID, in.Bead.Labels); err != nil {
				fmt.Fprintf(w, "Warning: spec gate failed: %v\n", err)
			}
		}
	}

	// 3. Worktree merge: merge pending interactive branches when enabled.
	if e.worktree != nil && in.Config != nil &&
		in.Config.Worktree.IsEnabled() && in.Config.Worktree.IsAutoMergeEnabled() {
		branches, err := e.worktree.PendingBranches()
		if err != nil {
			fmt.Fprintf(w, "Warning: failed to list pending branches: %v\n", err)
		} else {
			for _, branch := range branches {
				if err := e.worktree.MergeBack(branch); err != nil {
					fmt.Fprintf(w, "Warning: failed to merge branch %s: %v\n", branch, err)
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
		if err := e.status.Write(in.Iteration, in.Bead.ID, in.Bead.Title, "", maxIter, tbm); err != nil {
			fmt.Fprintf(w, "Warning: failed to write status: %v\n", err)
		}
	}

	// 5. Iteration log: write when a result and writer are both present.
	if e.logWriter != nil && in.Result != nil {
		if err := e.logWriter.Write(in.Result); err != nil {
			fmt.Fprintf(w, "Warning: failed to write iteration log: %v\n", err)
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
			hasChildren, err := e.epic.HasOpenChildren(in.Bead.Parent)
			if err != nil {
				fmt.Fprintf(w, "Warning: failed to check epic completion: %v\n", err)
			} else if !hasChildren {
				shouldRun = true
			}
		}
		if shouldRun {
			if err := e.review.Run(ctx, in.Iteration); err != nil {
				fmt.Fprintf(w, "Warning: thorough review failed: %v\n", err)
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
				fmt.Fprintf(w, "Warning: between-iterations command failed: %v\n", err)
			} else if exitCode != 0 {
				fmt.Fprintf(w, "Warning: between-iterations command exited with code %d: %s\n", exitCode, strings.TrimSpace(stderr))
			}
		}
	}

	return pipeline.Output{
		Decision:        pipeline.Proceed,
		TouchedPackages: in.TouchedPackages,
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
