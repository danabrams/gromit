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
	RunnerStatus       *runner.Status
	PipelineStatus    *pipeline.PipelineStatus
	RunProgress       *RunProgress
	ActivePhase       *ActivePhase
	RecentCompletions []*Completion
	LastHydration     time.Time
	Warnings          []string
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

// Completion tracks a recently completed or failed bead.
type Completion struct {
	BeadID    string
	BeadTitle string
	Status    string // completed, failed, stuck
	Time      time.Time
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

// OnIterationComplete updates the store when an iteration completes.
func (s *Store) OnIterationComplete(event *events.IterationCompleteEvent) {
	if s.Dashboard.RunProgress == nil {
		s.Dashboard.RunProgress = &RunProgress{}
	}
	s.Dashboard.RunProgress.CurrentIteration++
	if s.Dashboard.RunProgress.MaxIterations > 0 {
		s.Dashboard.RunProgress.IterationPercent = (s.Dashboard.RunProgress.CurrentIteration * 100) / s.Dashboard.RunProgress.MaxIterations
	}
}

// OnBeadComplete updates the store when a bead completes successfully.
func (s *Store) OnBeadComplete(event *events.BeadCompleteEvent) {
	s.addRecentCompletion(&Completion{
		BeadID:    event.BeadID,
		BeadTitle: event.BeadTitle,
		Status:    "completed",
		Time:      event.EventTime(),
	})
}

// OnBeadFailed updates the store when a bead fails.
func (s *Store) OnBeadFailed(event *events.BeadFailedEvent) {
	s.addRecentCompletion(&Completion{
		BeadID:    event.BeadID,
		BeadTitle: event.BeadTitle,
		Status:    "failed",
		Time:      event.EventTime(),
	})
}

// addRecentCompletion adds a completion to the recent list, keeping only the 10 most recent.
func (s *Store) addRecentCompletion(completion *Completion) {
	s.Dashboard.RecentCompletions = append(s.Dashboard.RecentCompletions, completion)
	if len(s.Dashboard.RecentCompletions) > 10 {
		s.Dashboard.RecentCompletions = s.Dashboard.RecentCompletions[1:]
	}
}
