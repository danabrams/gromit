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

// TestResultToIterationLog_MapsSpecIDWithLabel verifies that ResultToIterationLog
// correctly maps SpecID from IterationResult to IterationLog when spec label is present.
func TestResultToIterationLog_MapsSpecIDWithLabel(t *testing.T) {
	result := &runtypes.IterationResult{
		BeadID: "test-bead",
		SpecID: "authentication",
	}

	log := ResultToIterationLog(result)

	if log.SpecID != "authentication" {
		t.Errorf("SpecID = %q, want %q", log.SpecID, "authentication")
	}
}

// TestResultToIterationLog_MapsSpecIDWithoutLabel verifies that ResultToIterationLog
// correctly maps an empty SpecID from IterationResult to IterationLog.
func TestResultToIterationLog_MapsSpecIDWithoutLabel(t *testing.T) {
	result := &runtypes.IterationResult{
		BeadID: "test-bead",
		SpecID: "",
	}

	log := ResultToIterationLog(result)

	if log.SpecID != "" {
		t.Errorf("SpecID = %q, want empty string", log.SpecID)
	}
}
