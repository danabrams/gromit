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
