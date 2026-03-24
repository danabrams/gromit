package stages

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// stubFindingVerifier implements FindingVerifier with fixed disposition for testing.
type stubFindingVerifier struct {
	disposition review.VerifierDisposition
	reason      string
}

func (s *stubFindingVerifier) Verify(ctx context.Context, f review.Finding, workDir string) (review.VerifierResult, error) {
	return review.VerifierResult{
		Finding:     f,
		Disposition: s.disposition,
		Reason:      s.reason,
	}, nil
}

// stubFindingVerifierByFile implements FindingVerifier with different dispositions per file.
type stubFindingVerifierByFile struct {
	dispositionsByFile map[string]review.VerifierDisposition
}

func (s *stubFindingVerifierByFile) Verify(ctx context.Context, f review.Finding, workDir string) (review.VerifierResult, error) {
	disposition, ok := s.dispositionsByFile[f.File]
	if !ok {
		disposition = review.DispositionConfirmed
	}
	return review.VerifierResult{
		Finding:     f,
		Disposition: disposition,
	}, nil
}

// TestReviewStage_NilVerifier leaves all blocking findings unchanged and emits no verifier events (AC 8)
func TestReviewStage_NilVerifier(t *testing.T) {
	// Seed: Configure ReviewStage with nil Verifier, event log, and blocking findings exist
	eventLogPath := filepath.Join(t.TempDir(), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	blockingFinding := review.Finding{
		Facet:       "test_facet",
		Severity:    review.SeverityError,
		File:        "unmodified.go",
		Line:        42,
		Description: "blocking issue",
		Cycle:       1,
	}

	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings:         []review.Finding{blockingFinding},
			BlockingFindings:    []review.Finding{blockingFinding},
			HasBlockingFindings: true,
			FindingsByFacet: map[string][]review.Finding{
				"test_facet": {blockingFinding},
			},
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		Verifier:     nil,                         // explicitly nil
		DiffProvider: &fakeDiffProvider{diff: ""}, // empty diff so finding is out-of-diff
	}, eventLog)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	// Invoke
	action, err := stage.Run(context.Background(), rs)

	// Assert: no error
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Assert: ReplanFrom since blocking finding unchanged
	if action.Kind != specloop.ReplanFrom {
		t.Errorf("expected ReplanFrom, got %v", action.Kind)
	}

	// Assert: blocking findings unchanged (still in action context)
	if len(action.Context.Failures) == 0 {
		t.Error("expected blocking findings to remain in action context")
	}

	// Assert: no verifier events emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	for _, ev := range events {
		if ev.EventType() == "review_finding_verified" {
			t.Error("expected no review_finding_verified events when verifier is nil")
		}
	}
}
