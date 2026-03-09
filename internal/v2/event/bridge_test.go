package event

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
)

func TestLegacyEventsFromTyped(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name     string
		typed    TypedEvent
		wantType string
		check    func(events.Event) bool
	}{
		{
			name: "spec started",
			typed: &SpecStartedEvent{
				Event:    Event{Timestamp: now},
				SpecID:   "spec-id",
				Worktree: "worktree",
			},
			wantType: "*events.SpecStartedEvent",
		},
		{
			name: "generation cap reached",
			typed: &GenerationCapReachedEvent{
				Event:         Event{Timestamp: now},
				GenerationCap: 3,
			},
			wantType: "*events.GenerationCapReachedEvent",
			check: func(evt events.Event) bool {
				gcEvt, ok := evt.(*events.GenerationCapReachedEvent)
				return ok && gcEvt.GenerationCap == 3
			},
		},
		{
			name: "build invocation start",
			typed: &BuildInvocationStartEvent{
				Event:       Event{Timestamp: now},
				BeadID:      "bead-1",
				Model:       "opus",
				Attempt:     2,
				MaxAttempts: 3,
			},
			wantType: "*events.BuildStartEvent",
			check: func(evt events.Event) bool {
				e, ok := evt.(*events.BuildStartEvent)
				return ok && e.BeadID == "bead-1" && e.Model == "opus" && e.Attempt == 2 && e.MaxAttempts == 3
			},
		},
		{
			name: "build invocation complete",
			typed: &BuildInvocationCompleteEvent{
				Event:        Event{Timestamp: now},
				BeadID:       "bead-2",
				Success:      true,
				Duration:     5 * time.Second,
				CostUSD:      1.23,
				InputTokens:  1000,
				OutputTokens: 200,
				PromptSize:   5000,
			},
			wantType: "*events.BuildCompleteEvent",
			check: func(evt events.Event) bool {
				e, ok := evt.(*events.BuildCompleteEvent)
				return ok && e.BeadID == "bead-2" && e.Success && e.Duration == 5*time.Second &&
					e.Cost == 1.23 && e.TokensIn == 1000 && e.TokensOut == 200
			},
		},
		{
			name: "model selected with reason",
			typed: &ModelSelectedEvent{
				Event:  Event{Timestamp: now},
				Model:  "sonnet",
				Reason: "complexity P1",
			},
			wantType: "*events.ModelSelectedEvent",
			check: func(evt events.Event) bool {
				e, ok := evt.(*events.ModelSelectedEvent)
				return ok && e.Model == "sonnet" && e.Reason == "complexity P1"
			},
		},
		{
			name: "model selected with provider and tier fallback",
			typed: &ModelSelectedEvent{
				Event:    Event{Timestamp: now},
				Model:    "opus",
				Provider: "anthropic",
				Tier:     "P0",
			},
			wantType: "*events.ModelSelectedEvent",
			check: func(evt events.Event) bool {
				e, ok := evt.(*events.ModelSelectedEvent)
				return ok && e.Model == "opus" && e.Reason == "anthropic via P0"
			},
		},
		{
			name: "stage started logs",
			typed: &StageStartedEvent{
				Event:     Event{Timestamp: now},
				StageName: "build",
				BeadID:    "bead-id",
				Iteration: 2,
			},
			wantType: "*events.LogEvent",
			check: func(evt events.Event) bool {
				logEvt, ok := evt.(*events.LogEvent)
				return ok &&
					strings.Contains(logEvt.Message, "build") &&
					strings.Contains(logEvt.Message, "iteration 2")
			},
		},
		{
			name: "spec review completed logs",
			typed: &SpecReviewCompletedEvent{
				Event:            Event{Timestamp: now},
				SpecID:           "spec-1",
				Verdict:          "pass",
				FindingCount:     3,
				CriticalFindings: 1,
			},
			wantType: "*events.LogEvent",
			check: func(evt events.Event) bool {
				logEvt, ok := evt.(*events.LogEvent)
				return ok && strings.Contains(logEvt.Message, "spec review") && strings.Contains(logEvt.Message, "findings=3")
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			legacy := legacyEventsFromTyped(tc.typed)
			if len(legacy) == 0 {
				t.Fatalf("no legacy events produced")
			}
			gotType := fmt.Sprintf("%T", legacy[0])
			if gotType != tc.wantType {
				t.Fatalf("legacy event type = %s, want %s", gotType, tc.wantType)
			}
			if tc.check != nil && !tc.check(legacy[0]) {
				t.Fatalf("check failed for event %+v", legacy[0])
			}
		})
	}
}

func TestConvertBeadCompleted_PopulatesNewFields(t *testing.T) {
	now := time.Now().UTC()
	e := &BeadCompletedEvent{
		Event:        Event{Timestamp: now},
		BeadID:       "bead-x",
		BeadTitle:    "title-x",
		Iteration:    1,
		Success:      true,
		Model:        "opus",
		CostUSD:      2.50,
		InputTokens:  3000,
		OutputTokens: 750,
		Duration:     10 * time.Second,
	}
	legacy := legacyEventsFromTyped(e)
	if len(legacy) < 2 {
		t.Fatalf("expected at least 2 legacy events, got %d", len(legacy))
	}
	bce, ok := legacy[1].(*events.BeadCompleteEvent)
	if !ok {
		t.Fatalf("expected *events.BeadCompleteEvent, got %T", legacy[1])
	}
	if bce.Model != "opus" {
		t.Errorf("Model = %q, want %q", bce.Model, "opus")
	}
	if bce.CostUSD != 2.50 {
		t.Errorf("CostUSD = %f, want %f", bce.CostUSD, 2.50)
	}
	if bce.InputTokens != 3000 {
		t.Errorf("InputTokens = %d, want %d", bce.InputTokens, 3000)
	}
	if bce.OutputTokens != 750 {
		t.Errorf("OutputTokens = %d, want %d", bce.OutputTokens, 750)
	}
	if bce.Duration != 10*time.Second {
		t.Errorf("Duration = %v, want %v", bce.Duration, 10*time.Second)
	}
}

// unknownEvent is a test-only TypedEvent implementation that doesn't match
// any known event type in the bridge switch. It verifies that consumers
// handle unknown event types gracefully (ignore, don't crash).
type unknownEvent struct {
	Event
}

func (unknownEvent) EventType() string { return "unknown.test" }

func TestLegacyEventsFromTyped_UnknownEventType(t *testing.T) {
	evt := &unknownEvent{Event: Event{Timestamp: time.Now().UTC()}}
	legacy := legacyEventsFromTyped(evt)
	if legacy != nil {
		t.Fatalf("expected nil for unknown event type, got %v", legacy)
	}
}

func TestBridgeTypedToLegacy_UnknownEventDoesNotPanic(t *testing.T) {
	typed := NewEmitter()
	legacy := events.NewEmitter()
	ch := legacy.Subscribe()
	defer legacy.Unsubscribe(ch)

	BridgeTypedToLegacy(typed, legacy)

	// Emit an unknown event followed by a known event.
	// The unknown event should be silently ignored; the known event should arrive.
	typed.Emit(&unknownEvent{Event: Event{Timestamp: time.Now().UTC()}})
	typed.Emit(&SpecStartedEvent{
		Event:    Event{Timestamp: time.Now().UTC()},
		SpecID:   "after-unknown",
		Worktree: "wt",
	})

	select {
	case evt, ok := <-ch:
		if !ok {
			t.Fatalf("legacy channel closed early")
		}
		if se, ok := evt.(*events.SpecStartedEvent); !ok {
			t.Fatalf("expected *events.SpecStartedEvent, got %T", evt)
		} else if se.SpecID != "after-unknown" {
			t.Fatalf("expected SpecID after-unknown, got %s", se.SpecID)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for legacy event after unknown event")
	}
}

func TestBridgeTypedEventsToLegacyEmitter(t *testing.T) {
	typed := NewEmitter()
	legacy := events.NewEmitter()
	ch := legacy.Subscribe()
	defer legacy.Unsubscribe(ch)

	BridgeTypedToLegacy(typed, legacy)

	typed.Emit(&SpecStartedEvent{
		Event:    Event{Timestamp: time.Now().UTC()},
		SpecID:   "bridge-spec",
		Worktree: "bridge-worktree",
	})

	select {
	case evt, ok := <-ch:
		if !ok {
			t.Fatalf("legacy channel closed early")
		}
		if _, ok := evt.(*events.SpecStartedEvent); !ok {
			t.Fatalf("legacy event type = %T, want *events.SpecStartedEvent", evt)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for legacy event")
	}
}
