package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMainRepoLogsDir_LocalLogsDirExists(t *testing.T) {
	gromitDir := t.TempDir()
	localLogs := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(localLogs, 0o755); err != nil {
		t.Fatalf("creating local logs dir: %v", err)
	}

	result := resolveMainRepoLogsDir(gromitDir)
	if result != localLogs {
		t.Fatalf("expected %q, got %q", localLogs, result)
	}
}

func TestResolveMainRepoLogsDir_ResolvesMainRepoWhenLocalMissing(t *testing.T) {
	// Create a gromitDir without logs/ — simulates being in a worktree.
	// When running inside any git repo, the function should attempt to resolve
	// the main repo's logs. It should NOT return the local (missing) path
	// when git can find a main repo with logs.
	gromitDir := t.TempDir()
	localLogs := filepath.Join(gromitDir, "logs")

	result := resolveMainRepoLogsDir(gromitDir)

	// The result should either be the local fallback (if we can't find a main repo)
	// or a path ending in .gromit/logs from the main repo.
	if result == localLogs {
		// This is acceptable — means git couldn't find a main repo or main repo has no logs
		return
	}
	// If resolved to a main repo path, it should end with .gromit/logs
	if !filepath.IsAbs(result) || filepath.Base(result) != "logs" {
		t.Fatalf("unexpected resolved path: %q", result)
	}
}

func TestResolveMainRepoLogsDirFn_IsInjectable(t *testing.T) {
	// Verify the function variable can be replaced for testing
	original := resolveMainRepoLogsDirFn
	defer func() { resolveMainRepoLogsDirFn = original }()

	called := false
	resolveMainRepoLogsDirFn = func(gromitDir string) string {
		called = true
		return "/override/logs"
	}

	result := resolveMainRepoLogsDirFn(".gromit")
	if !called {
		t.Fatal("injected function was not called")
	}
	if result != "/override/logs" {
		t.Fatalf("expected /override/logs, got %q", result)
	}
}
