// Package pipeline defines the stage interfaces for the Gromit runner pipeline.
// Each stage receives an Input, performs its work, and returns an Output.
// The Runner orchestrator in internal/runner/ wires stages together.
package pipeline

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
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
	// BuildSucceeded is true when the build and validate stages both passed
	// for this iteration. Set by the orchestrator before calling Epilogue.
	BuildSucceeded bool
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
}

// Stage is the interface that each pipeline stage implements.
// Stages must be stateless across iterations; all per-iteration state flows
// through Input and Output.
type Stage interface {
	Run(ctx context.Context, in Input) (Output, error)
}
