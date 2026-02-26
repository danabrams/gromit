package specmerge

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/specgate"
)

// FixBeadDependencies holds the dependencies needed to create fix beads.
type FixBeadDependencies struct {
	BeadCreator specgate.BeadCreator
}

// HandleStageFailureOptions holds the parameters for handling a stage failure.
type HandleStageFailureOptions struct {
	SpecName     string
	Failures     []specgate.CriterionResult
	Priority     string
	AttemptCount int
	RetryCap     int
}

// HandleStageFailure processes a stage failure by creating fix beads for failed criteria.
func HandleStageFailure(ctx context.Context, deps FixBeadDependencies, opts HandleStageFailureOptions) error {
	if deps.BeadCreator == nil {
		return fmt.Errorf("bead creator is required")
	}

	_, err := specgate.SynthesizeFixBeads(ctx, opts.SpecName, opts.Failures, opts.Priority, deps.BeadCreator)
	if err != nil {
		return fmt.Errorf("synthesize fix beads: %w", err)
	}

	return nil
}

// CheckRetryCapExceeded returns true if the attempt count has reached or exceeded the retry cap.
func CheckRetryCapExceeded(attemptCount, retryCap int) (bool, error) {
	return attemptCount >= retryCap, nil
}

// EmitRetryCapReachedAlert returns a terminal alert message when the retry cap is reached.
func EmitRetryCapReachedAlert(specName string, retryCap int) string {
	return fmt.Sprintf("merge pipeline for spec %q has reached retry cap of %d fix attempts", specName, retryCap)
}
