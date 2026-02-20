package runner

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestWriteIterationLog_PropagatesFailurePhase(t *testing.T) {
	mockLog := &mockIterationLogger{}
	r := &Runner{
		logger: mockLog,
		output: &strings.Builder{},
	}

	result := &runtypes.IterationResult{
		BeadID:       "bead-1",
		Model:        "haiku",
		Success:      false,
		FailurePhase: "validation",
	}

	r.writeIterationLog(1, result)

	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mockLog.Logs))
	}
	if mockLog.Logs[0].FailurePhase != "validation" {
		t.Errorf("FailurePhase = %q, want %q", mockLog.Logs[0].FailurePhase, "validation")
	}
}

func TestWriteIterationLog_PropagatesFailureCategory(t *testing.T) {
	mockLog := &mockIterationLogger{}
	r := &Runner{
		logger: mockLog,
		output: &strings.Builder{},
	}

	result := &runtypes.IterationResult{
		BeadID:          "bead-1",
		Model:           "haiku",
		Success:         false,
		FailureCategory: "rate_limited",
	}

	r.writeIterationLog(1, result)

	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mockLog.Logs))
	}
	if mockLog.Logs[0].FailureCategory != "rate_limited" {
		t.Errorf("FailureCategory = %q, want %q", mockLog.Logs[0].FailureCategory, "rate_limited")
	}
}

func TestWriteIterationLog_FailureFieldsEmptyOnSuccess(t *testing.T) {
	mockLog := &mockIterationLogger{}
	r := &Runner{
		logger: mockLog,
		output: &strings.Builder{},
	}

	result := &runtypes.IterationResult{
		BeadID:  "bead-1",
		Model:   "haiku",
		Success: true,
	}

	r.writeIterationLog(1, result)

	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mockLog.Logs))
	}
	if mockLog.Logs[0].FailurePhase != "" {
		t.Errorf("FailurePhase = %q, want empty on success", mockLog.Logs[0].FailurePhase)
	}
	if mockLog.Logs[0].FailureCategory != "" {
		t.Errorf("FailureCategory = %q, want empty on success", mockLog.Logs[0].FailureCategory)
	}
}

func TestWriteIterationLog_PropagatesSpecID(t *testing.T) {
	mockLog := &mockIterationLogger{}
	r := &Runner{
		logger: mockLog,
		output: &strings.Builder{},
	}

	result := &runtypes.IterationResult{
		BeadID:    "bead-1",
		BeadTitle: "Test Bead",
		Model:     "haiku",
		Success:   true,
		SpecID:    "my-spec",
	}

	r.writeIterationLog(1, result)

	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mockLog.Logs))
	}
	if mockLog.Logs[0].SpecID != "my-spec" {
		t.Errorf("SpecID = %q, want %q", mockLog.Logs[0].SpecID, "my-spec")
	}
}

func TestWriteIterationLog_PropagatesPromptDiagnosticsToMockLogger(t *testing.T) {
	mockLog := &mockIterationLogger{}
	r := &Runner{
		logger: mockLog,
		output: &strings.Builder{},
	}

	result := &runtypes.IterationResult{
		BeadID:    "bead-1",
		BeadTitle: "Test Bead",
		Model:     "haiku",
		Success:   true,
		PromptDiagnostics: &prompt.PromptDiagnostics{
			PromptType:      "build",
			EstimatedTokens: 144,
			ReportedTokens:  128,
			TokenDelta:      16,
		},
	}

	r.writeIterationLog(1, result)

	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mockLog.Logs))
	}
	if mockLog.Logs[0].PromptDiagnostics == nil {
		t.Fatal("expected PromptDiagnostics in iteration log")
	}
	if mockLog.Logs[0].PromptDiagnostics.PromptType != "build" {
		t.Errorf("PromptType = %q, want %q", mockLog.Logs[0].PromptDiagnostics.PromptType, "build")
	}
	if mockLog.Logs[0].PromptDiagnostics.ReportedTokens != 128 {
		t.Errorf("ReportedTokens = %d, want 128", mockLog.Logs[0].PromptDiagnostics.ReportedTokens)
	}
}

func TestWriteIterationLog_WritesTDDMetricsToJSONL(t *testing.T) {
	r, tmpDir := newTestRunnerWithLogger(t, false)
	result := &IterationResult{
		BeadID:    "tdd-bead-1",
		BeadTitle: "TDD metrics bead",
		Model:     "sonnet",
		Success:   false,
		Duration:  3 * time.Second,
		Error:     fmt.Errorf("green phase failed"),
		PhaseMetrics: []runtypes.PhaseMetric{
			{
				Phase:        "red",
				CycleNumber:  1,
				BeadID:       "tdd-bead-1",
				Model:        "haiku",
				Tier:         "low",
				InputTokens:  100,
				OutputTokens: 60,
				DurationMs:   1000,
				Success:      true,
			},
			{
				Phase:         "green",
				CycleNumber:   1,
				BeadID:        "tdd-bead-1",
				Model:         "sonnet",
				Tier:          "medium",
				InputTokens:   130,
				OutputTokens:  70,
				DurationMs:    1200,
				Success:       false,
				Escalated:     true,
				EscalatedFrom: "haiku",
			},
			{
				Phase:        "red",
				CycleNumber:  2,
				BeadID:       "tdd-bead-1",
				Model:        "sonnet",
				Tier:         "medium",
				InputTokens:  110,
				OutputTokens: 80,
				DurationMs:   900,
				Success:      true,
			},
		},
	}

	r.writeIterationLog(1, result)

	phaseRecords, err := logger.ReadTDDPhaseRecords(tmpDir)
	if err != nil {
		t.Fatalf("ReadTDDPhaseRecords() error: %v", err)
	}
	if len(phaseRecords) != 3 {
		t.Fatalf("phase records length = %d, want 3", len(phaseRecords))
	}
	if phaseRecords[1].Phase != "green" {
		t.Fatalf("phase[1].Phase = %q, want %q", phaseRecords[1].Phase, "green")
	}
	if !phaseRecords[1].Escalated {
		t.Fatal("phase[1].Escalated = false, want true")
	}
	if phaseRecords[1].EscalatedFrom != "haiku" {
		t.Fatalf("phase[1].EscalatedFrom = %q, want %q", phaseRecords[1].EscalatedFrom, "haiku")
	}

	summaries, err := logger.ReadTDDSummaries(tmpDir)
	if err != nil {
		t.Fatalf("ReadTDDSummaries() error: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("summary records length = %d, want 1", len(summaries))
	}

	summary := summaries[0]
	if summary.BeadID != "tdd-bead-1" {
		t.Fatalf("summary.BeadID = %q, want %q", summary.BeadID, "tdd-bead-1")
	}
	if summary.TotalCycles != 2 {
		t.Fatalf("summary.TotalCycles = %d, want 2", summary.TotalCycles)
	}
	if summary.TotalPhases != 3 {
		t.Fatalf("summary.TotalPhases = %d, want 3", summary.TotalPhases)
	}
	if summary.Success {
		t.Fatal("summary.Success = true, want false")
	}
	if summary.TotalDurationMs != 3100 {
		t.Fatalf("summary.TotalDurationMs = %d, want 3100", summary.TotalDurationMs)
	}
	if summary.TotalInputTokens != 340 {
		t.Fatalf("summary.TotalInputTokens = %d, want 340", summary.TotalInputTokens)
	}
	if summary.TotalOutputTokens != 210 {
		t.Fatalf("summary.TotalOutputTokens = %d, want 210", summary.TotalOutputTokens)
	}
	if summary.EscalationCount != 1 {
		t.Fatalf("summary.EscalationCount = %d, want 1", summary.EscalationCount)
	}
	if got := summary.PhaseSuccessRates["red"]; got != 1 {
		t.Fatalf("summary.PhaseSuccessRates[red] = %v, want 1", got)
	}
	if got := summary.PhaseSuccessRates["green"]; got != 0 {
		t.Fatalf("summary.PhaseSuccessRates[green] = %v, want 0", got)
	}
}

func TestWriteIterationLog_EmptyPhaseMetricsWritesNoTDDRecords(t *testing.T) {
	r, tmpDir := newTestRunnerWithLogger(t, false)
	result := &IterationResult{
		BeadID:    "bead-no-tdd",
		BeadTitle: "No metrics",
		Model:     "haiku",
		Success:   true,
		Duration:  time.Second,
	}

	r.writeIterationLog(1, result)

	phaseRecords, err := logger.ReadTDDPhaseRecords(tmpDir)
	if err != nil {
		t.Fatalf("ReadTDDPhaseRecords() error: %v", err)
	}
	if len(phaseRecords) != 0 {
		t.Fatalf("phase records length = %d, want 0", len(phaseRecords))
	}

	summaries, err := logger.ReadTDDSummaries(tmpDir)
	if err != nil {
		t.Fatalf("ReadTDDSummaries() error: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("summary records length = %d, want 0", len(summaries))
	}
}
