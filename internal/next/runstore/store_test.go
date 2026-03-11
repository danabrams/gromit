package runstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestStore_List(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	s.Save(NewRunState("spec-1", "proj-1"))
	s.Save(NewRunState("spec-2", "proj-1"))
	s.Save(NewRunState("spec-3", "proj-2"))

	runs, err := s.List("proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("want 2 runs, got %d", len(runs))
	}
}

func TestStore_List_Empty(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	runs, err := s.List("proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("want 0 runs, got %d", len(runs))
	}
}

func TestStore_RunDir_Layout(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-1", "proj-1")
	s.Save(rs)

	runDir := s.RunDir(rs.RunID)
	if _, err := os.Stat(filepath.Join(runDir, "run.json")); err != nil {
		t.Fatal("run.json must exist in run dir")
	}

	taskDir := s.TaskDir(rs.RunID, "t-001")
	if !strings.Contains(taskDir, "tasks/t-001") {
		t.Fatalf("unexpected task dir: %s", taskDir)
	}

	evidenceDir := s.EvidenceDir(rs.RunID, "t-001")
	if !strings.Contains(evidenceDir, "tasks/t-001/evidence") {
		t.Fatalf("unexpected evidence dir: %s", evidenceDir)
	}
}

func TestStore_WriteAndReadTaskArtifact(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-1", "proj-1")
	s.Save(rs)

	err := s.WriteTaskArtifact(rs.RunID, "t-001", "result.json", map[string]string{"status": "done"})
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]string
	err = s.ReadTaskArtifact(rs.RunID, "t-001", "result.json", &result)
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "done" {
		t.Fatalf("want done, got %s", result["status"])
	}
}

func TestStore_ReadTaskArtifact_NotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	var result map[string]string
	err := s.ReadTaskArtifact("run-xxx", "t-001", "missing.json", &result)
	if err == nil {
		t.Fatal("expected error for missing artifact")
	}
}
