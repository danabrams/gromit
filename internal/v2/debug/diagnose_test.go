package debug

import (
	"testing"

	"github.com/danabrams/gromit/internal/v2/adapter"
)

func TestDiagnose_IdentifiesFailurePointAndRootCause(t *testing.T) {
	events := []map[string]interface{}{
		{"type": "stage.completed", "stage_name": "build", "decision": "Proceed"},
		{"type": "stage.failed", "stage_name": "build", "error": "provider reported unsuccessful result: no detail available"},
	}
	logEntries := []adapter.LogEntry{
		{Hash: "abc11111", Message: "[bead:b1/build/iter:1] Proceed"},
		{Hash: "abc22222", Message: "[bead:b1/build/iter:2] Fail"},
	}

	diagnosis := Diagnose(Input{
		Events:     events,
		LogEntries: logEntries,
	})

	if diagnosis.Stage != "build" {
		t.Fatalf("diagnosis.Stage = %q, want %q", diagnosis.Stage, "build")
	}
	if diagnosis.FailureCommit != "abc22222" {
		t.Fatalf("diagnosis.FailureCommit = %q, want %q", diagnosis.FailureCommit, "abc22222")
	}
	if diagnosis.RootCause != RootCauseBadBuildOutput {
		t.Fatalf("diagnosis.RootCause = %q, want %q", diagnosis.RootCause, RootCauseBadBuildOutput)
	}
	if diagnosis.FailureEvent == nil {
		t.Fatal("diagnosis.FailureEvent = nil, want non-nil")
	}
}
