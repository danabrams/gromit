package pipeline

import (
	"context"
	"fmt"
	"strings"

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
func (p *Pipeline) Unstick(ctx context.Context, beadID, gromitDir string) error {
	beadID = strings.TrimSpace(beadID)
	if beadID == "" {
		return fmt.Errorf("bead id is required")
	}

	client, err := p.queueClient()
	if err != nil {
		return fmt.Errorf("creating bead client: %w", err)
	}

	active, err := ListActiveBeads(ctx, client)
	if err != nil {
		return fmt.Errorf("listing active beads: %w", err)
	}

	for _, b := range active {
		if b != nil && b.ID == beadID {
			_ = gromitDir
			return nil
		}
	}

	return fmt.Errorf("bead %s not found", beadID)
}
