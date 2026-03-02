package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRoot_FallsBackWhenWorkingDirectoryRemoved(t *testing.T) {
	t.Parallel()

	expectedRoot := resolveProjectPath("t", ".")
	tempDir := t.TempDir()

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir to temp: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	if err := os.RemoveAll(tempDir); err != nil {
		t.Fatalf("remove temp dir: %v", err)
	}

	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}
	if root != expectedRoot {
		t.Fatalf("root = %q, want %q", root, expectedRoot)
	}
}

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

	rootAbs, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("eval symlinks root: %v", err)
	}
	cwdAbs, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatalf("eval symlinks cwd: %v", err)
	}

	if cwdAbs != rootAbs {
		t.Fatalf("cwd = %s, want %s", cwdAbs, rootAbs)
	}
}
