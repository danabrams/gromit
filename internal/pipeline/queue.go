package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/queue"
)

type QueueClient interface {
	ActiveBeadClient
	ListReadyWork(ctx context.Context) ([]*bead.Bead, error)
}

// QueueInput contains the configuration required to render the queue.
type QueueInput struct {
	LogsDir        string
	StuckThreshold int
	GromitDir      string
}

// QueueResult contains beads organized by processing state for CLI display.
type QueueResult struct {
	Ready      []*bead.Bead
	Blocked    []*bead.Bead
	Stuck      []*bead.Bead
	InProgress []*bead.Bead
	All        []*bead.Bead
}

// Queue assembles queue data for the CLI.
func (p *Pipeline) Queue(ctx context.Context, input QueueInput) (*QueueResult, error) {
	client, err := p.queueClient()
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

	gromitDir := input.GromitDir
	if gromitDir == "" && p != nil && p.paths != nil {
		gromitDir = p.paths.GromitDir
	}
	restartPoints := map[string]time.Time{}
	if gromitDir != "" {
		store := newRestartPointStore(gromitDir)
		if err := store.load(); err == nil {
			restartPoints = store.all()
		}
	}
	beadStats, err := logger.ReadPerBeadStatsAfter(input.LogsDir, restartPoints)
	if err != nil {
		beadStats = map[string]logger.BeadStats{}
	}

	stuckThreshold := input.StuckThreshold
	if stuckThreshold <= 0 {
		stuckThreshold = 3
	}

	ready, blocked, stuck, inProgress := queue.PartitionQueueBeads(readyBeads, allBeads, beadStats, stuckThreshold)

	return &QueueResult{
		Ready:      ready,
		Blocked:    blocked,
		Stuck:      stuck,
		InProgress: inProgress,
		All:        allBeads,
	}, nil
}

func (p *Pipeline) queueClient() (QueueClient, error) {
	if p != nil && p.deps != nil && p.deps.QueueClient != nil {
		return p.deps.QueueClient, nil
	}
	return bead.NewClient()
}
