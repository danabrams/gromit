package doctrine

import (
	"path/filepath"
	"testing"
)

func TestFSStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewFSStore()

	d := Doctrine{
		Rules: []Rule{
			{ID: "arch-001", Summary: "Use hexagonal architecture", Scope: "architecture"},
		},
	}

	if err := store.Save(filepath.Join(dir, "doctrine"), d); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load(filepath.Join(dir, "doctrine"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Rules) != 1 || got.Rules[0].ID != "arch-001" {
		t.Errorf("Load = %+v, want 1 rule with ID arch-001", got)
	}
}

func TestFSStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewFSStore()

	got, err := store.Load(filepath.Join(dir, "doctrine"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Rules) != 0 {
		t.Errorf("expected empty rules, got %d", len(got.Rules))
	}
}

func TestRule_SourceAlwaysDeclared(t *testing.T) {
	r := NewRule("test-001", "Test rule", "testing")
	if r.Source != "declared" {
		t.Errorf("Source = %q, want %q", r.Source, "declared")
	}
	if r.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}
