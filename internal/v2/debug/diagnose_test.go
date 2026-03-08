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

func TestDiagnose_InfersFlakyRootCauseFromValidationStageDetails(t *testing.T) {
		events := []map[string]interface{}{
			{
				"type":       "stage.failed",
				"stage_name": "validate",
				"error":      "validation stage failed unexpectedly",
			},
		{
			"type":           "validation",
			"stage_name":     "validate",
			"failed_command": "go test ./cmd/gromit",
			"details":        "timeout waiting for fixture cleanup",
			"succeeded":      false,
		},
	}

	diagnosis := Diagnose(Input{Events: events})
	if diagnosis.RootCause != RootCauseFlakyTest {
		t.Fatalf("RootCause = %q, want %q", diagnosis.RootCause, RootCauseFlakyTest)
	}
}

func TestDiagnose_CollectsStageValidationDetails(t *testing.T) {
	events := []map[string]interface{}{
		{
			"type":       "stage.failed",
			"stage_name": "validate",
			"bead_id":    "b1",
			"iteration":  1,
			"error":      "validation pipeline failed",
		},
		{
			"type":           "validation",
			"stage_name":     "validate",
			"bead_id":        "b1",
			"iteration":      1,
			"commands":       []interface{}{"go test ./cmd/gromit"},
			"failed_command": "go test ./cmd/gromit",
			"details":        "stdout: panic in TestFoo",
			"succeeded":      false,
		},
	}

	diagnosis := Diagnose(Input{Events: events})

	if diagnosis.StageTrace.StageName != "validate" {
		t.Fatalf("StageTrace.StageName = %q, want %q", diagnosis.StageTrace.StageName, "validate")
	}
	if diagnosis.StageTrace.Validation == nil {
		t.Fatal("StageTrace.Validation = nil, want non-nil")
	}
	if len(diagnosis.StageTrace.Events) != 2 {
		t.Fatalf("StageTrace.Events len = %d, want %d", len(diagnosis.StageTrace.Events), 2)
	}
	if len(diagnosis.StageTrace.Validation.Commands) != 1 {
		t.Fatalf("validation commands = %v, want 1 entry", diagnosis.StageTrace.Validation.Commands)
	}
	if diagnosis.StageTrace.Validation.Details != "stdout: panic in TestFoo" {
		t.Fatalf("validation details = %q, want %q", diagnosis.StageTrace.Validation.Details, "stdout: panic in TestFoo")
	}
}

func TestDiagnose_FallsBackToGitHistoryForStageAndRootCause(t *testing.T) {
	logEntries := []adapter.LogEntry{
		{Hash: "abc11111", Message: "[bead:b1/build/iter:1] Proceed"},
		{Hash: "abc22222", Message: "[bead:b1/decompose/iter:2] Fail"},
	}

	diagnosis := Diagnose(Input{
		LogEntries: logEntries,
	})

	if diagnosis.Stage != "decompose" {
		t.Fatalf("diagnosis.Stage = %q, want %q", diagnosis.Stage, "decompose")
	}
	if diagnosis.FailureCommit != "abc22222" {
		t.Fatalf("diagnosis.FailureCommit = %q, want %q", diagnosis.FailureCommit, "abc22222")
	}
	if diagnosis.RootCause != RootCauseBadDecomposition {
		t.Fatalf("diagnosis.RootCause = %q, want %q", diagnosis.RootCause, RootCauseBadDecomposition)
	}
}

func TestDiagnose_StageTraceUsesCommitInfoWhenEventStageMissing(t *testing.T) {
	events := []map[string]interface{}{
		{"type": "stage.failed", "bead_id": "b2", "error": "stage metadata missing"},
	}
	logEntries := []adapter.LogEntry{
		{Hash: "abc33333", Message: "[bead:b2/build/iter:4] Fail"},
	}

	diagnosis := Diagnose(Input{Events: events, LogEntries: logEntries})

	if diagnosis.StageTrace.StageName != "build" {
		t.Fatalf("StageTrace.StageName = %q, want %q", diagnosis.StageTrace.StageName, "build")
	}
	if diagnosis.StageTrace.CommitHash != "abc33333" {
		t.Fatalf("StageTrace.CommitHash = %q, want %q", diagnosis.StageTrace.CommitHash, "abc33333")
	}
	if diagnosis.StageTrace.CommitDecision != "Fail" {
		t.Fatalf("StageTrace.CommitDecision = %q, want %q", diagnosis.StageTrace.CommitDecision, "Fail")
	}
	if diagnosis.StageTrace.BeadID != "b2" {
		t.Fatalf("StageTrace.BeadID = %q, want %q", diagnosis.StageTrace.BeadID, "b2")
	}
	if diagnosis.StageTrace.Iteration != 4 {
		t.Fatalf("StageTrace.Iteration = %d, want %d", diagnosis.StageTrace.Iteration, 4)
	}
}
