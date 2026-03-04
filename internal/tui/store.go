package tui

import (
	"strings"
	"sync"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/conversation"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner"
)

// Store holds the UI state for the TUI clients.
type Store struct {
	mu            sync.RWMutex
	Dashboard     DashboardState
	Queue         QueueState
	Conversation  ConversationState
	PipelineItems PipelineItems
	// DeletePipelineItemFunc is invoked when the TUI requests a pipeline deletion.
	DeletePipelineItemFunc func(tab Tab, identifier string)
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

// PipelineItems holds the data needed to render pipeline tabs.
type PipelineItems struct {
	BacklogIdeas      []backlog.Idea
	UnplannedSpecs    []string
	UndecomposedPlans []string
	Beads             []bead.Bead
}

func normalizePipelineItems(items PipelineItems) PipelineItems {
	if items.BacklogIdeas == nil {
		items.BacklogIdeas = []backlog.Idea{}
	}
	if items.UnplannedSpecs == nil {
		items.UnplannedSpecs = []string{}
	}
	if items.UndecomposedPlans == nil {
		items.UndecomposedPlans = []string{}
	}
	if items.Beads == nil {
		items.Beads = []bead.Bead{}
	}
	return items
}

func copyPipelineItems(items PipelineItems) PipelineItems {
	items = normalizePipelineItems(items)
	items.BacklogIdeas = append([]backlog.Idea{}, items.BacklogIdeas...)
	items.UnplannedSpecs = append([]string{}, items.UnplannedSpecs...)
	items.UndecomposedPlans = append([]string{}, items.UndecomposedPlans...)
	items.Beads = append([]bead.Bead{}, items.Beads...)
	return items
}

// ConversationState tracks conversation state.
type ConversationState struct {
	EventCount int
	LastEvent  *conversation.Event

	Transcript     []ConversationTranscriptRow
	Lifecycle      ConversationLifecycle
	ToolIndicators []ConversationToolIndicator
	Session        ConversationSessionMetadata

	activeTranscriptIndex int
	hasActiveTranscript   bool
}

// ConversationLifecycle describes the current state of the conversation stream.
type ConversationLifecycle int

const (
	ConversationLifecycleIdle ConversationLifecycle = iota
	ConversationLifecycleStreaming
	ConversationLifecycleToolWait
	ConversationLifecycleToolResult
	ConversationLifecycleDone
)

func (l ConversationLifecycle) String() string {
	switch l {
	case ConversationLifecycleStreaming:
		return "streaming"
	case ConversationLifecycleToolWait:
		return "tool_wait"
	case ConversationLifecycleToolResult:
		return "tool_result"
	case ConversationLifecycleDone:
		return "done"
	default:
		return "idle"
	}
}

// ConversationTranscriptRow represents a single row in the transcript.
type ConversationTranscriptRow struct {
	Type conversation.EventType
	Text string
}

// ConversationToolIndicator captures tool activity separate from the transcript.
type ConversationToolIndicator struct {
	ToolName string
	Status   string
}

// ConversationSessionMetadata summarizes session-level activity.
type ConversationSessionMetadata struct {
	Started         bool
	Completed       bool
	ToolWaitCount   int
	ToolResultCount int
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

// ApplyConversationEvent updates the store when a conversation event occurs.
func (s *Store) ApplyConversationEvent(event conversation.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Conversation.EventCount++
	s.Conversation.LastEvent = &event
	if !s.Conversation.Session.Started {
		s.Conversation.Session.Started = true
	}
	switch event.Type {
	case conversation.EventTypeStream:
		s.Conversation.Lifecycle = ConversationLifecycleStreaming
		s.appendConversationTranscript(event.Text)
	case conversation.EventTypeToolWait:
		s.Conversation.Lifecycle = ConversationLifecycleToolWait
		s.recordConversationToolIndicator(event.ToolName, "waiting")
		s.Conversation.Session.ToolWaitCount++
	case conversation.EventTypeToolResult:
		s.Conversation.Lifecycle = ConversationLifecycleToolResult
		s.recordConversationToolIndicator(event.ToolName, "result")
		s.Conversation.Session.ToolResultCount++
	case conversation.EventTypeDone:
		s.Conversation.Lifecycle = ConversationLifecycleDone
		s.appendConversationTranscript(event.Text)
		s.Conversation.Session.Completed = true
	default:
		s.Conversation.Lifecycle = ConversationLifecycleIdle
	}
}

func (s *Store) appendConversationTranscript(text string) {
	if text == "" {
		return
	}
	if !s.Conversation.hasActiveTranscript || s.Conversation.activeTranscriptIndex >= len(s.Conversation.Transcript) {
		row := ConversationTranscriptRow{
			Type: conversation.EventTypeStream,
			Text: text,
		}
		s.Conversation.Transcript = append(s.Conversation.Transcript, row)
		s.Conversation.activeTranscriptIndex = len(s.Conversation.Transcript) - 1
		s.Conversation.hasActiveTranscript = true
		return
	}
	s.Conversation.Transcript[s.Conversation.activeTranscriptIndex].Text += text
}

// RunProgressSnapshot returns a copy of the current RunProgress under a read lock.
// Returns nil if no progress is set.
func (s *Store) RunProgressSnapshot() *RunProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Dashboard.RunProgress == nil {
		return nil
	}
	return &RunProgress{
		CurrentIteration: s.Dashboard.RunProgress.CurrentIteration,
		MaxIterations:    s.Dashboard.RunProgress.MaxIterations,
		IterationPercent: s.Dashboard.RunProgress.IterationPercent,
		Status:           s.Dashboard.RunProgress.Status,
	}
}

func (s *Store) recordConversationToolIndicator(toolName, status string) {
	indicator := ConversationToolIndicator{
		ToolName: toolName,
		Status:   status,
	}
	s.Conversation.ToolIndicators = append(s.Conversation.ToolIndicators, indicator)
}

// SetPipelineItems updates the current pipeline items, normalizing nil slices.
func (s *Store) SetPipelineItems(items PipelineItems) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PipelineItems = copyPipelineItems(items)
}

// GetPipelineItems returns the current pipeline items under a read lock, ensuring slices are normalized.
func (s *Store) GetPipelineItems() PipelineItems {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyPipelineItems(s.PipelineItems)
}

// DeletePipelineItem invokes the configured deletion callback for the requested tab and identifier.
func (s *Store) DeletePipelineItem(tab Tab, identifier string) {
	if s == nil {
		return
	}
	if fn := s.DeletePipelineItemFunc; fn != nil {
		fn(tab, identifier)
	}
}
