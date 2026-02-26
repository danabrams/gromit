package specmerge

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/tracker"
)

type trackerBeadQuery struct {
	client tracker.Client
}

// NewTrackerBeadQuery wraps a tracker.Client to satisfy the beadQuery contract.
func NewTrackerBeadQuery(client tracker.Client) beadQuery {
	if client == nil {
		return nil
	}
	return &trackerBeadQuery{client: client}
}

func (q *trackerBeadQuery) ListWithLabel(label string) ([]*bead.Bead, error) {
	if q == nil || q.client == nil {
		return nil, fmt.Errorf("bead query is not configured")
	}
	items, err := q.client.ListWithLabel(context.Background(), label)
	if err != nil {
		return nil, err
	}
	beads := make([]*bead.Bead, 0, len(items))
	for i := range items {
		b, err := bead.TrackerItemToBead(&items[i])
		if err != nil {
			return nil, err
		}
		if b != nil {
			beads = append(beads, b)
		}
	}
	return beads, nil
}
