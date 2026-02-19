package runner

import (
	"strings"
	"testing"

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
