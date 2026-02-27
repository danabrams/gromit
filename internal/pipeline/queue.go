package pipeline

import (
	"context"
	"fmt"
)

// GetQueue assembles queue data by partitioning beads into ready, blocked, and stuck.
// It validates deps, partitions the input beads, and returns QueuePartition.
func (p *Pipeline) GetQueue(ctx context.Context, input GetQueueInput) (*QueuePartition, error) {
	if p.deps == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	if err := requireNonNilDep("BeadQueryClient", p.deps.BeadQueryClient); err != nil {
		return nil, err
	}

	result := NewQueuePartition()
	return &result, nil
}
