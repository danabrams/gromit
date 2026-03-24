package stages

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// callCountingVerifier tracks how many times Verify is called.
type callCountingVerifier struct {
	callCount int
}

func (v *callCountingVerifier) Verify(ctx context.Context, f review.Finding, workDir string) (review.VerifierResult, error) {
	v.callCount++
	return review.VerifierResult{
		Finding:     f,
		Disposition: review.DispositionConfirmed,
	}, nil
}

// TestScenario_VerifierSkipped_ForNonBlockingWarning verifies that when the reviewer
// returns a SeverityWarning finding on an out-of-diff file and Threshold is SeverityError,
// the verifier is never called, the finding appears in AllFindings, and the stage returns Continue.
func TestScenario_VerifierSkipped_ForNonBlockingWarning(t *testing.T) {
	// Seed: reviewer returns one warning finding; BlockingFindings is empty
	// because the warning severity is below the SeverityError threshold.
	warningFinding := review.Finding{
		Facet:       "code_quality",
		Severity:    review.SeverityWarning,
		File:        "out_of_diff_file.go", // out-of-diff: not in the empty diff
		Line:        42,
		Description: "consider refactoring this loop",
		Cycle:       1,
	}

	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings:         []review.Finding{warningFinding},
			BlockingFindings:    []review.Finding{}, // empty — warning is below Error threshold
			HasBlockingFindings: false,
			FindingsByFacet: map[string][]review.Finding{
				"code_quality": {warningFinding},
			},
		},
	}

	verifier := &callCountingVerifier{}

	eventLogPath := filepath.Join(t.TempDir(), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	stage := NewReviewStage(runner, ReviewStageConfig{
		Verifier:     verifier,
		DiffProvider: &fakeDiffProvider{diff: ""}, // empty diff — file is out-of-diff
		WorkDir:      t.TempDir(),
	}, eventLog)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	// Invoke
	action, err := stage.Run(context.Background(), rs)

	// Assert: no error
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Assert: Continue — no blocking findings
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue (no blocking findings), got %v", action.Kind)
	}

	// Assert: verifier was never called (warning does not meet blocking threshold)
	if verifier.callCount != 0 {
		t.Errorf("expected verifier to be called 0 times, got %d", verifier.callCount)
	}

	// Assert: warning finding appears in rs.ReviewFindings (derived from AllFindings)
	found := false
	for _, f := range rs.ReviewFindings {
		if strings.Contains(f, "consider refactoring this loop") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning finding in rs.ReviewFindings, got %v", rs.ReviewFindings)
	}

	// Assert: no review_finding_verified events emitted (verifier was skipped)
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	for _, ev := range events {
		if ev.EventType() == "review_finding_verified" {
			t.Error("expected no review_finding_verified events when finding is non-blocking")
		}
	}
}
