//go:build !wasm

package events

import (
	"testing"
	"time"
)

// TestAllEventTypesImplementEvent tests that all event types implement the Event interface.
func TestAllEventTypesImplementEvent(t *testing.T) {
	t.Parallel()

	// Lifecycle events
	runStartEvent := &RunStartEvent{MaxIterations: 10, TimeBudget: 1 * time.Hour}
	runCompleteEvent := &RunCompleteEvent{IterationsCompleted: 5, Reason: "test"}
	iterStartEvent := &IterationStartEvent{Iteration: 1, BeadID: "b1", BeadTitle: "Test"}
	iterCompleteEvent := &IterationCompleteEvent{Iteration: 1, BeadID: "b1", Success: true}
	beadCompleteEvent := &BeadCompleteEvent{BeadID: "b1", BeadTitle: "Test", Duration: 1 * time.Second}
	beadFailedEvent := &BeadFailedEvent{BeadID: "b1", BeadTitle: "Test", Error: "failed"}
	beadStuckEvent := &BeadStuckEvent{BeadID: "b1", BeadTitle: "Test", Reason: "stalled"}
	beadSkippedEvent := &BeadSkippedEvent{BeadID: "b1", Reason: "test"}

	// Phase events
	buildStartEvent := &BuildStartEvent{BeadID: "b1", Model: "opus", Attempt: 1}
	buildCompleteEvent := &BuildCompleteEvent{BeadID: "b1", Success: true, Duration: 1 * time.Second}
	validationStartEvent := &ValidationStartEvent{BeadID: "b1"}
	validationPassEvent := &ValidationPassEvent{BeadID: "b1", Duration: 1 * time.Second}
	validationFailEvent := &ValidationFailEvent{BeadID: "b1", Duration: 1 * time.Second}
	reviewStartEvent := &ReviewStartEvent{BeadID: "b1", Model: "opus"}
	reviewCompleteEvent := &ReviewCompleteEvent{BeadID: "b1", Verdict: "pass"}
	analysisStartEvent := &AnalysisStartEvent{BeadID: "b1"}
	analysisCompleteEvent := &AnalysisCompleteEvent{BeadID: "b1", Category: "test"}
	retroStartEvent := &RetroStartEvent{BeadID: "b1"}
	retroCompleteEvent := &RetroCompleteEvent{BeadID: "b1"}

	// Progress events
	heartbeatEvent := &HeartbeatEvent{Elapsed: 1 * time.Second}
	modelSelectedEvent := &ModelSelectedEvent{Model: "opus", Reason: "test"}
	escalationEvent := &EscalationEvent{FromModel: "haiku", ToModel: "opus"}
	stallDetectedEvent := &StallDetectedEvent{Elapsed: 1 * time.Second, Threshold: 30 * time.Second}
	scopeCheckEvent := &ScopeCheckEvent{BeadID: "b1", Approved: true}

	// Decomposition events
	decomposeStartEvent := &DecomposeStartEvent{BeadID: "b1", BeadTitle: "Test"}
	subBeadCreatedEvent := &SubBeadCreatedEvent{ParentBeadID: "b1", SubBeadID: "b1-1", SubBeadTitle: "Sub"}
	decomposeCompleteEvent := &DecomposeCompleteEvent{BeadID: "b1", SubBeadsCreated: 1}

	// Collect all events to test
	events := []Event{
		runStartEvent, runCompleteEvent, iterStartEvent, iterCompleteEvent,
		beadCompleteEvent, beadFailedEvent, beadStuckEvent, beadSkippedEvent,
		buildStartEvent, buildCompleteEvent, validationStartEvent, validationPassEvent,
		validationFailEvent, reviewStartEvent, reviewCompleteEvent,
		analysisStartEvent, analysisCompleteEvent, retroStartEvent, retroCompleteEvent,
		heartbeatEvent, modelSelectedEvent, escalationEvent, stallDetectedEvent, scopeCheckEvent,
		decomposeStartEvent, subBeadCreatedEvent, decomposeCompleteEvent,
	}

	// Verify each event implements Event interface
	for _, evt := range events {
		// These calls verify the interface is satisfied
		_ = evt.EventType()
		_ = evt.EventTime()
	}
}

func TestValidationStartEventIncludesModel(t *testing.T) {
	t.Parallel()

	event := &ValidationStartEvent{
		Model: "haiku",
	}

	if event.Model != "haiku" {
		t.Fatalf("Model = %q, want %q", event.Model, "haiku")
	}
}

// TestEventTypeStringsAreUnique tests that each event type has a unique EventType() string.
func TestEventTypeStringsAreUnique(t *testing.T) {
	t.Parallel()

	eventTypes := map[string]bool{
		(&RunStartEvent{}).EventType():           true,
		(&RunCompleteEvent{}).EventType():        true,
		(&IterationStartEvent{}).EventType():     true,
		(&IterationCompleteEvent{}).EventType():  true,
		(&BeadCompleteEvent{}).EventType():       true,
		(&BeadFailedEvent{}).EventType():         true,
		(&BeadStuckEvent{}).EventType():          true,
		(&BeadSkippedEvent{}).EventType():        true,
		(&BuildStartEvent{}).EventType():         true,
		(&BuildCompleteEvent{}).EventType():      true,
		(&ValidationStartEvent{}).EventType():    true,
		(&ValidationPassEvent{}).EventType():     true,
		(&ValidationFailEvent{}).EventType():     true,
		(&ReviewStartEvent{}).EventType():        true,
		(&ReviewCompleteEvent{}).EventType():     true,
		(&AnalysisStartEvent{}).EventType():      true,
		(&AnalysisCompleteEvent{}).EventType():   true,
		(&RetroStartEvent{}).EventType():         true,
		(&RetroCompleteEvent{}).EventType():      true,
		(&HeartbeatEvent{}).EventType():          true,
		(&ModelSelectedEvent{}).EventType():      true,
		(&EscalationEvent{}).EventType():         true,
		(&StallDetectedEvent{}).EventType():      true,
		(&ScopeCheckEvent{}).EventType():         true,
		(&DecomposeStartEvent{}).EventType():     true,
		(&SubBeadCreatedEvent{}).EventType():     true,
		(&DecomposeCompleteEvent{}).EventType():  true,
	}

	// If we reach here, all EventType() strings are unique (map keys are unique)
	if len(eventTypes) != 27 {
		t.Errorf("Expected 27 unique event types, got %d", len(eventTypes))
	}
}

// TestEventTimeReturnsValidTime tests that EventTime() returns a valid time.
func TestEventTimeReturnsValidTime(t *testing.T) {
	t.Parallel()

	event := &RunStartEvent{MaxIterations: 10, TimeBudget: 1 * time.Hour}
	eventTime := event.EventTime()

	// Should not be zero time
	if eventTime.IsZero() {
		t.Error("EventTime() returned zero time")
	}

	// Should be close to now
	now := time.Now()
	if eventTime.After(now.Add(1 * time.Second)) {
		t.Error("EventTime() is in the future")
	}
}
