package runner

import (
	"testing"
	"time"

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

// TestResultToIterationLog_MapsTimeoutDecompositionAuditFields verifies that
// ResultToIterationLog correctly maps timeout decomposition audit fields from
// IterationResult to IterationLog.
func TestResultToIterationLog_MapsTimeoutDecompositionAuditFields(t *testing.T) {
	attemptTime := time.Now()
	result := &runtypes.IterationResult{
		BeadID:                           "test-bead",
		TimeoutDecompositionAttempted:    true,
		TimeoutDecompositionSucceeded:    true,
		TimeoutDecompositionAttemptTime:  attemptTime,
		TimeoutDecompositionOutcome:      "succeeded",
		TimeoutDecompositionReason:       "high_complexity_timeout_exceeded",
	}

	log := ResultToIterationLog(result)

	if log.TimeoutDecompositionAttempted != true {
		t.Errorf("TimeoutDecompositionAttempted = %v, want true", log.TimeoutDecompositionAttempted)
	}
	if log.TimeoutDecompositionSucceeded != true {
		t.Errorf("TimeoutDecompositionSucceeded = %v, want true", log.TimeoutDecompositionSucceeded)
	}
	if log.TimeoutDecompositionAttemptTime != attemptTime {
		t.Errorf("TimeoutDecompositionAttemptTime = %v, want %v", log.TimeoutDecompositionAttemptTime, attemptTime)
	}
	if log.TimeoutDecompositionOutcome != "succeeded" {
		t.Errorf("TimeoutDecompositionOutcome = %q, want %q", log.TimeoutDecompositionOutcome, "succeeded")
	}
	if log.TimeoutDecompositionReason != "high_complexity_timeout_exceeded" {
		t.Errorf("TimeoutDecompositionReason = %q, want %q", log.TimeoutDecompositionReason, "high_complexity_timeout_exceeded")
	}
}
