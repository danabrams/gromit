package enrich

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
)

// CategorySelectiveMock fails for specific categories and returns default facts for the rest.
type CategorySelectiveMock struct {
	failCategories map[EnrichmentCategory]bool
	defaultFacts   []InferredFact
}

func (m *CategorySelectiveMock) Enrich(ctx context.Context, category EnrichmentCategory, observed []fact.Fact, input EnrichInput) (EnrichResult, error) {
	if m.failCategories[category] {
		return EnrichResult{Category: category, Success: false, Error: "mock failure"}, fmt.Errorf("mock failure for %s", category)
	}
	return EnrichResult{
		Category:  category,
		Facts:     m.defaultFacts,
		FactCount: len(m.defaultFacts),
		Success:   true,
	}, nil
}

func TestOrchestrator_RunAll(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)

	mock := &MockEnricher{
		Facts: []InferredFact{
			{Category: CategoryEntrypoint, Statement: "main.go is the entrypoint"},
		},
	}

	orch := NewOrchestrator(mock, NewFactStore(), NewRunStore())

	observed := []fact.Fact{
		fact.New("f1", fact.Observed, "main.go exists", "file-tree"),
	}

	result, err := orch.Run(context.Background(), dir, observed, EnrichInput{ProjectName: "test"}, Config{
		Provider: "claude", Model: "sonnet", Reasoning: "medium", StalenessExpiryDays: 30,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.TotalFacts == 0 {
		t.Error("expected at least 1 fact")
	}
	if result.RunID == "" {
		t.Error("RunID should not be empty")
	}
}

func TestOrchestrator_PartialFailure(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)

	failingMock := &CategorySelectiveMock{
		failCategories: map[EnrichmentCategory]bool{
			CategoryOwnershipBoundary: true,
		},
		defaultFacts: []InferredFact{
			{Statement: "test fact"},
		},
	}

	orch := NewOrchestrator(failingMock, NewFactStore(), NewRunStore())
	result, err := orch.Run(context.Background(), dir, []fact.Fact{}, EnrichInput{}, DefaultConfig())
	if err != nil {
		t.Fatalf("Run should not error on partial failure: %v", err)
	}
	if len(result.FailedCategories) != 1 {
		t.Errorf("expected 1 failed category, got %d", len(result.FailedCategories))
	}
	if result.TotalFacts == 0 {
		t.Error("successful categories should still produce facts")
	}
}

func TestOrchestrator_MergesStatusesOnRerun(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)

	factStore := NewFactStore()
	// The content hash of {Category: "entrypoint", Statement: "main.go"} is "3edf5570ab22".
	// Use the same ID so the merge can match incoming facts to existing ones.
	factStore.SaveFacts(dir, []InferredFact{
		{FactID: "3edf5570ab22", Status: StatusAccepted, Category: CategoryEntrypoint, Statement: "main.go"},
	})

	mock := &MockEnricher{
		Facts: []InferredFact{
			{Category: CategoryEntrypoint, Statement: "main.go"},
		},
	}

	orch := NewOrchestrator(mock, factStore, NewRunStore())
	orch.Run(context.Background(), dir, []fact.Fact{}, EnrichInput{}, DefaultConfig())

	loaded, _ := factStore.LoadFacts(dir)
	for _, f := range loaded {
		if f.Statement == "main.go" && f.Status != StatusAccepted {
			t.Errorf("accepted fact should preserve status, got %v", f.Status)
		}
	}
}

func TestOrchestrator_NoObservedFacts(t *testing.T) {
	dir := t.TempDir()
	mock := &MockEnricher{
		Facts: []InferredFact{
			{Category: CategoryEntrypoint, Statement: "main.go"},
		},
	}

	orch := NewOrchestrator(mock, NewFactStore(), NewRunStore())
	_, err := orch.Run(context.Background(), dir, []fact.Fact{}, EnrichInput{}, DefaultConfig())
	if err == nil {
		t.Error("expected error when observed facts are empty and no artifacts directory exists")
	}
}

func TestOrchestrator_DryRun(t *testing.T) {
	dir := t.TempDir()

	mock := &MockEnricher{
		Facts: []InferredFact{
			{Category: CategoryEntrypoint, Statement: "main.go"},
		},
	}

	orch := NewOrchestrator(mock, NewFactStore(), NewRunStore())
	result, err := orch.DryRun(context.Background(), dir, []fact.Fact{}, EnrichInput{}, DefaultConfig())
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if result.TotalFacts == 0 {
		t.Error("DryRun should still produce facts")
	}

	// Verify nothing was written
	if _, err := os.Stat(filepath.Join(dir, "inferred", "facts.json")); err == nil {
		t.Error("DryRun should not write facts.json")
	}
}
