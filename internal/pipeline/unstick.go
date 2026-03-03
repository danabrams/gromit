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
