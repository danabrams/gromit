package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRepoRootFromSubdir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gromit.yaml"), []byte(""), 0644); err != nil {
		t.Fatalf("write gromit.yaml: %v", err)
	}
	subdir := filepath.Join(root, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	t.Chdir(subdir)

	if err := ensureRepoRoot(); err != nil {
		t.Fatalf("ensureRepoRoot: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd after ensure: %v", err)
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}

	if cwdAbs != rootAbs {
		t.Fatalf("cwd = %s, want %s", cwdAbs, rootAbs)
	}
}
