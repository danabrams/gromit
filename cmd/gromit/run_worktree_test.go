package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/worktree"
)

func TestRunInDedicatedWorktree_CreatesWorktreeAndRunsFn(t *testing.T) {
	origManager := runWorktreeNewManagerFn
	origCleanup := runWorktreeCleanupFn
	defer func() {
		runWorktreeNewManagerFn = origManager
		runWorktreeCleanupFn = origCleanup
	}()

	tmpDir := t.TempDir()
	worktreeDir := tmpDir + "-gromit-run-12345"
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(worktreeDir)

	var createdCommand string
	runWorktreeNewManagerFn = func(mainDir string) (runWorktreeManager, error) {
		return &fakeRunWorktreeManager{
			session: &worktree.SessionWorktree{
				BranchName:  "gromit/run-12345",
				WorktreeDir: worktreeDir,
			},
			onCreateCommand: func(cmd string) { createdCommand = cmd },
		}, nil
	}

	var cleanedUp bool
	var cleanedMainDir, cleanedWorktreeDir, cleanedBranch string
	runWorktreeCleanupFn = func(mainDir, wtDir, branch string) {
		cleanedUp = true
		cleanedMainDir = mainDir
		cleanedWorktreeDir = wtDir
		cleanedBranch = branch
	}

	var fnCwd string
	fnCalled := false
	err := runInDedicatedWorktree(context.Background(), tmpDir, func() error {
		fnCalled = true
		fnCwd, _ = os.Getwd()
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fnCalled {
		t.Fatal("fn was not called")
	}
	// Resolve symlinks for macOS /var -> /private/var
	resolvedCwd, _ := filepath.EvalSymlinks(fnCwd)
	resolvedWant, _ := filepath.EvalSymlinks(worktreeDir)
	if resolvedCwd != resolvedWant {
		t.Errorf("fn cwd = %q, want %q", resolvedCwd, resolvedWant)
	}
	if createdCommand != "run" {
		t.Errorf("created command = %q, want %q", createdCommand, "run")
	}
	if !cleanedUp {
		t.Fatal("cleanup was not called")
	}
	if cleanedMainDir != tmpDir {
		t.Errorf("cleanup mainDir = %q, want %q", cleanedMainDir, tmpDir)
	}
	if cleanedWorktreeDir != worktreeDir {
		t.Errorf("cleanup worktreeDir = %q, want %q", cleanedWorktreeDir, worktreeDir)
	}
	if cleanedBranch != "gromit/run-12345" {
		t.Errorf("cleanup branch = %q, want %q", cleanedBranch, "gromit/run-12345")
	}
}

func TestRunInDedicatedWorktree_CleansUpOnFnError(t *testing.T) {
	origManager := runWorktreeNewManagerFn
	origCleanup := runWorktreeCleanupFn
	defer func() {
		runWorktreeNewManagerFn = origManager
		runWorktreeCleanupFn = origCleanup
	}()

	tmpDir := t.TempDir()
	worktreeDir := tmpDir + "-gromit-run-99999"
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(worktreeDir)

	runWorktreeNewManagerFn = func(mainDir string) (runWorktreeManager, error) {
		return &fakeRunWorktreeManager{
			session: &worktree.SessionWorktree{
				BranchName:  "gromit/run-99999",
				WorktreeDir: worktreeDir,
			},
		}, nil
	}

	var cleanedUp bool
	runWorktreeCleanupFn = func(_, _, _ string) {
		cleanedUp = true
	}

	wantErr := fmt.Errorf("build failed")
	err := runInDedicatedWorktree(context.Background(), tmpDir, func() error {
		return wantErr
	})

	if err != wantErr {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}
	if !cleanedUp {
		t.Fatal("cleanup was not called on fn error")
	}
}

func TestRunInDedicatedWorktree_EmptyMainDir(t *testing.T) {
	err := runInDedicatedWorktree(context.Background(), "", func() error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for empty mainDir")
	}
}

func TestRunInDedicatedWorktree_ManagerCreationError(t *testing.T) {
	origManager := runWorktreeNewManagerFn
	defer func() { runWorktreeNewManagerFn = origManager }()

	runWorktreeNewManagerFn = func(mainDir string) (runWorktreeManager, error) {
		return nil, fmt.Errorf("git not found")
	}

	err := runInDedicatedWorktree(context.Background(), "/tmp/test", func() error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for manager creation failure")
	}
}

func TestRunInDedicatedWorktree_SessionCreationError(t *testing.T) {
	origManager := runWorktreeNewManagerFn
	defer func() { runWorktreeNewManagerFn = origManager }()

	runWorktreeNewManagerFn = func(mainDir string) (runWorktreeManager, error) {
		return &fakeRunWorktreeManager{
			createErr: fmt.Errorf("branch already exists"),
		}, nil
	}

	err := runInDedicatedWorktree(context.Background(), "/tmp/test", func() error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for session creation failure")
	}
}

func TestRunInDedicatedWorktree_BranchPrefix(t *testing.T) {
	origManager := runWorktreeNewManagerFn
	origCleanup := runWorktreeCleanupFn
	defer func() {
		runWorktreeNewManagerFn = origManager
		runWorktreeCleanupFn = origCleanup
	}()

	tmpDir := t.TempDir()
	worktreeDir := tmpDir + "-gromit-run-55555"
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(worktreeDir)

	var capturedBranch string
	runWorktreeNewManagerFn = func(mainDir string) (runWorktreeManager, error) {
		return &fakeRunWorktreeManager{
			session: &worktree.SessionWorktree{
				BranchName:  "gromit/run-55555",
				WorktreeDir: worktreeDir,
			},
			onCreateCommand: func(_ string) {},
		}, nil
	}
	runWorktreeCleanupFn = func(_, _, branch string) {
		capturedBranch = branch
	}

	_ = runInDedicatedWorktree(context.Background(), tmpDir, func() error {
		return nil
	})

	if capturedBranch == "" {
		t.Fatal("branch not captured")
	}
	const wantPrefix = "gromit/run-"
	if len(capturedBranch) < len(wantPrefix) || capturedBranch[:len(wantPrefix)] != wantPrefix {
		t.Errorf("branch %q does not have prefix %q", capturedBranch, wantPrefix)
	}
}

// fakeRunWorktreeManager is a test double for runWorktreeManager.
type fakeRunWorktreeManager struct {
	session         *worktree.SessionWorktree
	createErr       error
	onCreateCommand func(string)
}

func (f *fakeRunWorktreeManager) CreateSessionWorktree(_ context.Context, command string) (*worktree.SessionWorktree, error) {
	if f.onCreateCommand != nil {
		f.onCreateCommand(command)
	}
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.session, nil
}
