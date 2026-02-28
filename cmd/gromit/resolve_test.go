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

func TestSetupRetroWorktreeLogsSymlink_CreatesSymlinkWhenSessionCreated(t *testing.T) {
	// RED test: Verify that when a retro worktree is created via CreateSessionWorktree,
	// the worktree's .gromit/logs directory is symlinked or pass-through from main repo's logs.
	//
	// Scenario: A retro command creates a session worktree. The worktree has a .gromit
	// directory created, but .gromit/logs is gitignored and doesn't exist. This function
	// should set up a symlink from worktree/.gromit/logs to the main repo's .gromit/logs.

	tmpDir := t.TempDir()

	// Create main repo structure with logs
	mainRepoRoot := filepath.Join(tmpDir, "main-repo")
	mainGromitDir := filepath.Join(mainRepoRoot, ".gromit")
	mainLogsDir := filepath.Join(mainGromitDir, "logs")
	if err := os.MkdirAll(mainLogsDir, 0o755); err != nil {
		t.Fatalf("creating main repo logs: %v", err)
	}

	// Create a worktree directory with .gromit but no logs
	// (simulating what git worktree add creates - .gromit exists but logs don't)
	worktreeRoot := filepath.Join(tmpDir, "worktree")
	worktreeGromitDir := filepath.Join(worktreeRoot, ".gromit")
	if err := os.MkdirAll(worktreeGromitDir, 0o755); err != nil {
		t.Fatalf("creating worktree .gromit: %v", err)
	}

	worktreeLogsPath := filepath.Join(worktreeGromitDir, "logs")

	// Before calling setupRetroWorktreeLogsSymlink, logs shouldn't exist in worktree
	if _, err := os.Stat(worktreeLogsPath); err == nil {
		t.Fatal("worktree logs should not exist initially")
	}

	// Call the setup function to create symlink
	// This function doesn't exist yet - we're testing the spec
	if err := setupRetroWorktreeLogsSymlink(worktreeGromitDir, mainGromitDir); err != nil {
		t.Fatalf("setupRetroWorktreeLogsSymlink failed: %v", err)
	}

	// After calling setupRetroWorktreeLogsSymlink, worktree logs should be accessible
	info, err := os.Stat(worktreeLogsPath)
	if err != nil {
		t.Fatalf("worktree logs not accessible after setup: %v", err)
	}

	// It should be a symlink or accessible directory pointing to main repo logs
	if !info.IsDir() {
		t.Fatalf("worktree logs path is not a directory: %v", info)
	}

	// Verify we can read from the symlinked logs directory
	if _, err := os.ReadDir(worktreeLogsPath); err != nil {
		t.Fatalf("cannot read from worktree logs: %v", err)
	}
}

func TestEnsureRetroWorktreeLogsSetup_CallsSetupWhenNeeded(t *testing.T) {
	// RED test: Verify that ensureRetroWorktreeLogsSetup is called to set up
	// the logs symlink when retro runs in a session worktree.

	tmpDir := t.TempDir()

	// Create main repo structure with logs
	mainGromitDir := filepath.Join(tmpDir, "main", ".gromit")
	mainLogsDir := filepath.Join(mainGromitDir, "logs")
	if err := os.MkdirAll(mainLogsDir, 0o755); err != nil {
		t.Fatalf("creating main repo logs: %v", err)
	}

	// Create worktree structure without logs
	worktreeGromitDir := filepath.Join(tmpDir, "worktree", ".gromit")
	if err := os.MkdirAll(worktreeGromitDir, 0o755); err != nil {
		t.Fatalf("creating worktree .gromit: %v", err)
	}

	// Call ensureRetroWorktreeLogsSetup to set up the symlink
	if err := ensureRetroWorktreeLogsSetup(worktreeGromitDir, mainGromitDir); err != nil {
		t.Fatalf("ensureRetroWorktreeLogsSetup failed: %v", err)
	}

	// Verify that worktree logs are now accessible
	worktreeLogsPath := filepath.Join(worktreeGromitDir, "logs")
	if _, err := os.Stat(worktreeLogsPath); err != nil {
		t.Fatalf("worktree logs not accessible after setup: %v", err)
	}
}

func TestPrepareRetroWorktreeWithMainRepoLogs_SymlinksLogsAndReturnsLogsPath(t *testing.T) {
	// RED test: Verify that when retro is set up in a worktree, it gets the
	// main repo's logs either via symlink or by resolving the path.

	tmpDir := t.TempDir()

	// Create main repo
	mainRepoRoot := filepath.Join(tmpDir, "main-repo")
	mainGromitDir := filepath.Join(mainRepoRoot, ".gromit")
	mainLogsDir := filepath.Join(mainGromitDir, "logs")
	if err := os.MkdirAll(mainLogsDir, 0o755); err != nil {
		t.Fatalf("creating main logs: %v", err)
	}

	// Create worktree
	worktreeRoot := filepath.Join(tmpDir, "worktree")
	worktreeGromitDir := filepath.Join(worktreeRoot, ".gromit")
	if err := os.MkdirAll(worktreeGromitDir, 0o755); err != nil {
		t.Fatalf("creating worktree .gromit: %v", err)
	}

	// Call prepareRetroWorktreeWithMainRepoLogs
	logsPath, err := prepareRetroWorktreeWithMainRepoLogs(worktreeGromitDir, mainGromitDir)
	if err != nil {
		t.Fatalf("prepareRetroWorktreeWithMainRepoLogs failed: %v", err)
	}

	// The returned path should point to accessible logs
	if logsPath == "" {
		t.Fatal("prepareRetroWorktreeWithMainRepoLogs returned empty logs path")
	}

	// The path should be accessible
	if _, err := os.Stat(logsPath); err != nil {
		t.Fatalf("returned logs path not accessible: %q: %v", logsPath, err)
	}

	// The worktree should have either a symlink or the logs should be accessible
	worktreeLogsPath := filepath.Join(worktreeGromitDir, "logs")
	if _, err := os.Stat(worktreeLogsPath); err == nil {
		// Either the symlink was created or logs exist directly
		return
	}

	// If no symlink, the returned path should still work
	if logsPath != mainLogsDir {
		t.Fatalf("expected logs path to be main repo logs or symlinked, got: %q", logsPath)
	}
}
