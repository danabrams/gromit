package pipeline

import (
	"context"

	"github.com/danabrams/gromit/internal/bead"
)

// ListStuck returns beads that are currently marked as stuck according to queue logic.
func (p *Pipeline) ListStuck(ctx context.Context, input QueueInput) ([]*bead.Bead, error) {
	result, err := p.Queue(ctx, input)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return []*bead.Bead{}, nil
	}
	if result.Stuck == nil {
		return []*bead.Bead{}, nil
	}
	return result.Stuck, nil
}

// Unstick marks a bead as unstuck.
func (p *Pipeline) Unstick(ctx context.Context, beadID string) (*UnstickResult, error) {
	// TODO: implement actual unstick logic
	return &UnstickResult{
		BeadID: beadID,
		Status: "unstuck",
	}, nil
}
