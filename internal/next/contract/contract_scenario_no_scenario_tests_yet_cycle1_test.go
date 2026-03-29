package contract

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_AugmentWithTestAssertions_NoScenarioTestsYetCycle1_PreservesOriginalAssertions(t *testing.T) {
	// Seed
	workDir := t.TempDir()
	store := runstore.NewStore(filepath.Join(workDir, ".store"))
	rs := runstore.NewRunState("run-001", "proj-001")
	rs.SpecID = "spec-001"
	rs.Status = runstore.StatusRunning
	rs.StartedAt = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run state: %v", err)
	}

	sc := &ScenarioContract{
		Scenarios: []ScenarioAssertions{
			{
				Name: "No scenario tests yet (cycle 1)",
				Assertions: []ContractAssertion{
					{FileExists: "pkg/planner/plan.go"},
					{FileContains: &FileContainsAssertion{Path: "pkg/planner/plan.go", Pattern: "vision change resume"}},
					{FileNotContains: &FileContainsAssertion{Path: "pkg/planner/plan.go", Pattern: "deprecated flow"}},
				},
			},
		},
	}
	original := append([]ContractAssertion(nil), sc.Scenarios[0].Assertions...)

	// Invoke
	if err := AugmentWithTestAssertions(sc, workDir); err != nil {
		t.Fatalf("AugmentWithTestAssertions: %v", err)
	}

	// Assert
	got := sc.Scenarios[0].Assertions
	if len(got) != len(original) {
		t.Fatalf("expected %d assertions unchanged, got %d", len(original), len(got))
	}

	for i := range original {
		if got[i].FileExists != original[i].FileExists {
			t.Fatalf("assertion[%d] file_exists mismatch: got %q want %q", i, got[i].FileExists, original[i].FileExists)
		}
		if (got[i].FileContains == nil) != (original[i].FileContains == nil) {
			t.Fatalf("assertion[%d] file_contains presence mismatch", i)
		}
		if original[i].FileContains != nil {
			if got[i].FileContains.Path != original[i].FileContains.Path || got[i].FileContains.Pattern != original[i].FileContains.Pattern {
				t.Fatalf("assertion[%d] file_contains mismatch: got %+v want %+v", i, got[i].FileContains, original[i].FileContains)
			}
		}
		if (got[i].FileNotContains == nil) != (original[i].FileNotContains == nil) {
			t.Fatalf("assertion[%d] file_not_contains presence mismatch", i)
		}
		if original[i].FileNotContains != nil {
			if got[i].FileNotContains.Path != original[i].FileNotContains.Path || got[i].FileNotContains.Pattern != original[i].FileNotContains.Pattern {
				t.Fatalf("assertion[%d] file_not_contains mismatch: got %+v want %+v", i, got[i].FileNotContains, original[i].FileNotContains)
			}
		}
		if got[i].GoTestPass != nil {
			t.Fatalf("assertion[%d] unexpectedly added go_test_pass: %+v", i, got[i].GoTestPass)
		}
	}
}
