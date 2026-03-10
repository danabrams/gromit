package projectcell

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
}

func TestFSStore_CreateAndGet(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	store := NewFSStore(filepath.Join(workspace, "projects"))
	cell, err := store.Create("myapp", repoDir)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cell.Name != "myapp" {
		t.Errorf("Name = %q, want %q", cell.Name, "myapp")
	}
	if cell.RepoPath != repoDir {
		t.Errorf("RepoPath = %q, want %q", cell.RepoPath, repoDir)
	}

	got, err := store.Get("myapp")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "myapp" || got.RepoPath != repoDir {
		t.Errorf("Get returned %+v", got)
	}
}

func TestFSStore_CreateDuplicate(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	store := NewFSStore(filepath.Join(workspace, "projects"))
	if _, err := store.Create("myapp", repoDir); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := store.Create("myapp", repoDir); err == nil {
		t.Error("expected error on duplicate create")
	}
}

func TestFSStore_CreateNonGitRepo(t *testing.T) {
	workspace := t.TempDir()
	notARepo := t.TempDir()

	store := NewFSStore(filepath.Join(workspace, "projects"))
	if _, err := store.Create("myapp", notARepo); err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestFSStore_List(t *testing.T) {
	workspace := t.TempDir()
	store := NewFSStore(filepath.Join(workspace, "projects"))

	repo1 := t.TempDir()
	initGitRepo(t, repo1)
	repo2 := t.TempDir()
	initGitRepo(t, repo2)

	store.Create("alpha", repo1)
	store.Create("beta", repo2)

	cells, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cells) != 2 {
		t.Errorf("List returned %d cells, want 2", len(cells))
	}
}

func TestFSStore_Delete(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	store := NewFSStore(filepath.Join(workspace, "projects"))
	store.Create("myapp", repoDir)

	if err := store.Delete("myapp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get("myapp"); err == nil {
		t.Error("Get should fail after Delete")
	}
}

func TestFSStore_CreateBuildsDirectoryStructure(t *testing.T) {
	workspace := t.TempDir()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	store := NewFSStore(filepath.Join(workspace, "projects"))
	cell, _ := store.Create("myapp", repoDir)

	for _, sub := range []string{"artifacts", "doctrine", "provenance", "guide"} {
		dir := filepath.Join(cell.CellPath, sub)
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("subdirectory %s missing: %v", sub, err)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
	}
}
