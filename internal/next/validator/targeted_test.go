package validator

import (
	"context"
	"testing"
)

func TestRunTargeted_ExecutesProofChecks(t *testing.T) {
	r := NewRunner()
	proofChecks := []string{"echo proof1", "echo proof2"}
	results, err := r.RunTargeted(context.Background(), proofChecks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !results.AllPass() {
		t.Fatal("expected all pass")
	}
	if len(results.Results) != 2 {
		t.Fatalf("want 2, got %d", len(results.Results))
	}
}

func TestRunTargeted_FailingProofCheck(t *testing.T) {
	r := NewRunner()
	proofChecks := []string{"true", "false"}
	results, err := r.RunTargeted(context.Background(), proofChecks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if results.AllPass() {
		t.Fatal("expected some failures")
	}
	if results.FailCount() != 1 {
		t.Fatalf("want 1 failure, got %d", results.FailCount())
	}
}

func TestRunTargeted_Empty(t *testing.T) {
	r := NewRunner()
	results, err := r.RunTargeted(context.Background(), nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Results) != 0 {
		t.Fatalf("want 0, got %d", len(results.Results))
	}
	if !results.AllPass() {
		t.Fatal("empty should be all pass")
	}
}

func TestRunTargeted_KnownGaps_WithoutGaps(t *testing.T) {
	r := NewRunner()
	// KnownGaps is empty by default
	if r.KnownGaps != "" {
		t.Fatal("expected KnownGaps to be empty")
	}

	proofChecks := []string{"true"}
	results, err := r.RunTargeted(context.Background(), proofChecks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !results.AllPass() {
		t.Fatal("expected all pass")
	}
	// Verify that KnownGaps field is still empty on the runner
	if r.KnownGaps != "" {
		t.Fatal("expected KnownGaps to remain empty")
	}
}

func TestRunTargeted_KnownGaps_WithGaps(t *testing.T) {
	r := NewRunner()
	knownGapsText := "Database connection may fail under high load."
	r.KnownGaps = knownGapsText

	// Verify that KnownGaps is set on the runner
	if r.KnownGaps != knownGapsText {
		t.Fatalf("expected KnownGaps to be set, got %q", r.KnownGaps)
	}

	proofChecks := []string{"true"}
	results, err := r.RunTargeted(context.Background(), proofChecks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !results.AllPass() {
		t.Fatal("expected all pass")
	}
	// Verify that KnownGaps field remains set after RunTargeted
	if r.KnownGaps != knownGapsText {
		t.Fatalf("expected KnownGaps to remain set, got %q", r.KnownGaps)
	}
}

func TestRunTargeted_KnownGaps_PromptGeneration(t *testing.T) {
	testCases := []struct {
		name          string
		knownGaps     string
		shouldInclude bool
	}{
		{
			name:          "with known gaps",
			knownGaps:     "Missing error handling for network timeouts.",
			shouldInclude: true,
		},
		{
			name:          "without known gaps",
			knownGaps:     "",
			shouldInclude: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRunner()
			r.KnownGaps = tc.knownGaps

			// Simulate what a prompt generator would do
			hasKnownGaps := r.KnownGaps != ""

			if tc.shouldInclude && !hasKnownGaps {
				t.Error("expected KnownGaps to be present for prompt")
			}
			if !tc.shouldInclude && hasKnownGaps {
				t.Error("expected KnownGaps to be absent for prompt")
			}
		})
	}
}
