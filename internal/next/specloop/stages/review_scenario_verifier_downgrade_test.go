package stages

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// TestScenario_VerifierDowngradesOverclassifiedFinding verifies that when the verifier
// returns "downgraded" for an out-of-diff blocking finding, the finding is kept as a
// warning, HasBlockingFindings is false, and a review_finding_verified event with
// disposition "downgraded" is emitted.
func TestScenario_VerifierDowngradesOverclassifiedFinding(t *testing.T) {
	// Seed
	eventLogPath := t.TempDir() + "/events.jsonl"
	eventLog := runstore.NewEventLog(eventLogPath)

	blockingFinding := review.Finding{
		Facet:       "correctness",
		Severity:    review.SeverityError,
		File:        "cmd/gromit-next/stage_provider.go",
		Line:        75,
		Description: "potential nil dereference",
		Cycle:       1,
	}

	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings:         []review.Finding{blockingFinding},
			BlockingFindings:    []review.Finding{blockingFinding},
			HasBlockingFindings: true,
			FindingsByFacet: map[string][]review.Finding{
				"correctness": {blockingFinding},
			},
		},
	}

	// stage_provider.go is NOT in the diff
	diffWithoutStageProvider := "+++ b/internal/next/specloop/stages/review.go\n"

	stubVerifier := &stubFindingVerifier{
		disposition: review.DispositionDowngraded,
		reason:      "minor style issue, not a bug",
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		Verifier:     stubVerifier,
		DiffProvider: &fakeDiffProvider{diff: diffWithoutStageProvider},
		WorkDir:      t.TempDir(),
	}, eventLog)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	// Invoke
	result, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Assert: finding appears in kept with SeverityWarning — stage returns Continue
	if result.Kind != specloop.Continue {
		t.Errorf("expected Continue (warning does not block), got %v", result.Kind)
	}

	// Assert: result.HasBlockingFindings is false (warning does not meet SeverityError threshold)
	// Verified indirectly: if HasBlockingFindings were true, Run would return ReplanFrom.
	// The Continue action confirms it.

	// Assert: review_finding_verified event emitted with disposition "downgraded"
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
	if verifiedEvent.Disposition != "downgraded" {
		t.Errorf("expected disposition=downgraded, got %q", verifiedEvent.Disposition)
	}
	if verifiedEvent.File != "cmd/gromit-next/stage_provider.go" {
		t.Errorf("expected File=cmd/gromit-next/stage_provider.go, got %q", verifiedEvent.File)
	}
	if verifiedEvent.Line != 75 {
		t.Errorf("expected Line=75, got %d", verifiedEvent.Line)
	}
}
