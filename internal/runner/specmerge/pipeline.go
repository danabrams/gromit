package specmerge

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/specgate"
)

const specLabelPrefix = "spec:"

// beadQuery defines the subset of the bead client needed by Pipeline.
type beadQuery interface {
	ListWithLabel(label string) ([]*bead.Bead, error)
}

// Controller coordinates spec merge completion triggers.
type Controller interface {
	IsSpecComplete(specName string) (bool, error)
	Trigger(ctx context.Context, specName string) error
}

// Pipeline runs spec merge orchestration helpers.
type Pipeline struct {
	query beadQuery
}

// NewPipeline constructs a Pipeline with the provided bead query dependencies.
func NewPipeline(query beadQuery) *Pipeline {
	return &Pipeline{query: query}
}

// IsSpecComplete returns true when no open beads remain for the given spec.
func (p *Pipeline) IsSpecComplete(specName string) (bool, error) {
	if p == nil {
		return false, fmt.Errorf("pipeline is nil")
	}
	if p.query == nil {
		return false, fmt.Errorf("bead query is required")
	}
	specName = strings.TrimSpace(specName)
	if specName == "" {
		return false, fmt.Errorf("spec name is required")
	}

	label := specLabelPrefix + specName
	beads, err := p.query.ListWithLabel(label)
	if err != nil {
		return false, fmt.Errorf("list beads for spec %q: %w", specName, err)
	}
	for _, b := range beads {
		if b == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(b.Status), "closed") {
			continue
		}
		return false, nil
	}
	return true, nil
}

// Trigger is a placeholder that will eventually start the spec merge pipeline.
func (p *Pipeline) Trigger(ctx context.Context, specName string) error {
	if p == nil {
		return fmt.Errorf("pipeline is nil")
	}
	_ = ctx
	_ = specName
	return nil
}

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
