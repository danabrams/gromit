package tui

import (
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner"
)

// Store holds the UI state for the TUI clients.
type Store struct {
	Dashboard DashboardState
	Queue     QueueState
}

// DashboardState captures the fields needed to render the dashboard view.
type DashboardState struct {
	RunnerStatus   *runner.Status
	PipelineStatus *pipeline.PipelineStatus
	LastHydration  time.Time
	Warnings       []string
}

// QueueState tracks the queue snapshot visible in the queue view.
type QueueState struct {
	Snapshot      *QueueSnapshot
	LastHydration time.Time
	Warnings      []string
}

// QueueSnapshot represents the data shared between queue views and hydration.
type QueueSnapshot struct {
	Ready          []*bead.Bead
	Blocked        []*bead.Bead
	Stuck          []*bead.Bead
	All            []*bead.Bead
	Stats          map[string]logger.BeadStats
	StuckThreshold int
}
