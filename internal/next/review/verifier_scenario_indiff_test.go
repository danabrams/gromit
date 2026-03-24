package review

import (
	"context"
	"testing"
)

// TestScenario_VerifierSkippedForInDiffFiles verifies that when a blocking finding's file
// appears in the current diff, VerifyBlockingFindings passes it through to kept unchanged
// without ever invoking the verifier, and returns an empty results slice.
func TestScenario_VerifierSkippedForInDiffFiles(t *testing.T) {
	// Seed: reviewer returned one blocking error finding for validate.go,
	// and validate.go IS in the current diff (modified this cycle).
	finding := Finding{
		Facet:       "correctness",
		Severity:    SeverityError,
		File:        "internal/next/specloop/stages/validate.go",
		Line:        42,
		Description: "missing nil check before dereference",
		Cycle:       1,
	}

	diffFiles := map[string]bool{
		"internal/next/specloop/stages/validate.go": true,
	}

	verifier := &mockVerifier{
		dispositions: map[string]VerifierDisposition{},
	}

	// Invoke
	kept, results := VerifyBlockingFindings(
		context.Background(),
		[]Finding{finding},
		diffFiles,
		verifier,
		"/test/workdir",
	)

	// Assert: verifier is never invoked
	if verifier.callCount != 0 {
		t.Errorf("expected verifier call count 0 for in-diff file, got %d", verifier.callCount)
	}

	// Assert: finding passes through to kept unchanged
	if len(kept) != 1 {
		t.Fatalf("expected 1 kept finding, got %d", len(kept))
	}
	if kept[0].File != finding.File {
		t.Errorf("expected kept[0].File=%q, got %q", finding.File, kept[0].File)
	}
	if kept[0].Severity != finding.Severity {
		t.Errorf("expected kept[0].Severity=%q, got %q", finding.Severity.String(), kept[0].Severity.String())
	}
	if kept[0].Description != finding.Description {
		t.Errorf("expected kept[0].Description=%q, got %q", finding.Description, kept[0].Description)
	}

	// Assert: results is empty
	if len(results) != 0 {
		t.Errorf("expected empty results for in-diff finding, got %d results", len(results))
	}
}
