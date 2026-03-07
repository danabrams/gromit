package stage

import (
	"testing"

	"github.com/danabrams/gromit/internal/v2/event"
)

func TestStageResultEventsAcceptTypedEvent(t *testing.T) {
	// Test that StageResult.Events can hold TypedEvent instances from new event system
	result := &StageResult{
		Decision: DecisionProceed,
		Events: []event.TypedEvent{
			&event.StageStartedEvent{
				Event: event.Event{
					SchemaVersion: event.SchemaVersion,
					Type:          event.EventTypeStageStarted,
				},
				StageName: "test",
			},
		},
	}

	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}

	typed, ok := result.Events[0].(event.TypedEvent)
	if !ok {
		t.Fatalf("expected event to be TypedEvent, got %T", result.Events[0])
	}

	if typed.EventType() != event.EventTypeStageStarted {
		t.Fatalf("expected EventTypeStageStarted, got %s", typed.EventType())
	}
}

func TestStageRequestConstruction(t *testing.T) {
	req := StageRequest{
		Bead: BeadInfo{ID: "spec"},
	}

	if req.Bead.ID != "spec" {
		t.Fatalf("expected bead ID to be spec, got %s", req.Bead.ID)
	}
}

func TestDecisionStrings(t *testing.T) {
	expected := map[Decision]string{
		DecisionProceed: "proceed",
		DecisionSkip:    "skip",
		DecisionBlock:   "block",
		DecisionFail:    "fail",
	}

	for dec, want := range expected {
		got, ok := decisionStrings[dec]
		if !ok {
			t.Fatalf("decision strings missing entry for %v", dec)
		}
		if got != want {
			t.Fatalf("expected %v string to be %q, got %q", dec, want, got)
		}
		if dec.String() != want {
			t.Fatalf("expected %v.String() to be %q, got %q", dec, want, dec.String())
		}
	}
}
