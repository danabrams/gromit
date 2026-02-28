package tui

import (
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/conversation"
	"github.com/danabrams/gromit/internal/events"
)

func TestOnRunStart_UpdatesRunProgress(t *testing.T) {
	t.Parallel()

	store := &Store{}
	event := &events.RunStartEvent{
		MaxIterations: 5,
		TimeBudget:    10 * time.Minute,
		DryRun:        false,
		Time:          time.Unix(1000, 0),
	}

	store.OnRunStart(event)

	if store.Dashboard.RunProgress == nil {
		t.Fatalf("expected RunProgress to be set")
	}
	if store.Dashboard.RunProgress.MaxIterations != 5 {
		t.Fatalf("MaxIterations = %d, want 5", store.Dashboard.RunProgress.MaxIterations)
	}
	if store.Dashboard.RunProgress.CurrentIteration != 0 {
		t.Fatalf("CurrentIteration = %d, want 0", store.Dashboard.RunProgress.CurrentIteration)
	}
	if store.Dashboard.RunProgress.Status != "running" {
		t.Fatalf("Status = %q, want %q", store.Dashboard.RunProgress.Status, "running")
	}
}

func TestOnRunComplete_UpdatesRunProgress(t *testing.T) {
	t.Parallel()

	store := &Store{
		Dashboard: DashboardState{
			RunProgress: &RunProgress{
				CurrentIteration: 3,
				MaxIterations:    5,
				Status:           "running",
			},
		},
	}
	event := &events.RunCompleteEvent{
		IterationsCompleted: 3,
		Reason:              "success",
		Time:                time.Unix(2000, 0),
	}

	store.OnRunComplete(event)

	if store.Dashboard.RunProgress.Status != "completed" {
		t.Fatalf("Status = %q, want %q", store.Dashboard.RunProgress.Status, "completed")
	}
}

func TestOnRunComplete_MarksFailedOnFailureReason(t *testing.T) {
	t.Parallel()

	store := &Store{
		Dashboard: DashboardState{
			RunProgress: &RunProgress{
				CurrentIteration: 2,
				MaxIterations:    4,
				Status:           "running",
			},
		},
	}
	event := &events.RunCompleteEvent{
		IterationsCompleted: 2,
		Reason:              "failed: build error",
		Time:                time.Unix(3000, 0),
	}

	store.OnRunComplete(event)

	if store.Dashboard.RunProgress.Status != "failed" {
		t.Fatalf("Status = %q, want %q", store.Dashboard.RunProgress.Status, "failed")
	}
}

func TestOnIterationStart_UpdatesActivePhase(t *testing.T) {
	t.Parallel()

	store := &Store{}
	event := &events.IterationStartEvent{
		Iteration: 1,
		BeadID:    "bead-123",
		BeadTitle: "Test Feature",
		Time:      time.Unix(1000, 0),
	}

	store.OnIterationStart(event)

	if store.Dashboard.ActivePhase == nil {
		t.Fatalf("expected ActivePhase to be set")
	}
	if store.Dashboard.ActivePhase.BeadID != "bead-123" {
		t.Fatalf("BeadID = %q, want %q", store.Dashboard.ActivePhase.BeadID, "bead-123")
	}
	if store.Dashboard.ActivePhase.BeadTitle != "Test Feature" {
		t.Fatalf("BeadTitle = %q, want %q", store.Dashboard.ActivePhase.BeadTitle, "Test Feature")
	}
}

func TestOnIterationComplete_UpdatesProgressPercentage(t *testing.T) {
	t.Parallel()

	store := &Store{
		Dashboard: DashboardState{
			RunProgress: &RunProgress{
				CurrentIteration: 1,
				MaxIterations:    5,
			},
		},
	}
	event := &events.IterationCompleteEvent{
		Iteration: 1,
		BeadID:    "bead-123",
		Success:   true,
		Duration:  5 * time.Second,
		Time:      time.Unix(2000, 0),
	}

	store.OnIterationComplete(event)

	if store.Dashboard.RunProgress.CurrentIteration != 2 {
		t.Fatalf("CurrentIteration = %d, want 2", store.Dashboard.RunProgress.CurrentIteration)
	}
	if store.Dashboard.RunProgress.IterationPercent != 40 {
		t.Fatalf("IterationPercent = %d, want 40", store.Dashboard.RunProgress.IterationPercent)
	}
}

func TestOnBeadComplete_TracksRecentCompletion(t *testing.T) {
	t.Parallel()

	store := &Store{}
	event := &events.BeadCompleteEvent{
		BeadID:    "bead-123",
		BeadTitle: "Test Feature",
		Duration:  5 * time.Second,
		Time:      time.Unix(3000, 0),
	}

	store.OnBeadComplete(event)

	if len(store.Dashboard.RecentCompletions) != 1 {
		t.Fatalf("expected 1 recent completion, got %d", len(store.Dashboard.RecentCompletions))
	}
	if store.Dashboard.RecentCompletions[0].BeadID != "bead-123" {
		t.Fatalf("BeadID = %q, want %q", store.Dashboard.RecentCompletions[0].BeadID, "bead-123")
	}
	if store.Dashboard.RecentCompletions[0].Status != "completed" {
		t.Fatalf("Status = %q, want %q", store.Dashboard.RecentCompletions[0].Status, "completed")
	}
}

func TestOnBeadFailed_TracksRecentFailure(t *testing.T) {
	t.Parallel()

	store := &Store{}
	event := &events.BeadFailedEvent{
		BeadID:    "bead-456",
		BeadTitle: "Failed Feature",
		Error:     "test failed",
		Time:      time.Unix(4000, 0),
	}

	store.OnBeadFailed(event)

	if len(store.Dashboard.RecentCompletions) != 1 {
		t.Fatalf("expected 1 recent completion, got %d", len(store.Dashboard.RecentCompletions))
	}
	if store.Dashboard.RecentCompletions[0].Status != "failed" {
		t.Fatalf("Status = %q, want %q", store.Dashboard.RecentCompletions[0].Status, "failed")
	}
}

func TestRecentCompletions_LimitsTo10(t *testing.T) {
	t.Parallel()

	store := &Store{}
	for i := 0; i < 15; i++ {
		event := &events.BeadCompleteEvent{
			BeadID:    "bead-" + string(rune(i)),
			BeadTitle: "Feature",
			Duration:  time.Second,
			Time:      time.Unix(int64(5000+i), 0),
		}
		store.OnBeadComplete(event)
	}

	if len(store.Dashboard.RecentCompletions) != 10 {
		t.Fatalf("expected 10 recent completions, got %d", len(store.Dashboard.RecentCompletions))
	}
}

func TestOnBuildStart_UpdatesPhase(t *testing.T) {
	t.Parallel()

	store := &Store{
		Dashboard: DashboardState{
			ActivePhase: &ActivePhase{
				BeadID: "bead-123",
			},
		},
	}
	event := &events.BuildStartEvent{
		BeadID:      "bead-123",
		Model:       "haiku",
		Attempt:     1,
		MaxAttempts: 3,
		Time:        time.Unix(3000, 0),
	}

	store.OnBuildStart(event)

	if store.Dashboard.ActivePhase.Phase != "build" {
		t.Fatalf("Phase = %q, want %q", store.Dashboard.ActivePhase.Phase, "build")
	}
}

func TestOnValidationStart_UpdatesPhase(t *testing.T) {
	t.Parallel()

	store := &Store{
		Dashboard: DashboardState{
			ActivePhase: &ActivePhase{
				BeadID: "bead-123",
				Phase:  "build",
			},
		},
	}
	event := &events.ValidationStartEvent{
		BeadID:   "bead-123",
		Commands: []string{"make test"},
		Time:     time.Unix(4000, 0),
	}

	store.OnValidationStart(event)

	if store.Dashboard.ActivePhase.Phase != "validation" {
		t.Fatalf("Phase = %q, want %q", store.Dashboard.ActivePhase.Phase, "validation")
	}
}

func TestOnHeartbeat_UpdatesHealthIndicator(t *testing.T) {
	t.Parallel()

	store := &Store{}
	event := &events.HeartbeatEvent{
		Elapsed:            5 * time.Second,
		ToolCalls:          3,
		FilesModified:      2,
		RateLimitHits:      0,
		WaitingForResponse: true,
		Time:               time.Unix(5000, 0),
	}

	store.OnHeartbeat(event)

	if store.Dashboard.HealthIndicator == nil {
		t.Fatalf("expected HealthIndicator to be set")
	}
	if store.Dashboard.HealthIndicator.LastEventType != "heartbeat" {
		t.Fatalf("LastEventType = %q, want %q", store.Dashboard.HealthIndicator.LastEventType, "heartbeat")
	}
	if store.Dashboard.HealthIndicator.IsHealthy != true {
		t.Fatalf("IsHealthy = %v, want true", store.Dashboard.HealthIndicator.IsHealthy)
	}
}

func TestOnStallDetected_MarkUnhealthy(t *testing.T) {
	t.Parallel()

	store := &Store{
		Dashboard: DashboardState{
			HealthIndicator: &HealthIndicator{
				IsHealthy: true,
			},
		},
	}
	event := &events.StallDetectedEvent{
		Elapsed:   15 * time.Second,
		Threshold: 10 * time.Second,
		Time:      time.Unix(6000, 0),
	}

	store.OnStallDetected(event)

	if store.Dashboard.HealthIndicator.IsHealthy != false {
		t.Fatalf("IsHealthy = %v, want false", store.Dashboard.HealthIndicator.IsHealthy)
	}
	if store.Dashboard.HealthIndicator.LastEventType != "stall_detected" {
		t.Fatalf("LastEventType = %q, want %q", store.Dashboard.HealthIndicator.LastEventType, "stall_detected")
	}
}

func TestMapRunProgress_ExtractsProgressState(t *testing.T) {
	t.Parallel()

	store := &Store{
		Dashboard: DashboardState{
			RunProgress: &RunProgress{
				CurrentIteration: 2,
				MaxIterations:    5,
				IterationPercent: 40,
				Status:           "running",
			},
		},
	}

	result := MapRunProgress(store)

	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if result.CurrentIteration != 2 {
		t.Fatalf("CurrentIteration = %d, want 2", result.CurrentIteration)
	}
	if result.MaxIterations != 5 {
		t.Fatalf("MaxIterations = %d, want 5", result.MaxIterations)
	}
	if result.Percent != 40 {
		t.Fatalf("Percent = %d, want 40", result.Percent)
	}
}

func TestMapActivePhase_ExtractsPhaseState(t *testing.T) {
	t.Parallel()

	startTime := time.Unix(1000, 0)
	store := &Store{
		Dashboard: DashboardState{
			ActivePhase: &ActivePhase{
				BeadID:    "bead-123",
				BeadTitle: "Test Feature",
				Phase:     "build",
				StartTime: startTime,
			},
		},
	}

	result := MapActivePhase(store)

	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if result.BeadID != "bead-123" {
		t.Fatalf("BeadID = %q, want %q", result.BeadID, "bead-123")
	}
	if result.Phase != "build" {
		t.Fatalf("Phase = %q, want %q", result.Phase, "build")
	}
}

func TestOnBeadStuck_TracksStuckCompletion(t *testing.T) {
	t.Parallel()

	store := &Store{}
	event := &events.BeadStuckEvent{
		BeadID:    "bead-789",
		BeadTitle: "Stuck Feature",
		Reason:    "timeout",
		Time:      time.Unix(7000, 0),
	}

	store.OnBeadStuck(event)

	if len(store.Dashboard.RecentCompletions) != 1 {
		t.Fatalf("expected 1 recent completion, got %d", len(store.Dashboard.RecentCompletions))
	}
	if store.Dashboard.RecentCompletions[0].Status != "stuck" {
		t.Fatalf("Status = %q, want %q", store.Dashboard.RecentCompletions[0].Status, "stuck")
	}
}

func TestOnBuildComplete_UpdatesPhase(t *testing.T) {
	t.Parallel()

	store := &Store{
		Dashboard: DashboardState{
			ActivePhase: &ActivePhase{
				Phase: "build",
			},
		},
	}
	event := &events.BuildCompleteEvent{
		BeadID:    "bead-123",
		Success:   true,
		Duration:  10 * time.Second,
		Cost:      0.001,
		TokensIn:  1000,
		TokensOut: 200,
		Time:      time.Unix(8000, 0),
	}

	store.OnBuildComplete(event)

	if store.Dashboard.ActivePhase.Phase != "build" {
		t.Fatalf("Phase should still be build")
	}
}

func TestMapHealth_ExtractsHealthState(t *testing.T) {
	t.Parallel()

	store := &Store{
		Dashboard: DashboardState{
			HealthIndicator: &HealthIndicator{
				IsHealthy:       true,
				LastEventType:   "heartbeat",
				LastEventTime:   time.Unix(5000, 0),
				HasStalledBeads: false,
			},
		},
	}

	result := MapHealth(store)

	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if !result.IsHealthy {
		t.Fatalf("IsHealthy = %v, want true", result.IsHealthy)
	}
	if result.LastEventType != "heartbeat" {
		t.Fatalf("LastEventType = %q, want %q", result.LastEventType, "heartbeat")
	}
}

func TestMapRecentCompletions_ReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	store := &Store{}
	result := MapRecentCompletions(store)

	if result == nil {
		t.Fatalf("expected non-nil slice")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(result))
	}
}

func TestApplyConversationEvent_UpdatesConversationState(t *testing.T) {
	t.Parallel()

	store := &Store{}

	event := conversation.Event{
		Type: conversation.EventTypeStream,
		Text: "hello",
	}

	store.ApplyConversationEvent(event)

	if store.Conversation.EventCount != 1 {
		t.Errorf("EventCount = %d, want 1", store.Conversation.EventCount)
	}
	if store.Conversation.LastEvent == nil {
		t.Error("expected LastEvent to be set")
	} else if store.Conversation.LastEvent.Text != "hello" {
		t.Errorf("LastEvent.Text = %q, want %q", store.Conversation.LastEvent.Text, "hello")
	}
}

func TestApplyConversationEvent_CombinesMultipleEvents(t *testing.T) {
	t.Parallel()

	store := &Store{}

	event1 := conversation.Event{
		Type: conversation.EventTypeStream,
		Text: "first",
	}
	event2 := conversation.Event{
		Type: conversation.EventTypeStream,
		Text: "second",
	}

	store.ApplyConversationEvent(event1)
	store.ApplyConversationEvent(event2)

	if store.Conversation.EventCount != 2 {
		t.Errorf("EventCount = %d, want 2", store.Conversation.EventCount)
	}
	if store.Conversation.LastEvent.Text != "second" {
		t.Errorf("LastEvent.Text = %q, want %q", store.Conversation.LastEvent.Text, "second")
	}
}

func TestMapConversationEventToMsg(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		event conversation.Event
	}{
		{"stream", conversation.Event{Type: conversation.EventTypeStream, Text: "assistant"}},
		{"tool_wait", conversation.Event{Type: conversation.EventTypeToolWait, Text: "waiting", ToolName: "formatter"}},
		{"tool_result", conversation.Event{Type: conversation.EventTypeToolResult, Text: "result", ToolName: "formatter"}},
		{"done", conversation.Event{Type: conversation.EventTypeDone, Text: "complete"}},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg := MapConversationEventToMsg(tc.event)
			eventMsg, ok := msg.(conversationEventMsg)
			if !ok {
				t.Fatalf("expected conversationEventMsg, got %T", msg)
			}
			if eventMsg.Event != tc.event {
				t.Fatalf("event mismatch: got %+v, want %+v", eventMsg.Event, tc.event)
			}
		})
	}
}
