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

func TestStageRequestFindingsField(t *testing.T) {
	finding := SpecFinding{
		Title:       "missing tests",
		Description: "add regression coverage",
		Severity:    SpecFindingSeverityHigh,
		Category:    SpecFindingCategoryScope,
		Scope:       SpecFindingScopeSpec,
	}
	req := StageRequest{SpecFindings: []SpecFinding{finding}}

	if len(req.SpecFindings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(req.SpecFindings))
	}

	got := req.SpecFindings[0]
	if got.Severity != SpecFindingSeverityHigh {
		t.Fatalf("unexpected severity %q", got.Severity)
	}
	if got.Category != SpecFindingCategoryScope {
		t.Fatalf("unexpected category %q", got.Category)
	}
	if got.Scope != SpecFindingScopeSpec {
		t.Fatalf("unexpected scope %q", got.Scope)
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

func TestStageRequestIncludesFindings(t *testing.T) {
	req := StageRequest{
		Findings: []Finding{
			{
				Severity:      SeverityWarning,
				Category:      CategoryQuality,
				Scope:         ScopeGeneral,
				Description:   "needs review",
				AffectedFiles: []string{"review_spec_v2.md"},
			},
		},
	}

	if got := len(req.Findings); got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}

	reqFinding := req.Findings[0]
	if reqFinding.Severity != SeverityWarning {
		t.Fatalf("expected severity %s, got %s", SeverityWarning, reqFinding.Severity)
	}

	if reqFinding.Category != CategoryQuality {
		t.Fatalf("expected category %s, got %s", CategoryQuality, reqFinding.Category)
	}
}

func TestSpecFindingNormalizeNilFields(t *testing.T) {
	f := SpecFinding{}
	f.NormalizeNilFields()
	if f.AffectedFiles == nil {
		t.Fatalf("Expected AffectedFiles to be initialized, got nil")
	}
}
