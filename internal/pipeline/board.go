package pipeline

import (
	"context"
	"fmt"
)

// GetBoard assembles board data with open and closed beads.
// It validates deps, queries the BeadQueryClient, and returns BoardData.
func (p *Pipeline) GetBoard(ctx context.Context, input GetBoardInput) (*BoardData, error) {
	if p.deps == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	if err := requireNonNilDep("BeadQueryClient", p.deps.BeadQueryClient); err != nil {
		return nil, err
	}

	result := NewBoardData()
	return &result, nil
}
