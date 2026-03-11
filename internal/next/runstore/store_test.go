package runstore

import "testing"

func TestStore_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-1", "proj-1")

	if err := s.Save(rs); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Get(rs.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SpecID != "spec-1" {
		t.Fatalf("want spec-1, got %s", loaded.SpecID)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	s := NewStore(t.TempDir())
	_, err := s.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}
