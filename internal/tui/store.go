package tui

import (
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
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
	RunProgress    *RunProgress
	ActivePhase    *ActivePhase
	LastHydration  time.Time
	Warnings       []string
}

// RunProgress tracks the current progress through a run.
type RunProgress struct {
	CurrentIteration int
	MaxIterations    int
	IterationPercent int
	Status           string // running, completed, failed
}

// ActivePhase tracks the currently active bead and phase.
type ActivePhase struct {
	BeadID    string
	BeadTitle string
	Phase     string
	StartTime time.Time
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

// OnRunStart updates the store when a run starts.
func (s *Store) OnRunStart(event *events.RunStartEvent) {
	s.Dashboard.RunProgress = &RunProgress{
		CurrentIteration: 0,
		MaxIterations:    event.MaxIterations,
		Status:           "running",
	}
}

// OnRunComplete updates the store when a run completes.
func (s *Store) OnRunComplete(event *events.RunCompleteEvent) {
	if s.Dashboard.RunProgress == nil {
		s.Dashboard.RunProgress = &RunProgress{}
	}
	s.Dashboard.RunProgress.Status = "completed"
}

// OnIterationStart updates the store when an iteration starts.
func (s *Store) OnIterationStart(event *events.IterationStartEvent) {
	s.Dashboard.ActivePhase = &ActivePhase{
		BeadID:    event.BeadID,
		BeadTitle: event.BeadTitle,
		StartTime: event.EventTime(),
	}
}
