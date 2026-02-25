package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestResultToIterationLog_MapsExperimentFields verifies that ResultToIterationLog
// correctly maps ExperimentID and VariantID from IterationResult to IterationLog.
func TestResultToIterationLog_MapsExperimentFields(t *testing.T) {
	result := &runtypes.IterationResult{
		BeadID:       "test-bead",
		ExperimentID: "exp-123",
		VariantID:    "var-456",
	}

	log := ResultToIterationLog(result)

	if log.ExperimentID != "exp-123" {
		t.Errorf("ExperimentID = %q, want %q", log.ExperimentID, "exp-123")
	}
	if log.VariantID != "var-456" {
		t.Errorf("VariantID = %q, want %q", log.VariantID, "var-456")
	}
}
