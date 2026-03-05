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
	beadUnstickedEvent := &BeadUnstickedEvent{BeadID: "b1", Reason: "manual"}
	beadSkippedEvent := &BeadSkippedEvent{BeadID: "b1", Reason: "test"}

	// Phase events
	buildStartEvent := &BuildStartEvent{BeadID: "b1", Model: "opus", Attempt: 1}
	buildCompleteEvent := &BuildCompleteEvent{BeadID: "b1", Success: true, Duration: 1 * time.Second}
	validationStartEvent := &ValidationStartEvent{BeadID: "b1"}
	validationPassEvent := &ValidationPassEvent{BeadID: "b1", Duration: 1 * time.Second}
	validationFailEvent := &ValidationFailEvent{BeadID: "b1", Duration: 1 * time.Second}
	reviewStartEvent := &ReviewStartEvent{BeadID: "b1", Model: "opus"}
	reviewCompleteEvent := &ReviewCompleteEvent{BeadID: "b1", Verdict: "pass"}
	reviewFindingEvent := &ReviewFindingEvent{
		BeadID:        "b1",
		Description:   "out of scope",
		SchemaVersion: ReviewFindingSchemaVersion,
	}
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

	// Gate events
	gateScopeEvent := &GateScopeEvent{BeadID: "b1", FileCount: 5, MaxFiles: 10, Action: "block"}
	gateStuckEvent := &GateStuckEvent{BeadID: "b1", Reason: "timeout"}
	gateSkipEvent := &GateSkipEvent{BeadID: "b1", Reason: "precheck_passed"}
	gateBlockEvent := &GateBlockEvent{BeadID: "b1", Reason: "stuck"}

	// Epilogue events
	epilogueStartEvent := &EpilogueStartEvent{BeadID: "b1", Iteration: 1, Success: true}
	epilogueCompleteEvent := &EpilogueCompleteEvent{BeadID: "b1", Success: true}
	beadCloseEvent := &BeadCloseEvent{BeadID: "b1"}
	beadCleanupEvent := &BeadCleanupEvent{BeadID: "b1", Action: "sync"}

	// Collect all events to test
	events := []Event{
		runStartEvent, runCompleteEvent, iterStartEvent, iterCompleteEvent,
		beadCompleteEvent, beadFailedEvent, beadStuckEvent, beadUnstickedEvent, beadSkippedEvent,
		buildStartEvent, buildCompleteEvent, validationStartEvent, validationPassEvent,
	validationFailEvent, reviewStartEvent, reviewCompleteEvent, reviewFindingEvent,
		analysisStartEvent, analysisCompleteEvent, retroStartEvent, retroCompleteEvent,
		heartbeatEvent, modelSelectedEvent, escalationEvent, stallDetectedEvent, scopeCheckEvent,
		decomposeStartEvent, subBeadCreatedEvent, decomposeCompleteEvent,
		gateScopeEvent, gateStuckEvent, gateSkipEvent, gateBlockEvent,
		epilogueStartEvent, epilogueCompleteEvent, beadCloseEvent, beadCleanupEvent,
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
		(&RunStartEvent{}).EventType():          true,
		(&RunCompleteEvent{}).EventType():       true,
		(&IterationStartEvent{}).EventType():    true,
		(&IterationCompleteEvent{}).EventType(): true,
		(&BeadCompleteEvent{}).EventType():      true,
		(&BeadFailedEvent{}).EventType():        true,
		(&BeadStuckEvent{}).EventType():         true,
		(&BeadUnstickedEvent{}).EventType():     true,
		(&BeadSkippedEvent{}).EventType():       true,
		(&BuildStartEvent{}).EventType():        true,
		(&BuildCompleteEvent{}).EventType():     true,
		(&ValidationStartEvent{}).EventType():   true,
		(&ValidationPassEvent{}).EventType():    true,
		(&ValidationFailEvent{}).EventType():    true,
		(&ReviewStartEvent{}).EventType():       true,
		(&ReviewCompleteEvent{}).EventType():    true,
		(&ReviewFindingEvent{}).EventType():     true,
		(&AnalysisStartEvent{}).EventType():     true,
		(&AnalysisCompleteEvent{}).EventType():  true,
		(&RetroStartEvent{}).EventType():        true,
		(&RetroCompleteEvent{}).EventType():     true,
		(&HeartbeatEvent{}).EventType():         true,
		(&ModelSelectedEvent{}).EventType():     true,
		(&EscalationEvent{}).EventType():        true,
		(&StallDetectedEvent{}).EventType():     true,
		(&ScopeCheckEvent{}).EventType():        true,
		(&DecomposeStartEvent{}).EventType():    true,
		(&SubBeadCreatedEvent{}).EventType():    true,
		(&DecomposeCompleteEvent{}).EventType(): true,
		(&GateScopeEvent{}).EventType():         true,
		(&GateStuckEvent{}).EventType():         true,
		(&GateSkipEvent{}).EventType():          true,
		(&GateBlockEvent{}).EventType():         true,
		(&EpilogueStartEvent{}).EventType():     true,
		(&EpilogueCompleteEvent{}).EventType():  true,
		(&BeadCloseEvent{}).EventType():         true,
		(&BeadCleanupEvent{}).EventType():       true,
	}

	// If we reach here, all EventType() strings are unique (map keys are unique)
	if len(eventTypes) != 37 {
		t.Errorf("Expected 37 unique event types, got %d", len(eventTypes))
	}
}

func TestEventTypesReportExpectedTypeAndTimestamp(t *testing.T) {
	t.Parallel()
	now := time.Unix(1234, 0)

	for _, tc := range specEventCases(now) {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.event.EventType(); got != tc.wantType {
				t.Fatalf("EventType() = %q, want %q", got, tc.wantType)
			}
			if eventTime := tc.event.EventTime(); !eventTime.Equal(now) {
				t.Fatalf("EventTime() = %v, want %v", eventTime, now)
			}
		})
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

type eventSpec struct {
	name     string
	wantType string
	event    Event
}

func specEventCases(ts time.Time) []eventSpec {
	return []eventSpec{
		{
			name:     "RunStartEvent",
			wantType: "run_start",
			event: &RunStartEvent{
				MaxIterations: 1,
				TimeBudget:    1 * time.Hour,
				DryRun:        true,
				TimeMixin:     TimeMixin{Time: ts},
			},
		},
		{
			name:     "RunCompleteEvent",
			wantType: "run_complete",
			event: &RunCompleteEvent{
				IterationsCompleted: 1,
				Reason:              "done",
				TimeMixin:           TimeMixin{Time: ts},
			},
		},
		{
			name:     "IterationStartEvent",
			wantType: "iteration_start",
			event: &IterationStartEvent{
				Iteration: 1,
				BeadID:    "b1",
				BeadTitle: "title",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "IterationCompleteEvent",
			wantType: "iteration_complete",
			event: &IterationCompleteEvent{
				Iteration: 1,
				BeadID:    "b1",
				Success:   true,
				Duration:  1 * time.Second,
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "BeadCompleteEvent",
			wantType: "bead_complete",
			event: &BeadCompleteEvent{
				BeadID:    "b1",
				BeadTitle: "title",
				Duration:  1 * time.Second,
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "BeadFailedEvent",
			wantType: "bead_failed",
			event: &BeadFailedEvent{
				BeadID:    "b1",
				BeadTitle: "title",
				Error:     "boom",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "BeadStuckEvent",
			wantType: "bead_stuck",
			event: &BeadStuckEvent{
				BeadID:    "b1",
				BeadTitle: "title",
				Reason:    "stuck",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "BeadUnstickedEvent",
			wantType: "bead_unsticked",
			event: &BeadUnstickedEvent{
				BeadID:    "b1",
				Reason:    "manual",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "BeadSkippedEvent",
			wantType: "bead_skipped",
			event: &BeadSkippedEvent{
				BeadID:    "b1",
				Reason:    "skip",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "BuildStartEvent",
			wantType: "build_start",
			event: &BuildStartEvent{
				BeadID:      "b1",
				Model:       "opus",
				Attempt:     1,
				MaxAttempts: 3,
				TimeMixin:   TimeMixin{Time: ts},
			},
		},
		{
			name:     "BuildCompleteEvent",
			wantType: "build_complete",
			event: &BuildCompleteEvent{
				BeadID:    "b1",
				Success:   true,
				Duration:  1 * time.Second,
				Cost:      0.5,
				TokensIn:  10,
				TokensOut: 5,
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "ValidationStartEvent",
			wantType: "validation_start",
			event: &ValidationStartEvent{
				BeadID:    "b1",
				Model:     "haiku",
				Commands:  []string{"go test"},
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "ValidationPassEvent",
			wantType: "validation_pass",
			event: &ValidationPassEvent{
				BeadID:    "b1",
				Duration:  1 * time.Second,
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "ValidationFailEvent",
			wantType: "validation_fail",
			event: &ValidationFailEvent{
				BeadID:    "b1",
				Output:    "fail",
				Duration:  1 * time.Second,
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "ReviewStartEvent",
			wantType: "review_start",
			event: &ReviewStartEvent{
				BeadID:    "b1",
				Model:     "opus",
				Thorough:  true,
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "ReviewCompleteEvent",
			wantType: "review_complete",
			event: &ReviewCompleteEvent{
				BeadID:    "b1",
				Verdict:   "pass",
				Issues:    []string{"issue"},
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "ReviewFindingEvent",
			wantType: "review_finding",
			event: &ReviewFindingEvent{
				BeadID:        "b1",
				Description:   "note",
				InScope:       false,
				SchemaVersion: ReviewFindingSchemaVersion,
				TimeMixin:     TimeMixin{Time: ts},
			},
		},
		{
			name:     "AnalysisStartEvent",
			wantType: "analysis_start",
			event: &AnalysisStartEvent{
				BeadID:    "b1",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "AnalysisCompleteEvent",
			wantType: "analysis_complete",
			event: &AnalysisCompleteEvent{
				BeadID:      "b1",
				Category:    "logic",
				Recoverable: true,
				Suggestion:  "retry",
				TimeMixin:   TimeMixin{Time: ts},
			},
		},
		{
			name:     "RetroStartEvent",
			wantType: "retro_start",
			event: &RetroStartEvent{
				BeadID:    "b1",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "RetroCompleteEvent",
			wantType: "retro_complete",
			event: &RetroCompleteEvent{
				BeadID:               "b1",
				ProvisionalLearnings: 2,
				RulesUpdated:         true,
				TimeMixin:            TimeMixin{Time: ts},
			},
		},
		{
			name:     "HeartbeatEvent",
			wantType: "heartbeat",
			event: &HeartbeatEvent{
				Elapsed:            1 * time.Second,
				ToolCalls:          1,
				FilesModified:      0,
				RateLimitHits:      0,
				WaitingForResponse: true,
				TimeMixin:          TimeMixin{Time: ts},
			},
		},
		{
			name:     "ModelSelectedEvent",
			wantType: "model_selected",
			event: &ModelSelectedEvent{
				Model:     "opus",
				Reason:    "priority",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "EscalationEvent",
			wantType: "escalation",
			event: &EscalationEvent{
				FromModel: "haiku",
				ToModel:   "opus",
				Attempt:   2,
				Reason:    "retry",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "StallDetectedEvent",
			wantType: "stall_detected",
			event: &StallDetectedEvent{
				Elapsed:   2 * time.Second,
				Threshold: 5 * time.Second,
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "ScopeCheckEvent",
			wantType: "scope_check",
			event: &ScopeCheckEvent{
				BeadID:     "b1",
				Complexity: "medium",
				Approved:   true,
				Reason:     "ok",
				TimeMixin:  TimeMixin{Time: ts},
			},
		},
		{
			name:     "DecomposeStartEvent",
			wantType: "decompose_start",
			event: &DecomposeStartEvent{
				BeadID:    "b1",
				BeadTitle: "title",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "SubBeadCreatedEvent",
			wantType: "subbead_created",
			event: &SubBeadCreatedEvent{
				ParentBeadID: "b1",
				SubBeadID:    "b2",
				SubBeadTitle: "sub",
				Index:        1,
				Total:        3,
				TimeMixin:    TimeMixin{Time: ts},
			},
		},
		{
			name:     "DecomposeCompleteEvent",
			wantType: "decompose_complete",
			event: &DecomposeCompleteEvent{
				BeadID:          "b1",
				SubBeadsCreated: 3,
				TimeMixin:       TimeMixin{Time: ts},
			},
		},
		{
			name:     "LogEvent",
			wantType: "log",
			event: &LogEvent{
				Level:     "info",
				Message:   "hello",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "GateScopeEvent",
			wantType: "gate_scope",
			event: &GateScopeEvent{
				BeadID:    "b1",
				FileCount: 5,
				MaxFiles:  10,
				Action:    "block",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "GateStuckEvent",
			wantType: "gate_stuck",
			event: &GateStuckEvent{
				BeadID:    "b1",
				Reason:    "timeout",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "GateSkipEvent",
			wantType: "gate_skip",
			event: &GateSkipEvent{
				BeadID:    "b1",
				Reason:    "precheck_passed",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "GateBlockEvent",
			wantType: "gate_block",
			event: &GateBlockEvent{
				BeadID:    "b1",
				Reason:    "stuck",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "EpilogueStartEvent",
			wantType: "epilogue_start",
			event: &EpilogueStartEvent{
				BeadID:    "b1",
				Iteration: 1,
				Success:   true,
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "EpilogueCompleteEvent",
			wantType: "epilogue_complete",
			event: &EpilogueCompleteEvent{
				BeadID:    "b1",
				Success:   true,
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "BeadCloseEvent",
			wantType: "bead_close",
			event: &BeadCloseEvent{
				BeadID:    "b1",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
		{
			name:     "BeadCleanupEvent",
			wantType: "bead_cleanup",
			event: &BeadCleanupEvent{
				BeadID:    "b1",
				Action:    "sync",
				TimeMixin: TimeMixin{Time: ts},
			},
		},
	}
}
