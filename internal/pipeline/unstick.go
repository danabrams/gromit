package pipeline

import "context"

// Unstick marks a bead as unstuck.
func (p *Pipeline) Unstick(ctx context.Context, beadID string) (*UnstickResult, error) {
	// TODO: implement actual unstick logic
	return &UnstickResult{
		BeadID: beadID,
		Status: "unstuck",
	}, nil
}

// ListStuck returns a list of stuck beads.
func (p *Pipeline) ListStuck(ctx context.Context) (*ListStuckResult, error) {
	// TODO: implement actual listing of stuck beads
	return &ListStuckResult{
		StuckBeads: []BeadInfo{},
	}, nil
}
