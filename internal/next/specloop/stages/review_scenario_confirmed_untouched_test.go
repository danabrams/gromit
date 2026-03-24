package stages

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// TestScenario_VerifierConfirmsRealBugOnUntouchedFile verifies that when the reviewer
// returns a blocking error finding for a file not in the current diff, and the verifier
// confirms the bug is real, the stage returns ReplanFrom with the finding intact and
// emits a review_finding_verified event with disposition=confirmed.
func TestScenario_VerifierConfirmsRealBugOnUntouchedFile(t *testing.T) {
	// Seed
	eventLogPath := filepath.Join(t.TempDir(), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	finding := review.Finding{
		Facet:       "concurrency",
		Severity:    review.SeverityError,
		File:        "internal/next/review/runner.go",
		Line:        120,
		Description: "concurrent map write without lock",
		Cycle:       1,
	}

	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings:         []review.Finding{finding},
			BlockingFindings:    []review.Finding{finding},
			HasBlockingFindings: true,
			FindingsByFacet: map[string][]review.Finding{
				"concurrency": {finding},
			},
		},
	}

	// runner.go is NOT in the diff — only an unrelated file appears
	diffWithoutRunnerGo := "+++ b/internal/next/review/other.go\n"

	// Verifier reads lines 115–130 and confirms the map write is real
	stubVerifier := &stubFindingVerifier{
		disposition: review.DispositionConfirmed,
		reason:      "map write outside mutex at line 120",
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		Verifier:     stubVerifier,
		DiffProvider: &fakeDiffProvider{diff: diffWithoutRunnerGo},
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

	// Assert: stage returns ReplanFrom (confirmed blocking finding blocks progress)
	if action.Kind != specloop.ReplanFrom {
		t.Errorf("expected ReplanFrom, got %v", action.Kind)
	}

	// Assert: blocking finding with SeverityError is still in the failure context
	if action.Context == nil {
		t.Fatal("expected non-nil FailureContext on ReplanFrom action")
	}
	if len(action.Context.Failures) == 0 {
		t.Error("expected BlockingFindings to remain in action.Context.Failures after confirmed disposition")
	}

	// Assert: review_finding_verified event emitted with disposition=confirmed
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	var verifiedEvent *runstore.ReviewFindingVerifiedEvent
	for _, ev := range events {
		if ev.EventType() == "review_finding_verified" {
			if rfv, ok := ev.(*runstore.ReviewFindingVerifiedEvent); ok {
				verifiedEvent = rfv
				break
			}
		}
	}

	if verifiedEvent == nil {
		t.Fatal("expected review_finding_verified event to be emitted")
	}
	if verifiedEvent.Disposition != "confirmed" {
		t.Errorf("expected disposition=confirmed, got %q", verifiedEvent.Disposition)
	}
	if verifiedEvent.File != "internal/next/review/runner.go" {
		t.Errorf("expected File=internal/next/review/runner.go, got %q", verifiedEvent.File)
	}
	if verifiedEvent.Line != 120 {
		t.Errorf("expected Line=120, got %d", verifiedEvent.Line)
	}
	if verifiedEvent.Severity != "error" {
		t.Errorf("expected Severity=error, got %q", verifiedEvent.Severity)
	}
}
