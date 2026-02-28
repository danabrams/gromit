package tui

import (
	"strings"
	"sync"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner"
)

// Store holds the UI state for the TUI clients.
type Store struct {
	mu        sync.RWMutex
	Dashboard DashboardState
	Queue     QueueState
}

// DashboardState captures the fields needed to render the dashboard view.
type DashboardState struct {
	RunnerStatus      *runner.Status
	PipelineStatus    *pipeline.PipelineStatus
	RunProgress       *RunProgress
	ActivePhase       *ActivePhase
	RecentCompletions []*Completion
	HealthIndicator   *HealthIndicator
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

// HealthIndicator tracks the health of the current run.
type HealthIndicator struct {
	LastEventType    string
	LastEventTime    time.Time
	IsHealthy        bool
	HasStalledBeads  bool
	WarningThreshold time.Duration
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Dashboard.RunProgress = &RunProgress{
		CurrentIteration: 0,
		MaxIterations:    event.MaxIterations,
		Status:           "running",
	}
}

// OnRunComplete updates the store when a run completes.
func (s *Store) OnRunComplete(event *events.RunCompleteEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Dashboard.RunProgress == nil {
		s.Dashboard.RunProgress = &RunProgress{}
	}
	s.Dashboard.RunProgress.Status = runCompletionStatus(event.Reason)
}

func runCompletionStatus(reason string) string {
	normalized := strings.ToLower(strings.TrimSpace(reason))
	switch normalized {
	case "", "completed", "success":
		return "completed"
	default:
		return "failed"
	}
}

// OnIterationStart updates the store when an iteration starts.
func (s *Store) OnIterationStart(event *events.IterationStartEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Dashboard.ActivePhase = &ActivePhase{
		BeadID:    event.BeadID,
		BeadTitle: event.BeadTitle,
		StartTime: event.EventTime(),
	}
}

// OnIterationComplete updates the store when an iteration completes.
func (s *Store) OnIterationComplete(event *events.IterationCompleteEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addRecentCompletion(&Completion{
		BeadID:    event.BeadID,
		BeadTitle: event.BeadTitle,
		Status:    "completed",
		Time:      event.EventTime(),
	})
}

// OnBeadFailed updates the store when a bead fails.
func (s *Store) OnBeadFailed(event *events.BeadFailedEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addRecentCompletion(&Completion{
		BeadID:    event.BeadID,
		BeadTitle: event.BeadTitle,
		Status:    "failed",
		Time:      event.EventTime(),
	})
}

// OnBeadStuck updates the store when a bead is marked as stuck.
func (s *Store) OnBeadStuck(event *events.BeadStuckEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addRecentCompletion(&Completion{
		BeadID:    event.BeadID,
		BeadTitle: event.BeadTitle,
		Status:    "stuck",
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

// OnBuildStart updates the phase when build starts.
func (s *Store) OnBuildStart(event *events.BuildStartEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Dashboard.ActivePhase == nil {
		s.Dashboard.ActivePhase = &ActivePhase{}
	}
	s.Dashboard.ActivePhase.Phase = "build"
	s.Dashboard.ActivePhase.StartTime = event.EventTime()
}

// OnBuildComplete updates the phase when build completes.
func (s *Store) OnBuildComplete(event *events.BuildCompleteEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Phase transitions are handled by the subsequent start events,
	// so we just keep the current phase context
	if s.Dashboard.ActivePhase != nil {
		s.Dashboard.ActivePhase.StartTime = event.EventTime()
	}
}

// OnValidationStart updates the phase when validation starts.
func (s *Store) OnValidationStart(event *events.ValidationStartEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Dashboard.ActivePhase == nil {
		s.Dashboard.ActivePhase = &ActivePhase{}
	}
	s.Dashboard.ActivePhase.Phase = "validation"
	s.Dashboard.ActivePhase.StartTime = event.EventTime()
}

// OnReviewStart updates the phase when review starts.
func (s *Store) OnReviewStart(event *events.ReviewStartEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Dashboard.ActivePhase == nil {
		s.Dashboard.ActivePhase = &ActivePhase{}
	}
	s.Dashboard.ActivePhase.Phase = "review"
	s.Dashboard.ActivePhase.StartTime = event.EventTime()
}

// OnAnalysisStart updates the phase when analysis starts.
func (s *Store) OnAnalysisStart(event *events.AnalysisStartEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Dashboard.ActivePhase == nil {
		s.Dashboard.ActivePhase = &ActivePhase{}
	}
	s.Dashboard.ActivePhase.Phase = "analysis"
	s.Dashboard.ActivePhase.StartTime = event.EventTime()
}

// OnRetroStart updates the phase when retrospective starts.
func (s *Store) OnRetroStart(event *events.RetroStartEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Dashboard.ActivePhase == nil {
		s.Dashboard.ActivePhase = &ActivePhase{}
	}
	s.Dashboard.ActivePhase.Phase = "retro"
	s.Dashboard.ActivePhase.StartTime = event.EventTime()
}

// OnHeartbeat updates health indicators when a heartbeat is received.
func (s *Store) OnHeartbeat(event *events.HeartbeatEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Dashboard.HealthIndicator == nil {
		s.Dashboard.HealthIndicator = &HealthIndicator{}
	}
	s.Dashboard.HealthIndicator.LastEventType = "heartbeat"
	s.Dashboard.HealthIndicator.LastEventTime = event.EventTime()
	s.Dashboard.HealthIndicator.IsHealthy = true
}

// OnStallDetected updates health indicators when a stall is detected.
func (s *Store) OnStallDetected(event *events.StallDetectedEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Dashboard.HealthIndicator == nil {
		s.Dashboard.HealthIndicator = &HealthIndicator{}
	}
	s.Dashboard.HealthIndicator.LastEventType = "stall_detected"
	s.Dashboard.HealthIndicator.LastEventTime = event.EventTime()
	s.Dashboard.HealthIndicator.IsHealthy = false
}
