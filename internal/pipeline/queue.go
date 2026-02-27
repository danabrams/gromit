package pipeline

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/queue"
)

type queueClient interface {
	ActiveBeadClient
	ListReadyWork(ctx context.Context) ([]*bead.Bead, error)
}

var newQueueClient = func() (queueClient, error) {
	client, err := bead.NewClient()
	if err != nil {
		return nil, err
	}
	return client, nil
}

// QueueInput contains the configuration required to render the queue.
type QueueInput struct {
	LogsDir        string
	StuckThreshold int
}

// QueueResult contains beads organized by processing state for CLI display.
type QueueResult struct {
	Ready   []*bead.Bead
	Blocked []*bead.Bead
	Stuck   []*bead.Bead
	All     []*bead.Bead
}

// Queue assembles queue data for the CLI.
func (p *Pipeline) Queue(ctx context.Context, input QueueInput) (*QueueResult, error) {
	client, err := newQueueClient()
	if err != nil {
		return nil, fmt.Errorf("creating bead client: %w", err)
	}

	readyBeads, err := client.ListReadyWork(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting ready beads: %w", err)
	}

	allBeads, err := ListActiveBeads(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("listing active beads: %w", err)
	}

	readyBeads = queue.EnrichReadyBeads(readyBeads, allBeads)

	beadStats, err := logger.ReadPerBeadStats(input.LogsDir)
	if err != nil {
		beadStats = map[string]logger.BeadStats{}
	}

	stuckThreshold := input.StuckThreshold
	if stuckThreshold <= 0 {
		stuckThreshold = 3
	}

	ready, blocked, stuck := queue.PartitionQueueBeads(readyBeads, allBeads, beadStats, stuckThreshold)

	return &QueueResult{
		Ready:   ready,
		Blocked: blocked,
		Stuck:   stuck,
		All:     allBeads,
	}, nil
}
