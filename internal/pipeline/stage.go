// Package pipeline defines the stage interfaces for the Gromit runner pipeline.
// Each stage receives an Input, performs its work, and returns an Output.
// The Runner orchestrator in internal/runner/ wires stages together.
package pipeline

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
)

// Decision indicates how the pipeline should proceed after a stage runs.
type Decision int

const (
	// Proceed means the pipeline continues to the next stage.
	Proceed Decision = iota
	// Skip means the current bead is skipped; the orchestrator moves on.
	Skip
	// Block means the bead is blocked; the orchestrator moves on without executing.
	Block
)

// Input is the data passed to every stage at the start of its execution.
type Input struct {
	// Bead is the work item being processed this iteration.
	Bead *bead.Bead
	// Config is the loaded gromit configuration.
	Config *config.Config
	// Iteration is the monotonically increasing loop iteration number.
	Iteration int
	// Deadline is the wall-clock time by which the stage must complete.
	Deadline time.Time
	// ValidationFailures holds summaries of recent validation failures
	// from the previous validate stage, fed into the next execute stage.
	ValidationFailures []string
	// EscalationEnabled controls whether the Build stage may escalate to higher
	// tiers (haiku→sonnet→opus) on invocation failure. When false, a single
	// attempt is made at the bead's configured tier with no retry.
	EscalationEnabled bool
	// BuildSucceeded is true when the build and validate stages both passed
	// for this iteration. Set by the orchestrator before calling Epilogue.
	BuildSucceeded bool
	// TouchedPackages holds the Go package paths touched by this iteration.
	// Set by the orchestrator before calling Epilogue.
	// The Epilogue returns an updated set in Output.TouchedPackages for the orchestrator to
	// accumulate across iterations and use for success-learning gating.
	TouchedPackages []string
	// Result is the pre-built iteration log entry populated by the orchestrator.
	// The Epilogue stage writes this entry to the iteration log JSONL file.
	// When Result.UsageLimited is true, the JSONL entry includes usage_limited:true.
	// Nil when no log entry should be written (e.g. bead was skipped).
	Result *logger.IterationLog
}

// Output is the result returned by every stage after its execution.
type Output struct {
	// Decision tells the orchestrator how to proceed.
	// A zero-value Output has Decision==Proceed.
	Decision Decision
	// ValidationFailures holds failure summaries produced by the validate stage
	// to be fed into the next execute stage Input.
	ValidationFailures []string
	// ReviewBeadIDs holds IDs of beads created by the review stage from findings.
	ReviewBeadIDs []string
	// TouchedPackages is the updated set of Go package paths touched across iterations,
	// returned by the Epilogue for the orchestrator to accumulate.
	TouchedPackages []string
}

// Stage is the interface that each pipeline stage implements.
// Stages must be stateless across iterations; all per-iteration state flows
// through Input and Output.
type Stage interface {
	Run(ctx context.Context, in Input) (Output, error)
}
