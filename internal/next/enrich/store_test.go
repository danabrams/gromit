package enrich

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFactStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred"), 0o755)
	store := NewFactStore()

	facts := []InferredFact{
		{
			FactID:    "abc123",
			Category:  CategoryEntrypoint,
			Statement: "main.go is the primary entrypoint",
			Status:    StatusProposed,
			CreatedAt: time.Now(),
		},
	}

	if err := store.SaveFacts(dir, facts); err != nil {
		t.Fatalf("SaveFacts: %v", err)
	}

	loaded, err := store.LoadFacts(dir)
	if err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(loaded))
	}
	if loaded[0].Statement != "main.go is the primary entrypoint" {
		t.Errorf("Statement = %q", loaded[0].Statement)
	}
}

func TestFactStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred"), 0o755)
	store := NewFactStore()

	facts, err := store.LoadFacts(dir)
	if err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts, got %d", len(facts))
	}
}

func TestFactStore_MergeStatuses(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred"), 0o755)
	store := NewFactStore()

	// Save initial facts with accepted and rejected statuses
	existing := []InferredFact{
		{FactID: "abc", Status: StatusAccepted, Category: CategoryEntrypoint, Statement: "main"},
		{FactID: "def", Status: StatusRejected, Category: CategoryRiskyArea, Statement: "risky"},
		{FactID: "jkl", Status: StatusRejected, Category: CategoryGlossaryTerm, Statement: "re-proposed term"},
	}
	store.SaveFacts(dir, existing)

	// New enrichment produces overlapping facts.
	// abc re-appears (accepted, should stay accepted).
	// def does NOT re-appear (rejected, should become superseded).
	// jkl re-appears (rejected, should become proposed per design doc).
	incoming := []InferredFact{
		{FactID: "abc", Status: StatusProposed, Category: CategoryEntrypoint, Statement: "main"},
		{FactID: "ghi", Status: StatusProposed, Category: CategoryGlossaryTerm, Statement: "term"},
		{FactID: "jkl", Status: StatusProposed, Category: CategoryGlossaryTerm, Statement: "re-proposed term"},
	}

	merged := store.MergeWithExisting(existing, incoming)

	// abc should retain accepted status
	for _, f := range merged {
		if f.FactID == "abc" && f.Status != StatusAccepted {
			t.Errorf("abc should retain accepted status, got %v", f.Status)
		}
		// def should be superseded (not in incoming)
		if f.FactID == "def" && f.Status != StatusSuperseded {
			t.Errorf("def should be superseded, got %v", f.Status)
		}
		// ghi should be proposed
		if f.FactID == "ghi" && f.Status != StatusProposed {
			t.Errorf("ghi should be proposed, got %v", f.Status)
		}
		// jkl was rejected but re-appears in incoming — should be re-proposed
		if f.FactID == "jkl" && f.Status != StatusProposed {
			t.Errorf("jkl (rejected, re-appearing) should be proposed, got %v", f.Status)
		}
	}
}

func TestFactStore_UpdateStatusNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewFactStore()

	// Save facts that do NOT contain a fact with ID "nonexistent".
	facts := []InferredFact{
		{FactID: "abc", Status: StatusProposed, Category: CategoryEntrypoint, Statement: "main.go"},
	}
	if err := store.SaveFacts(dir, facts); err != nil {
		t.Fatalf("SaveFacts: %v", err)
	}

	err := store.UpdateStatus(dir, "nonexistent", StatusAccepted)
	if err == nil {
		t.Fatal("expected error for non-existent fact ID, got nil")
	}
	if got := err.Error(); got != `fact "nonexistent" not found` {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestFactStore_UpdateStatus(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred"), 0o755)
	store := NewFactStore()

	facts := []InferredFact{
		{FactID: "abc", Status: StatusProposed},
		{FactID: "def", Status: StatusProposed},
	}
	store.SaveFacts(dir, facts)

	if err := store.UpdateStatus(dir, "abc", StatusAccepted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	loaded, _ := store.LoadFacts(dir)
	for _, f := range loaded {
		if f.FactID == "abc" && f.Status != StatusAccepted {
			t.Errorf("abc should be accepted, got %v", f.Status)
		}
	}
}
