package runner

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

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
