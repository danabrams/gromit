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

func TestDiagnose_ClassifiesConfiguredRootCauses(t *testing.T) {
	tests := []struct {
		name  string
		event map[string]interface{}
		want  RootCause
	}{
		{
			name: "flaky test from retry-only validation failure",
			event: map[string]interface{}{
				"type":       "stage.failed",
				"stage_name": "validate",
				"error":      "validation command failed but passed on retry",
			},
			want: RootCauseFlakyTest,
		},
		{
			name: "unclear bead description",
			event: map[string]interface{}{
				"type":       "stage.failed",
				"stage_name": "build",
				"error":      "bead description missing acceptance criteria and expected outputs",
			},
			want: RootCauseUnclearBead,
		},
		{
			name: "incorrect decomposition",
			event: map[string]interface{}{
				"type":       "stage.failed",
				"stage_name": "decompose",
				"error":      "task scope remained broad after decomposition and should be split again",
			},
			want: RootCauseBadDecomposition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnosis := Diagnose(Input{
				Events: []map[string]interface{}{tt.event},
			})
			if diagnosis.RootCause != tt.want {
				t.Fatalf("RootCause = %q, want %q", diagnosis.RootCause, tt.want)
			}
		})
	}
}
