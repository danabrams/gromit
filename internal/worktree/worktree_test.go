package worktree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewManager_CreatesManagerWithCorrectFields verifies that NewManager
// creates a Manager with the correct MainDir and WorktreeDir fields.
// Expected failure: Manager type and NewManager function do not exist yet.
func TestNewManager_CreatesManagerWithCorrectFields(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	// Expected failure: NewManager function does not exist
	m, err := NewManager(mainDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v, want nil", err)
	}
	if m == nil {
		t.Fatal("NewManager() returned nil manager")
	}

	// Expected failure: Manager.MainDir field does not exist
	if m.MainDir != mainDir {
		t.Errorf("Manager.MainDir = %q, want %q", m.MainDir, mainDir)
	}

	// WorktreeDir should be a sibling directory with "-gromit-interactive" suffix
	// Expected failure: Manager.WorktreeDir field does not exist
	expectedWorktreeDir := mainDir + "-gromit-interactive"
	if m.WorktreeDir != expectedWorktreeDir {
		t.Errorf("Manager.WorktreeDir = %q, want %q", m.WorktreeDir, expectedWorktreeDir)
	}
}

// TestNewManager_NilGitRunFn verifies that NewManager can create a manager
// without a gitRunFn (defaults to running real git commands).
// Expected failure: Manager type and NewManager function do not exist yet.
func TestNewManager_NilGitRunFn(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	// Expected failure: NewManager function does not exist
	m, err := NewManager(mainDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v, want nil", err)
	}

	// Expected failure: Manager.gitRunFn field does not exist
	// When gitRunFn is nil, methods should still work (they'll call real git)
	if m.gitRunFn == nil {
		// This is expected - nil means "use real git"
	}
}

// TestNewManager_WithGitRunFn verifies that NewManager correctly stores
// a provided gitRunFn for testing.
// Expected failure: Manager type, NewManager function, and WithGitRunFn option do not exist yet.
func TestNewManager_WithGitRunFn(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		return "", nil
	}

	// Expected failure: NewManager function and WithGitRunFn option do not exist
	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v, want nil", err)
	}

	// Expected failure: Manager.gitRunFn field does not exist
	if m.gitRunFn == nil {
		t.Error("expected gitRunFn to be set, got nil")
	}
}

// TestEnsureWorktree_CreatesWorktreeWhenMissing verifies that EnsureWorktree
// creates a new worktree when it doesn't exist.
// Expected failure: Manager.EnsureWorktree method does not exist yet.
func TestEnsureWorktree_CreatesWorktreeWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		// Simulate successful git worktree add
		if args[0] == "worktree" && args[1] == "add" {
			worktreePath := args[2]
			if err := os.MkdirAll(worktreePath, 0755); err != nil {
				return "", err
			}
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Expected failure: EnsureWorktree method does not exist
	worktreePath, err := m.EnsureWorktree()
	if err != nil {
		t.Fatalf("EnsureWorktree() error = %v, want nil", err)
	}

	expectedPath := mainDir + "-gromit-interactive"
	if worktreePath != expectedPath {
		t.Errorf("EnsureWorktree() returned path %q, want %q", worktreePath, expectedPath)
	}

	// Verify git worktree add was called
	foundWorktreeAdd := false
	for _, call := range gitCalls {
		if strings.Contains(call, "worktree add") {
			foundWorktreeAdd = true
			break
		}
	}
	if !foundWorktreeAdd {
		t.Errorf("expected 'git worktree add' to be called, got calls: %v", gitCalls)
	}
}

// TestEnsureWorktree_ReusesExistingWorktree verifies that EnsureWorktree
// returns the path without creating a new worktree when one already exists.
// Expected failure: Manager.EnsureWorktree method does not exist yet.
func TestEnsureWorktree_ReusesExistingWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	worktreeDir := mainDir + "-gromit-interactive"

	// Create both directories
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		// Simulate worktree already exists
		if args[0] == "worktree" && args[1] == "list" {
			return worktreeDir, nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Expected failure: EnsureWorktree method does not exist
	worktreePath, err := m.EnsureWorktree()
	if err != nil {
		t.Fatalf("EnsureWorktree() error = %v, want nil", err)
	}

	if worktreePath != worktreeDir {
		t.Errorf("EnsureWorktree() returned path %q, want %q", worktreePath, worktreeDir)
	}

	// Verify git worktree add was NOT called (worktree already exists)
	for _, call := range gitCalls {
		if strings.Contains(call, "worktree add") {
			t.Errorf("should not call 'git worktree add' when worktree exists, got: %v", gitCalls)
		}
	}
}

// TestCreateBranch_GeneratesCorrectBranchName verifies that CreateBranch
// generates a branch name with the format gromit/<command>-<timestamp>.
// Expected failure: Manager.CreateBranch method does not exist yet.
func TestCreateBranch_GeneratesCorrectBranchName(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Expected failure: CreateBranch method does not exist
	branchName, err := m.CreateBranch("retro")
	if err != nil {
		t.Fatalf("CreateBranch() error = %v, want nil", err)
	}

	// Verify branch name format: gromit/retro-<timestamp>
	if !strings.HasPrefix(branchName, "gromit/retro-") {
		t.Errorf("CreateBranch() returned %q, want prefix 'gromit/retro-'", branchName)
	}

	// Verify git branch command was called
	foundBranchCreate := false
	for _, call := range gitCalls {
		if strings.Contains(call, "branch") || strings.Contains(call, "checkout") {
			foundBranchCreate = true
			break
		}
	}
	if !foundBranchCreate {
		t.Errorf("expected git branch/checkout command, got calls: %v", gitCalls)
	}
}

// TestCreateBranch_DifferentCommands verifies that CreateBranch handles
// different command names correctly.
// Expected failure: Manager.CreateBranch method does not exist yet.
func TestCreateBranch_DifferentCommands(t *testing.T) {
	tests := []struct {
		command    string
		wantPrefix string
	}{
		{command: "retro", wantPrefix: "gromit/retro-"},
		{command: "review", wantPrefix: "gromit/review-"},
		{command: "test", wantPrefix: "gromit/test-"},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			tmpDir := t.TempDir()
			mainDir := filepath.Join(tmpDir, "myproject")
			if err := os.MkdirAll(mainDir, 0755); err != nil {
				t.Fatalf("failed to create main dir: %v", err)
			}

			mockGitRun := func(dir string, args ...string) (string, error) {
				return "", nil
			}

			m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}

			// Expected failure: CreateBranch method does not exist
			branchName, err := m.CreateBranch(tt.command)
			if err != nil {
				t.Fatalf("CreateBranch(%q) error = %v, want nil", tt.command, err)
			}

			if !strings.HasPrefix(branchName, tt.wantPrefix) {
				t.Errorf("CreateBranch(%q) = %q, want prefix %q", tt.command, branchName, tt.wantPrefix)
			}
		})
	}
}

// TestCleanup_RemovesWorktree verifies that Cleanup removes the worktree
// directory and cleans up git worktree state.
// Expected failure: Manager.Cleanup method does not exist yet.
func TestCleanup_RemovesWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	worktreeDir := mainDir + "-gromit-interactive"

	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		// Simulate successful worktree removal
		if args[0] == "worktree" && args[1] == "remove" {
			// Remove the directory to simulate git's behavior
			return "", os.RemoveAll(args[2])
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Expected failure: Cleanup method does not exist
	err = m.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v, want nil", err)
	}

	// Verify git worktree remove was called
	foundWorktreeRemove := false
	for _, call := range gitCalls {
		if strings.Contains(call, "worktree remove") {
			foundWorktreeRemove = true
			break
		}
	}
	if !foundWorktreeRemove {
		t.Errorf("expected 'git worktree remove' to be called, got calls: %v", gitCalls)
	}

	// Verify directory was actually removed
	if _, err := os.Stat(worktreeDir); !os.IsNotExist(err) {
		t.Errorf("expected worktree directory to be removed, but it still exists")
	}
}

// TestCleanup_HandlesNonexistentWorktree verifies that Cleanup doesn't fail
// when called on a worktree that doesn't exist.
// Expected failure: Manager.Cleanup method does not exist yet.
func TestCleanup_HandlesNonexistentWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	mockGitRun := func(dir string, args ...string) (string, error) {
		// Simulate worktree doesn't exist
		if args[0] == "worktree" && args[1] == "remove" {
			return "", errors.New("worktree not found")
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Expected failure: Cleanup method does not exist
	// Cleanup should succeed even if worktree doesn't exist
	err = m.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error = %v, want nil (should handle nonexistent worktree)", err)
	}
}

// TestManager_NilReceiver verifies that Manager methods handle nil receiver safely.
// Expected failure: Manager type and its methods do not exist yet.
func TestManager_NilReceiver(t *testing.T) {
	var m *Manager

	// Expected failure: EnsureWorktree method does not exist
	_, err := m.EnsureWorktree()
	if err == nil {
		t.Error("EnsureWorktree() on nil receiver should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "nil") {
		t.Errorf("EnsureWorktree() error should mention nil receiver, got: %v", err)
	}

	// Expected failure: CreateBranch method does not exist
	_, err = m.CreateBranch("test")
	if err == nil {
		t.Error("CreateBranch() on nil receiver should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "nil") {
		t.Errorf("CreateBranch() error should mention nil receiver, got: %v", err)
	}

	// Expected failure: Cleanup method does not exist
	err = m.Cleanup()
	if err == nil {
		t.Error("Cleanup() on nil receiver should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "nil") {
		t.Errorf("Cleanup() error should mention nil receiver, got: %v", err)
	}
}

// TestManager_EmptyMainDir verifies that NewManager rejects empty MainDir.
// Expected failure: NewManager function does not exist yet.
func TestManager_EmptyMainDir(t *testing.T) {
	// Expected failure: NewManager function does not exist
	_, err := NewManager("")
	if err == nil {
		t.Error("NewManager(\"\") should return error for empty MainDir")
	}
	if err != nil && !strings.Contains(err.Error(), "dir") && !strings.Contains(err.Error(), "empty") {
		t.Errorf("NewManager(\"\") error should mention empty/missing directory, got: %v", err)
	}
}

// TestEnsureWorktree_GitRunFnCalledWithCorrectDir verifies that EnsureWorktree
// calls gitRunFn with the correct working directory.
// Expected failure: Manager.EnsureWorktree method does not exist yet.
func TestEnsureWorktree_GitRunFnCalledWithCorrectDir(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	var capturedDir string
	mockGitRun := func(dir string, args ...string) (string, error) {
		capturedDir = dir
		// Simulate worktree doesn't exist (list returns empty)
		if args[0] == "worktree" && args[1] == "list" {
			return "", nil
		}
		// Simulate successful worktree add
		if args[0] == "worktree" && args[1] == "add" {
			worktreePath := args[2]
			return "", os.MkdirAll(worktreePath, 0755)
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Expected failure: EnsureWorktree method does not exist
	_, err = m.EnsureWorktree()
	if err != nil {
		t.Fatalf("EnsureWorktree() error = %v, want nil", err)
	}

	// Verify gitRunFn was called with mainDir as working directory
	if capturedDir != mainDir {
		t.Errorf("gitRunFn called with dir %q, want %q", capturedDir, mainDir)
	}
}

// TestCreateBranch_GitRunFnCalledInWorktreeDir verifies that CreateBranch
// executes git commands in the worktree directory, not the main directory.
// Expected failure: Manager.CreateBranch method does not exist yet.
func TestCreateBranch_GitRunFnCalledInWorktreeDir(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	worktreeDir := mainDir + "-gromit-interactive"

	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	var capturedDir string
	mockGitRun := func(dir string, args ...string) (string, error) {
		capturedDir = dir
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Expected failure: CreateBranch method does not exist
	_, err = m.CreateBranch("retro")
	if err != nil {
		t.Fatalf("CreateBranch() error = %v, want nil", err)
	}

	// CreateBranch should run git commands in the worktree directory
	if capturedDir != worktreeDir {
		t.Errorf("CreateBranch() called gitRunFn with dir %q, want %q", capturedDir, worktreeDir)
	}
}

// TestCleanup_GitRunFnCalledWithCorrectDir verifies that Cleanup calls
// gitRunFn with the correct working directory.
// Expected failure: Manager.Cleanup method does not exist yet.
func TestCleanup_GitRunFnCalledWithCorrectDir(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	worktreeDir := mainDir + "-gromit-interactive"

	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	var capturedDir string
	mockGitRun := func(dir string, args ...string) (string, error) {
		capturedDir = dir
		if args[0] == "worktree" && args[1] == "remove" {
			return "", os.RemoveAll(args[2])
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Expected failure: Cleanup method does not exist
	err = m.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v, want nil", err)
	}

	// Cleanup should run git commands in the main directory
	if capturedDir != mainDir {
		t.Errorf("Cleanup() called gitRunFn with dir %q, want %q", capturedDir, mainDir)
	}
}

// TestManager_FieldsAreAccessible verifies that Manager fields can be read
// after creation (for inspection/debugging).
// Expected failure: Manager type and its fields do not exist yet.
func TestManager_FieldsAreAccessible(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	// Expected failure: NewManager function does not exist
	m, err := NewManager(mainDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Fields should be accessible for reading
	// Expected failure: Manager.MainDir field does not exist
	if m.MainDir == "" {
		t.Error("Manager.MainDir should not be empty")
	}

	// Expected failure: Manager.WorktreeDir field does not exist
	if m.WorktreeDir == "" {
		t.Error("Manager.WorktreeDir should not be empty")
	}

	// The gitRunFn field is private and shouldn't be directly accessible
	// (This test just documents that MainDir and WorktreeDir are public)
}

// TestWithGitRunFn_IsOptionFunction verifies that WithGitRunFn returns
// a valid option function that can be passed to NewManager.
// Expected failure: WithGitRunFn function does not exist yet.
func TestWithGitRunFn_IsOptionFunction(t *testing.T) {
	mockGitRun := func(dir string, args ...string) (string, error) {
		return "", nil
	}

	// Expected failure: WithGitRunFn function does not exist
	opt := WithGitRunFn(mockGitRun)
	if opt == nil {
		t.Fatal("WithGitRunFn() returned nil option")
	}

	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	// Should be able to pass the option to NewManager
	// Expected failure: NewManager function does not exist
	_, err := NewManager(mainDir, opt)
	if err != nil {
		t.Errorf("NewManager() with WithGitRunFn option error = %v, want nil", err)
	}
}

// helperMockGitRun is a reusable mock gitRunFn for tests that need
// simple git command simulation.
func helperMockGitRun(t *testing.T, worktreePath string) func(string, ...string) (string, error) {
	t.Helper()
	return func(dir string, args ...string) (string, error) {
		switch {
		case len(args) >= 2 && args[0] == "worktree" && args[1] == "list":
			// Return empty to simulate no worktree exists
			return "", nil
		case len(args) >= 3 && args[0] == "worktree" && args[1] == "add":
			// Create the worktree directory
			return "", os.MkdirAll(worktreePath, 0755)
		case len(args) >= 3 && args[0] == "worktree" && args[1] == "remove":
			// Remove the worktree directory
			return "", os.RemoveAll(worktreePath)
		case args[0] == "checkout" && args[1] == "-b":
			// Branch creation
			return "", nil
		default:
			return "", fmt.Errorf("unexpected git command: %v", args)
		}
	}
}

// TestEnsureWorktree_UsesHelperMock demonstrates the helper function usage.
// Expected failure: Manager.EnsureWorktree method does not exist yet.
func TestEnsureWorktree_UsesHelperMock(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	worktreeDir := mainDir + "-gromit-interactive"

	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	m, err := NewManager(mainDir, WithGitRunFn(helperMockGitRun(t, worktreeDir)))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Expected failure: EnsureWorktree method does not exist
	path, err := m.EnsureWorktree()
	if err != nil {
		t.Fatalf("EnsureWorktree() error = %v, want nil", err)
	}

	if path != worktreeDir {
		t.Errorf("EnsureWorktree() = %q, want %q", path, worktreeDir)
	}

	// Verify directory was created
	if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
		t.Error("expected worktree directory to be created")
	}
}
