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

func TestFindProjectRoot_ProjectPathRelativeToWorkingDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, repoConfigName), []byte(""), 0o644); err != nil {
		t.Fatalf("write %s: %v", repoConfigName, err)
	}

	workDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir to work dir: %v", err)
	}

	relPath, err := filepath.Rel(workDir, root)
	if err != nil {
		t.Fatalf("rel path: %v", err)
	}

	origProjectPath := projectPath
	projectPath = relPath
	t.Cleanup(func() {
		projectPath = origProjectPath
	})

	got, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}
	assertSameProjectRoot(t, got, root)
}

func TestFindProjectRoot_UsesProjectPathFlag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gromit.yaml"), []byte(""), 0644); err != nil {
		t.Fatalf("write gromit.yaml: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	parent := filepath.Dir(root)
	if err := os.Chdir(parent); err != nil {
		t.Fatalf("chdir to parent: %v", err)
	}

	origProjectPath := projectPath
	t.Cleanup(func() {
		projectPath = origProjectPath
	})
	projectPath = filepath.Base(root)

	want, err := absPath(projectPath, "project path flag")
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}

	got, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}
	if got != want {
		t.Fatalf("root = %q, want %q", got, want)
	}
}

func TestFindProjectRoot_ProjectPathRelativeToInitialWorkingDir(t *testing.T) {
	t.Parallel()

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, repoConfigName), []byte(""), 0o644); err != nil {
		t.Fatalf("write gromit.yaml: %v", err)
	}

	relPath, err := filepath.Rel(origWD, root)
	if err != nil {
		t.Fatalf("rel root: %v", err)
	}

	origProjectPath := projectPath
	projectPath = relPath
	t.Cleanup(func() {
		projectPath = origProjectPath
	})

	otherDir := t.TempDir()
	if err := os.Chdir(otherDir); err != nil {
		t.Fatalf("chdir to other dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	got, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}
	assertSameProjectRoot(t, got, root)
}

func TestFindProjectRoot_WalksUpFromNestedSubdir(t *testing.T) {
	t.Parallel()

	root, nested := createRepoRootWithSubdir(t)

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir to nested dir: %v", err)
	}

	origProjectPath := projectPath
	projectPath = ""
	t.Cleanup(func() {
		projectPath = origProjectPath
	})

	got, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}
	assertSameProjectRoot(t, got, root)
}

func TestFindProjectRoot_WalksUpFromWorkingDirWhenProjectPathUnset(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, repoConfigName), []byte(""), 0o644); err != nil {
		t.Fatalf("write gromit.yaml: %v", err)
	}
	subdir := filepath.Join(root, "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	origProjectPath := projectPath
	projectPath = ""
	t.Cleanup(func() {
		projectPath = origProjectPath
	})

	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("chdir to subdir: %v", err)
	}

	got, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}
	assertSameProjectRoot(t, got, root)
}

func assertSameProjectRoot(t *testing.T, got, want string) {
	t.Helper()
	gotRoot, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("eval symlinks got: %v", err)
	}
	wantRoot, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("eval symlinks want: %v", err)
	}
	if gotRoot != wantRoot {
		t.Fatalf("root = %q, want %q", gotRoot, wantRoot)
	}
}

func createRepoRootWithSubdir(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, repoConfigName), []byte(""), 0o644); err != nil {
		t.Fatalf("write %s: %v", repoConfigName, err)
	}
	subdir := filepath.Join(root, "nested", "child")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("create nested subdir: %v", err)
	}
	return root, subdir
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
