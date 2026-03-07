package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDebug2Worktree_FindsExistingWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	specName := "my-spec"
	wtPath := filepath.Join(tmpDir, "spec-worktrees", specName)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := resolveDebug2Worktree(tmpDir, specName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != wtPath {
		t.Errorf("got %q, want %q", got, wtPath)
	}
}
