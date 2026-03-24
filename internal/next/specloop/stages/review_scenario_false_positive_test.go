package stages

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// TestScenario_ReviewStage_VerifierSuppressesFalsePositive verifies that when the
// reviewer returns a blocking finding for a file that is NOT in the current diff,
// and the verifier determines the issue was already fixed, the stage suppresses the
// false positive and returns Continue with no blocking findings.
func TestScenario_ReviewStage_VerifierSuppressesFalsePositive(t *testing.T) {
	// Seed: blocking finding for validate.go line 42, which is NOT in the current diff
	evidenceDir := t.TempDir()
	eventLogPath := filepath.Join(evidenceDir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	blockingFinding := review.Finding{
		Facet:       "correctness",
		Severity:    review.SeverityError,
		File:        "internal/next/specloop/stages/validate.go",
		Line:        42,
		Description: "specACMentionsPath still uses bare Acceptance Criteria marker",
		Cycle:       1,
	}

	// Verifier reads lines 37–52 and finds no bare marker: returns DispositionFixed
	stubVerifier := &stubFindingVerifier{
		disposition: review.DispositionFixed,
		reason:      "no bare marker found in lines 37-52; fix was committed in a prior cycle",
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

	// Empty diff: validate.go is not in the current diff (fix landed in a prior cycle)
	stage := NewReviewStage(runner, ReviewStageConfig{
		Verifier:     stubVerifier,
		EvidenceDir:  evidenceDir,
		DiffProvider: &fakeDiffProvider{diff: ""},
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

	// Assert: stage returns Continue — false positive suppressed, no replan triggered
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue (false positive suppressed), got %v", action.Kind)
	}

	// Assert: no blocking failures propagated in the action context
	if action.Context != nil && len(action.Context.Failures) > 0 {
		t.Errorf("expected no blocking failures in action context, got %v", action.Context.Failures)
	}

	// Assert: review_finding_verified event with disposition "fixed" appended to event log
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	var foundVerifiedEvent *runstore.ReviewFindingVerifiedEvent
	for _, ev := range events {
		if ev.EventType() == "review_finding_verified" {
			if rfv, ok := ev.(*runstore.ReviewFindingVerifiedEvent); ok {
				foundVerifiedEvent = rfv
				break
			}
		}
	}

	if foundVerifiedEvent == nil {
		t.Fatal("expected review_finding_verified event to be emitted")
	}
	if foundVerifiedEvent.Disposition != "fixed" {
		t.Errorf("expected disposition=fixed in event, got %q", foundVerifiedEvent.Disposition)
	}
	if foundVerifiedEvent.File != "internal/next/specloop/stages/validate.go" {
		t.Errorf("expected File=internal/next/specloop/stages/validate.go in event, got %q", foundVerifiedEvent.File)
	}
	if foundVerifiedEvent.Line != 42 {
		t.Errorf("expected Line=42 in event, got %d", foundVerifiedEvent.Line)
	}

	// Assert: verifier-audit.jsonl appended with a JSON line containing disposition "fixed"
	auditPath := filepath.Join(evidenceDir, "verifier-audit.jsonl")
	auditData, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read verifier-audit.jsonl: %v", err)
	}

	var auditEntry review.VerifierAuditEntry
	if err := json.Unmarshal(auditData, &auditEntry); err != nil {
		t.Fatalf("parse audit entry from verifier-audit.jsonl: %v", err)
	}
	if auditEntry.Disposition != string(review.DispositionFixed) {
		t.Errorf("expected audit Disposition=fixed, got %q", auditEntry.Disposition)
	}
	if auditEntry.File != "internal/next/specloop/stages/validate.go" {
		t.Errorf("expected audit File=internal/next/specloop/stages/validate.go, got %q", auditEntry.File)
	}
	if auditEntry.Line != 42 {
		t.Errorf("expected audit Line=42, got %d", auditEntry.Line)
	}
}
