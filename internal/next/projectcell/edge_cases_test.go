package projectcell

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFSStore_CorruptedProjectJSON(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	store := NewFSStore(filepath.Join(workspace, "projects"))
	cell, _ := store.Create("myapp", repoDir)

	// Corrupt the project.json
	os.WriteFile(filepath.Join(cell.CellPath, "project.json"), []byte("{invalid json"), 0o644)

	_, err := store.Get("myapp")
	if err == nil {
		t.Error("expected error for corrupted project.json")
	}
}

func TestFSStore_NonexistentPath(t *testing.T) {
	workspace := t.TempDir()
	store := NewFSStore(filepath.Join(workspace, "projects"))

	_, err := store.Create("myapp", "/nonexistent/path/to/repo")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestFSStore_EmptyRepo(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()

	// Init repo but don't commit anything
	cmd := exec.Command("git", "init", repoDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}

	store := NewFSStore(filepath.Join(workspace, "projects"))
	cell, err := store.Create("empty-repo", repoDir)
	if err != nil {
		t.Fatalf("Create should succeed for empty repo: %v", err)
	}
	if cell.Name != "empty-repo" {
		t.Errorf("Name = %q, want %q", cell.Name, "empty-repo")
	}
}

func TestFSStore_PathTraversal(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	store := NewFSStore(filepath.Join(workspace, "projects"))

	// Create with traversal name should fail
	_, err := store.Create("../../etc/evil", repoDir)
	if err == nil {
		t.Error("expected error for path traversal in Create")
	}

	// Get with traversal name should fail
	_, err = store.Get("../../../tmp/evil")
	if err == nil {
		t.Error("expected error for path traversal in Get")
	}

	// Delete with traversal name should fail
	err = store.Delete("../../etc")
	if err == nil {
		t.Error("expected error for path traversal in Delete")
	}
}

func TestFSStore_MissingSubdirectories(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	store := NewFSStore(filepath.Join(workspace, "projects"))
	cell, _ := store.Create("myapp", repoDir)

	// Delete artifacts subdirectory
	os.RemoveAll(filepath.Join(cell.CellPath, "artifacts"))

	// Get should still work (it reads project.json, not subdirectories)
	got, err := store.Get("myapp")
	if err != nil {
		t.Fatalf("Get should still work with missing subdirectory: %v", err)
	}
	if got.Name != "myapp" {
		t.Errorf("Name = %q, want %q", got.Name, "myapp")
	}
}


