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
func TestNewManager_CreatesManagerWithCorrectFields(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	m, err := NewManager(mainDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v, want nil", err)
	}
	if m == nil {
		t.Fatal("NewManager() returned nil manager")
	}

	if m.MainDir != mainDir {
		t.Errorf("Manager.MainDir = %q, want %q", m.MainDir, mainDir)
	}

	// WorktreeDir should be a sibling directory with "-gromit-interactive" suffix
	expectedWorktreeDir := mainDir + "-gromit-interactive"
	if m.WorktreeDir != expectedWorktreeDir {
		t.Errorf("Manager.WorktreeDir = %q, want %q", m.WorktreeDir, expectedWorktreeDir)
	}
}

// TestNewManager_NilGitRunFn verifies that NewManager can create a manager
// without a gitRunFn (defaults to running real git commands).
func TestNewManager_NilGitRunFn(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	m, err := NewManager(mainDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v, want nil", err)
	}

	// When gitRunFn is nil, methods should still work (they'll call real git)
	if m.gitRunFn == nil {
		// This is expected - nil means "use real git"
	}
}

// TestNewManager_WithGitRunFn verifies that NewManager correctly stores
// a provided gitRunFn for testing.
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

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v, want nil", err)
	}

	if m.gitRunFn == nil {
		t.Error("expected gitRunFn to be set, got nil")
	}
}

// TestEnsureWorktree_CreatesWorktreeWhenMissing verifies that EnsureWorktree
// creates a new worktree when it doesn't exist.
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

	// Cleanup should succeed even if worktree doesn't exist
	err = m.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error = %v, want nil (should handle nonexistent worktree)", err)
	}
}

// TestManager_NilReceiver verifies that Manager methods handle nil receiver safely.
func TestManager_NilReceiver(t *testing.T) {
	var m *Manager

	_, err := m.EnsureWorktree()
	if err == nil {
		t.Error("EnsureWorktree() on nil receiver should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "nil") {
		t.Errorf("EnsureWorktree() error should mention nil receiver, got: %v", err)
	}

	_, err = m.CreateBranch("test")
	if err == nil {
		t.Error("CreateBranch() on nil receiver should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "nil") {
		t.Errorf("CreateBranch() error should mention nil receiver, got: %v", err)
	}

	err = m.Cleanup()
	if err == nil {
		t.Error("Cleanup() on nil receiver should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "nil") {
		t.Errorf("Cleanup() error should mention nil receiver, got: %v", err)
	}
}

// TestManager_EmptyMainDir verifies that NewManager rejects empty MainDir.
func TestManager_EmptyMainDir(t *testing.T) {
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
func TestManager_FieldsAreAccessible(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	m, err := NewManager(mainDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Fields should be accessible for reading
	if m.MainDir == "" {
		t.Error("Manager.MainDir should not be empty")
	}

	if m.WorktreeDir == "" {
		t.Error("Manager.WorktreeDir should not be empty")
	}

	// The gitRunFn field is private and shouldn't be directly accessible
	// (This test just documents that MainDir and WorktreeDir are public)
}

// TestWithGitRunFn_IsOptionFunction verifies that WithGitRunFn returns
// a valid option function that can be passed to NewManager.
func TestWithGitRunFn_IsOptionFunction(t *testing.T) {
	mockGitRun := func(dir string, args ...string) (string, error) {
		return "", nil
	}

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

// TestPendingBranches_ReturnsEmptyWhenNoBranches verifies that PendingBranches
// returns an empty slice when no gromit/* branches exist.
func TestPendingBranches_ReturnsEmptyWhenNoBranches(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	mockGitRun := func(dir string, args ...string) (string, error) {
		// Simulate git for-each-ref returning no branches
		if args[0] == "for-each-ref" {
			return "", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	branches, err := m.PendingBranches()
	if err != nil {
		t.Fatalf("PendingBranches() error = %v, want nil", err)
	}

	if len(branches) != 0 {
		t.Errorf("PendingBranches() returned %d branches, want 0", len(branches))
	}
}

// TestPendingBranches_ReturnsGromitBranches verifies that PendingBranches
// returns only branches matching the gromit/* pattern.
func TestPendingBranches_ReturnsGromitBranches(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	mockGitRun := func(dir string, args ...string) (string, error) {
		// Simulate git for-each-ref returning multiple branches
		if args[0] == "for-each-ref" {
			return "refs/heads/gromit/retro-1234567890\nrefs/heads/gromit/review-9876543210\nrefs/heads/main\nrefs/heads/feature-branch\n", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	branches, err := m.PendingBranches()
	if err != nil {
		t.Fatalf("PendingBranches() error = %v, want nil", err)
	}

	// Should only return gromit/* branches, not main or feature-branch
	expectedBranches := []string{"gromit/retro-1234567890", "gromit/review-9876543210"}
	if len(branches) != len(expectedBranches) {
		t.Fatalf("PendingBranches() returned %d branches, want %d: %v", len(branches), len(expectedBranches), branches)
	}

	for i, branch := range branches {
		if branch != expectedBranches[i] {
			t.Errorf("PendingBranches()[%d] = %q, want %q", i, branch, expectedBranches[i])
		}
	}
}

// TestPendingBranches_FiltersNonGromitBranches verifies that PendingBranches
// excludes branches that don't match the gromit/* pattern.
func TestPendingBranches_FiltersNonGromitBranches(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	mockGitRun := func(dir string, args ...string) (string, error) {
		if args[0] == "for-each-ref" {
			// Mix of gromit and non-gromit branches
			return "refs/heads/main\nrefs/heads/develop\nrefs/heads/gromit/retro-123\nrefs/heads/feature/new-ui\nrefs/heads/gromit/review-456\n", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	branches, err := m.PendingBranches()
	if err != nil {
		t.Fatalf("PendingBranches() error = %v, want nil", err)
	}

	// Should only return the two gromit/* branches
	if len(branches) != 2 {
		t.Errorf("PendingBranches() returned %d branches, want 2: %v", len(branches), branches)
	}

	for _, branch := range branches {
		if !strings.HasPrefix(branch, "gromit/") {
			t.Errorf("PendingBranches() returned non-gromit branch: %q", branch)
		}
	}
}

func TestPendingBranches_ExcludesInteractiveControlBranch(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	mockGitRun := func(dir string, args ...string) (string, error) {
		if args[0] == "for-each-ref" {
			return "refs/heads/gromit/interactive\nrefs/heads/gromit/review-123\n", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	branches, err := m.PendingBranches()
	if err != nil {
		t.Fatalf("PendingBranches() error = %v", err)
	}
	if len(branches) != 1 {
		t.Fatalf("PendingBranches() returned %d branches, want 1: %v", len(branches), branches)
	}
	if branches[0] != "gromit/review-123" {
		t.Fatalf("PendingBranches()[0] = %q, want %q", branches[0], "gromit/review-123")
	}
}

func TestPendingBranches_ExcludesBranchesCheckedOutInWorktrees(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	mockGitRun := func(dir string, args ...string) (string, error) {
		if args[0] == "worktree" && len(args) >= 3 && args[1] == "list" && args[2] == "--porcelain" {
			return "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\nworktree /repo-gromit-review\nHEAD def\nbranch refs/heads/gromit/review-123\n", nil
		}
		if args[0] == "for-each-ref" {
			return "refs/heads/gromit/review-123\nrefs/heads/gromit/debug-456\n", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	branches, err := m.PendingBranches()
	if err != nil {
		t.Fatalf("PendingBranches() error = %v", err)
	}
	if len(branches) != 1 {
		t.Fatalf("PendingBranches() returned %d branches, want 1: %v", len(branches), branches)
	}
	if branches[0] != "gromit/debug-456" {
		t.Fatalf("PendingBranches()[0] = %q, want %q", branches[0], "gromit/debug-456")
	}
}

func TestPendingBranches_WorktreeProbeFailureFallsBackToBranchList(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	mockGitRun := func(dir string, args ...string) (string, error) {
		if args[0] == "worktree" && len(args) >= 3 && args[1] == "list" && args[2] == "--porcelain" {
			return "", errors.New("worktree listing failed")
		}
		if args[0] == "for-each-ref" {
			return "refs/heads/gromit/review-123\n", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	branches, err := m.PendingBranches()
	if err != nil {
		t.Fatalf("PendingBranches() error = %v", err)
	}
	if len(branches) != 1 {
		t.Fatalf("PendingBranches() returned %d branches, want 1: %v", len(branches), branches)
	}
	if branches[0] != "gromit/review-123" {
		t.Fatalf("PendingBranches()[0] = %q, want %q", branches[0], "gromit/review-123")
	}
}

// TestMergeBack_FastForwardSuccess verifies that MergeBack performs a
// fast-forward merge when possible and deletes the branch on success.
func TestMergeBack_FastForwardSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		// Simulate successful fast-forward merge
		if args[0] == "merge" && contains(args, "--ff-only") {
			return "Fast-forward merge successful", nil
		}
		// Simulate successful branch deletion
		if args[0] == "branch" && args[1] == "-d" {
			return "Deleted branch", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/retro-1234567890")
	if err != nil {
		t.Fatalf("MergeBack() error = %v, want nil", err)
	}

	// Verify git merge --ff-only was called
	foundFFMerge := false
	for _, call := range gitCalls {
		if strings.Contains(call, "merge") && strings.Contains(call, "--ff-only") {
			foundFFMerge = true
			break
		}
	}
	if !foundFFMerge {
		t.Errorf("expected 'git merge --ff-only' to be called, got calls: %v", gitCalls)
	}

	// Verify branch was deleted
	foundBranchDelete := false
	for _, call := range gitCalls {
		if strings.Contains(call, "branch -d") {
			foundBranchDelete = true
			break
		}
	}
	if !foundBranchDelete {
		t.Errorf("expected 'git branch -d' to be called, got calls: %v", gitCalls)
	}
}

// TestMergeBack_FallbackToMergeCommit verifies that MergeBack falls back
// to a regular merge commit when fast-forward merge fails.
func TestMergeBack_FallbackToMergeCommit(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		// Fast-forward merge fails (not possible)
		if args[0] == "merge" && contains(args, "--ff-only") {
			return "", errors.New("fatal: Not possible to fast-forward, aborting")
		}
		// Regular merge succeeds
		if args[0] == "merge" && !contains(args, "--ff-only") {
			return "Merge made by the 'recursive' strategy", nil
		}
		// Branch deletion succeeds
		if args[0] == "branch" && args[1] == "-d" {
			return "Deleted branch", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/review-1234567890")
	if err != nil {
		t.Fatalf("MergeBack() error = %v, want nil (should succeed with merge commit)", err)
	}

	// Verify both merge attempts were made
	foundFFAttempt := false
	foundRegularMerge := false
	for _, call := range gitCalls {
		if strings.Contains(call, "merge") && strings.Contains(call, "--ff-only") {
			foundFFAttempt = true
		}
		if strings.Contains(call, "merge") && !strings.Contains(call, "--ff-only") {
			foundRegularMerge = true
		}
	}
	if !foundFFAttempt {
		t.Errorf("expected fast-forward merge attempt, got calls: %v", gitCalls)
	}
	if !foundRegularMerge {
		t.Errorf("expected regular merge fallback, got calls: %v", gitCalls)
	}

	// Verify branch was deleted after successful merge
	foundBranchDelete := false
	for _, call := range gitCalls {
		if strings.Contains(call, "branch -d") {
			foundBranchDelete = true
			break
		}
	}
	if !foundBranchDelete {
		t.Errorf("expected branch deletion after successful merge, got calls: %v", gitCalls)
	}
}

// TestMergeBack_ConflictReturnsError verifies that MergeBack returns an error
// when a merge conflict occurs and does NOT delete the branch.
func TestMergeBack_ConflictReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		// Simulate abort succeeds
		if args[0] == "merge" && args[1] == "--abort" {
			return "Merge aborted", nil
		}
		// Fast-forward merge fails
		if args[0] == "merge" && contains(args, "--ff-only") {
			return "", errors.New("fatal: Not possible to fast-forward")
		}
		// Regular merge also fails with conflict
		if args[0] == "merge" && !contains(args, "--ff-only") {
			return "", errors.New("CONFLICT (content): Merge conflict in file.txt")
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/review-1234567890")
	if err == nil {
		t.Fatal("MergeBack() should return error on merge conflict, got nil")
	}

	// Error message should indicate conflict
	if !strings.Contains(err.Error(), "conflict") && !strings.Contains(err.Error(), "CONFLICT") && !strings.Contains(err.Error(), "merge") {
		t.Errorf("MergeBack() error should mention conflict, got: %v", err)
	}

	// Verify merge --abort was called
	foundAbort := false
	for _, call := range gitCalls {
		if strings.Contains(call, "merge --abort") {
			foundAbort = true
			break
		}
	}
	if !foundAbort {
		t.Errorf("expected 'git merge --abort' after conflict, got calls: %v", gitCalls)
	}

	// Verify branch was NOT deleted (conflict not resolved)
	for _, call := range gitCalls {
		if strings.Contains(call, "branch -d") {
			t.Errorf("should NOT delete branch after merge conflict, got calls: %v", gitCalls)
		}
	}
	for _, call := range gitCalls {
		if strings.Contains(call, "worktree remove") {
			t.Errorf("should NOT remove derived worktree after merge conflict, got calls: %v", gitCalls)
		}
	}
}

func TestMergeBack_ConflictDetectedFromOutputWithGenericError(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		if args[0] == "merge" && args[1] == "--abort" {
			return "", nil
		}
		if args[0] == "merge" && contains(args, "--ff-only") {
			return "", errors.New("fatal: Not possible to fast-forward")
		}
		if args[0] == "merge" && !contains(args, "--ff-only") {
			return "Auto-merging file.txt\nCONFLICT (content): Merge conflict in file.txt\nAutomatic merge failed; fix conflicts and then commit the result.\n", errors.New("exit status 1")
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/review-1234567890")
	if err == nil {
		t.Fatal("MergeBack() should return error on merge conflict, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "conflict") {
		t.Fatalf("MergeBack() error should indicate conflict, got: %v", err)
	}

	foundAbort := false
	for _, call := range gitCalls {
		if strings.Contains(call, "merge --abort") {
			foundAbort = true
		}
		if strings.Contains(call, "branch -d") {
			t.Errorf("should NOT delete branch after merge conflict, got calls: %v", gitCalls)
		}
	}
	if !foundAbort {
		t.Fatalf("expected 'git merge --abort' after conflict, got calls: %v", gitCalls)
	}
}

func TestMergeBack_ConflictDetectedFromAutomaticMergeFailedWithoutSemicolon(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		if args[0] == "merge" && args[1] == "--abort" {
			return "", nil
		}
		if args[0] == "merge" && contains(args, "--ff-only") {
			return "", errors.New("fatal: Not possible to fast-forward")
		}
		if args[0] == "merge" && !contains(args, "--ff-only") {
			return "Auto-merging file.txt\nAutomatic merge failed. Fix conflicts and then commit the result.\n", errors.New("exit status 1")
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/review-1234567890")
	if err == nil {
		t.Fatal("MergeBack() should return error on merge conflict, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "conflict") {
		t.Fatalf("MergeBack() error should indicate conflict, got: %v", err)
	}

	foundAbort := false
	for _, call := range gitCalls {
		if strings.Contains(call, "merge --abort") {
			foundAbort = true
		}
		if strings.Contains(call, "branch -d") {
			t.Errorf("should NOT delete branch after merge conflict, got calls: %v", gitCalls)
		}
	}
	if !foundAbort {
		t.Fatalf("expected 'git merge --abort' after conflict, got calls: %v", gitCalls)
	}
}

// TestMergeBack_NonConflictMergeFailureReturnsError verifies that MergeBack
// reports non-conflict merge failures without attempting merge --abort.
func TestMergeBack_NonConflictMergeFailureReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		if args[0] == "merge" && contains(args, "--ff-only") {
			return "", errors.New("fatal: Not possible to fast-forward")
		}
		if args[0] == "merge" && !contains(args, "--ff-only") {
			return "", errors.New("merge: gromit/review-1234567890 - not something we can merge")
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/review-1234567890")
	if err == nil {
		t.Fatal("MergeBack() should return error on non-conflict merge failure, got nil")
	}
	if strings.Contains(strings.ToLower(err.Error()), "conflict") {
		t.Errorf("MergeBack() error should not be labeled as conflict, got: %v", err)
	}

	for _, call := range gitCalls {
		if strings.Contains(call, "merge --abort") {
			t.Errorf("should NOT run 'git merge --abort' for non-conflict merge failure, got calls: %v", gitCalls)
		}
		if strings.Contains(call, "branch -d") {
			t.Errorf("should NOT delete branch after merge failure, got calls: %v", gitCalls)
		}
	}
}

// TestMergeBack_MergeStatePresentClassifiesAsConflict verifies that
// MergeBack classifies failures with active merge state as conflict.
func TestMergeBack_MergeStatePresentClassifiesAsConflict(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	gitCalls := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		if args[0] == "merge" && contains(args, "--ff-only") {
			return "", errors.New("fatal: Not possible to fast-forward")
		}
		if args[0] == "merge" && !contains(args, "--ff-only") && args[1] != "--abort" {
			return "", errors.New("merge: gromit/review-1234567890 - not something we can merge")
		}
		if args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "MERGE_HEAD" {
			return "deadbeef", nil
		}
		if args[0] == "merge" && args[1] == "--abort" {
			return "", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/review-1234567890")
	if err == nil {
		t.Fatal("MergeBack() should return error on merge failure, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "merge conflict") {
		t.Fatalf("MergeBack() should classify active merge state as conflict, got: %v", err)
	}

	foundAbort := false
	for _, call := range gitCalls {
		if strings.Contains(call, "merge --abort") {
			foundAbort = true
		}
		if strings.Contains(call, "branch -d") {
			t.Errorf("should NOT delete branch after merge failure, got calls: %v", gitCalls)
		}
	}
	if !foundAbort {
		t.Fatalf("expected 'git merge --abort' when merge state is present, got calls: %v", gitCalls)
	}
}

// TestMergeBack_NonConflictErrorContainingConflictInBranchName verifies that
// non-conflict errors are not misclassified when branch names include "conflict".
func TestMergeBack_NonConflictErrorContainingConflictInBranchName(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	gitCalls := []string{}
	branch := "gromit/review-conflict-1234567890"
	mockGitRun := func(dir string, args ...string) (string, error) {
		gitCalls = append(gitCalls, strings.Join(args, " "))
		if args[0] == "merge" && contains(args, "--ff-only") {
			return "", errors.New("fatal: Not possible to fast-forward")
		}
		if args[0] == "merge" && !contains(args, "--ff-only") {
			return "", fmt.Errorf("merge: %s - not something we can merge", branch)
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack(branch)
	if err == nil {
		t.Fatal("MergeBack() should return error on non-conflict merge failure, got nil")
	}
	if strings.Contains(strings.ToLower(err.Error()), "merge conflict") {
		t.Fatalf("MergeBack() should not label branch-name substring as conflict, got: %v", err)
	}

	for _, call := range gitCalls {
		if strings.Contains(call, "merge --abort") {
			t.Errorf("should NOT run 'git merge --abort' for non-conflict merge failure, got calls: %v", gitCalls)
		}
		if strings.Contains(call, "branch -d") {
			t.Errorf("should NOT delete branch after merge failure, got calls: %v", gitCalls)
		}
	}
}

// TestMergeBack_InvalidBranchName verifies that MergeBack returns an error
// for invalid branch names (empty string).
func TestMergeBack_InvalidBranchName(t *testing.T) {
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

	err = m.MergeBack("")
	if err == nil {
		t.Error("MergeBack(\"\") should return error for empty branch name")
	}
}

// TestMergeBack_NilReceiver verifies that MergeBack handles nil receiver safely.
func TestMergeBack_NilReceiver(t *testing.T) {
	var m *Manager

	err := m.MergeBack("gromit/retro-123")
	if err == nil {
		t.Error("MergeBack() on nil receiver should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "nil") {
		t.Errorf("MergeBack() error should mention nil receiver, got: %v", err)
	}
}

// TestPendingBranches_NilReceiver verifies that PendingBranches handles nil receiver safely.
func TestPendingBranches_NilReceiver(t *testing.T) {
	var m *Manager

	_, err := m.PendingBranches()
	if err == nil {
		t.Error("PendingBranches() on nil receiver should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "nil") {
		t.Errorf("PendingBranches() error should mention nil receiver, got: %v", err)
	}
}

// TestMergeBack_GitRunFnCalledInMainDir verifies that MergeBack executes
// git commands in the main directory, not the worktree directory.
func TestMergeBack_GitRunFnCalledInMainDir(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	capturedDirs := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		capturedDirs = append(capturedDirs, dir)
		// Simulate successful fast-forward merge
		if args[0] == "merge" {
			return "Fast-forward", nil
		}
		// Simulate successful branch deletion
		if args[0] == "branch" && args[1] == "-d" {
			return "Deleted", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/retro-123")
	if err != nil {
		t.Fatalf("MergeBack() error = %v, want nil", err)
	}

	// All git commands should run in the main directory
	for i, dir := range capturedDirs {
		if dir != mainDir {
			t.Errorf("git command %d called with dir %q, want %q", i, dir, mainDir)
		}
	}
}

// TestPendingBranches_GitRunFnCalledInMainDir verifies that PendingBranches
// executes git commands in the main directory.
func TestPendingBranches_GitRunFnCalledInMainDir(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	var capturedDir string
	mockGitRun := func(dir string, args ...string) (string, error) {
		capturedDir = dir
		if args[0] == "for-each-ref" {
			return "refs/heads/gromit/retro-123\n", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	_, err = m.PendingBranches()
	if err != nil {
		t.Fatalf("PendingBranches() error = %v, want nil", err)
	}

	// Git command should run in the main directory
	if capturedDir != mainDir {
		t.Errorf("PendingBranches() called gitRunFn with dir %q, want %q", capturedDir, mainDir)
	}
}

// TestMergeBack_DeletesBranchOnlyAfterSuccessfulMerge verifies that
// MergeBack deletes the branch only when the merge succeeds.
func TestMergeBack_DeletesBranchOnlyAfterSuccessfulMerge(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	callOrder := []string{}
	mockGitRun := func(dir string, args ...string) (string, error) {
		callOrder = append(callOrder, args[0])
		if args[0] == "merge" {
			return "Merged successfully", nil
		}
		if args[0] == "branch" {
			return "Deleted branch", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/retro-123")
	if err != nil {
		t.Fatalf("MergeBack() error = %v, want nil", err)
	}

	// Verify order: merge happens before branch deletion
	mergeIndex := -1
	branchIndex := -1
	for i, call := range callOrder {
		if call == "merge" && mergeIndex == -1 {
			mergeIndex = i
		}
		if call == "branch" {
			branchIndex = i
		}
	}

	if mergeIndex == -1 {
		t.Error("expected merge command to be called")
	}
	if branchIndex == -1 {
		t.Error("expected branch deletion command to be called")
	}
	if mergeIndex >= branchIndex {
		t.Errorf("branch deletion should happen after merge (merge at %d, branch at %d)", mergeIndex, branchIndex)
	}
}

func TestMergeBack_FastForwardSuccess_DoesNotRemoveDerivedSessionWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	removedWorktree := false
	mockGitRun := func(dir string, args ...string) (string, error) {
		if args[0] == "merge" && contains(args, "--ff-only") {
			return "Fast-forward merge successful", nil
		}
		if args[0] == "branch" && args[1] == "-d" {
			return "Deleted branch", nil
		}
		if args[0] == "worktree" && args[1] == "remove" {
			removedWorktree = true
			return "", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/review-123")
	if err != nil {
		t.Fatalf("MergeBack() error = %v, want nil", err)
	}

	if removedWorktree {
		t.Fatal("MergeBack() should not remove derived worktree")
	}
}

func TestMergeBack_FallbackToMergeCommit_DoesNotRemoveDerivedSessionWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	removedWorktree := false
	mockGitRun := func(dir string, args ...string) (string, error) {
		if args[0] == "merge" && contains(args, "--ff-only") {
			return "", errors.New("fatal: Not possible to fast-forward, aborting")
		}
		if args[0] == "merge" && !contains(args, "--ff-only") {
			return "Merge made by the 'recursive' strategy", nil
		}
		if args[0] == "branch" && args[1] == "-d" {
			return "Deleted branch", nil
		}
		if args[0] == "worktree" && args[1] == "remove" {
			removedWorktree = true
			return "", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/review-123")
	if err != nil {
		t.Fatalf("MergeBack() error = %v, want nil", err)
	}

	if removedWorktree {
		t.Fatal("MergeBack() should not remove derived worktree")
	}
}

func TestMergeBack_BranchDeleteErrorReturnsFailure(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	mockGitRun := func(dir string, args ...string) (string, error) {
		if args[0] == "merge" && contains(args, "--ff-only") {
			return "Fast-forward", nil
		}
		if args[0] == "branch" && args[1] == "-d" {
			return "", errors.New("delete failed")
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = m.MergeBack("gromit/review-123")
	if err != nil {
		if !strings.Contains(err.Error(), "delete failed") {
			t.Fatalf("MergeBack() error = %v, want delete failure context", err)
		}
		return
	}
	t.Fatal("MergeBack() error = nil, want branch delete failure")
}

func TestCreateSessionWorktree_UniqueNames(t *testing.T) {
	// Expected failure: CreateSessionWorktree does not exist yet and session-specific
	// worktree/branch naming is not implemented.

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

	first, err := m.CreateSessionWorktree("review")
	if err != nil {
		t.Fatalf("CreateSessionWorktree() error = %v, want nil", err)
	}

	second, err := m.CreateSessionWorktree("review")
	if err != nil {
		t.Fatalf("CreateSessionWorktree() error = %v, want nil", err)
	}

	if first.BranchName == "" || second.BranchName == "" {
		t.Fatalf("expected non-empty branch names, got %q and %q", first.BranchName, second.BranchName)
	}
	if first.WorktreeDir == "" || second.WorktreeDir == "" {
		t.Fatalf("expected non-empty worktree dirs, got %q and %q", first.WorktreeDir, second.WorktreeDir)
	}
	if first.BranchName == second.BranchName {
		t.Errorf("expected unique branch names per session, got %q", first.BranchName)
	}
	if first.WorktreeDir == second.WorktreeDir {
		t.Errorf("expected unique worktree paths per session, got %q", first.WorktreeDir)
	}
	if !strings.Contains(first.BranchName, "review") || !strings.Contains(second.BranchName, "review") {
		t.Errorf("expected branch names to include command %q, got %q and %q", "review", first.BranchName, second.BranchName)
	}
	if !strings.Contains(filepath.Base(first.WorktreeDir), "review") || !strings.Contains(filepath.Base(second.WorktreeDir), "review") {
		t.Errorf("expected worktree dirs to include command %q, got %q and %q", "review", first.WorktreeDir, second.WorktreeDir)
	}
	if first.WorktreeDir == mainDir+"-gromit-interactive" || second.WorktreeDir == mainDir+"-gromit-interactive" {
		t.Errorf("expected session worktrees to avoid legacy path %q", mainDir+"-gromit-interactive")
	}
}

func TestCreateSessionWorktree_RetriesOnBranchContention(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	var calls [][]string
	mockGitRun := func(dir string, args ...string) (string, error) {
		copied := append([]string{}, args...)
		calls = append(calls, copied)
		if len(args) >= 5 && args[0] == "worktree" && args[1] == "add" {
			branch := args[4]
			if strings.HasSuffix(branch, "-100") {
				return "", errors.New("fatal: a branch named 'gromit/review-100' already exists")
			}
			return "", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	origNowFn := sessionTimestampFn
	sessionTimestampFn = func() int64 { return 100 }
	t.Cleanup(func() { sessionTimestampFn = origNowFn })

	got, err := m.CreateSessionWorktree("review")
	if err != nil {
		t.Fatalf("CreateSessionWorktree() error = %v, want nil after retry", err)
	}

	if strings.HasSuffix(got.BranchName, "-100") {
		t.Fatalf("expected retry to produce a different branch name, got %q", got.BranchName)
	}

	addCalls := 0
	for _, call := range calls {
		if len(call) >= 2 && call[0] == "worktree" && call[1] == "add" {
			addCalls++
		}
	}
	if addCalls < 2 {
		t.Fatalf("expected at least 2 worktree add attempts, got %d", addCalls)
	}
}

func TestCreateSessionWorktree_RetriesWhenLockRefAmbiguousAndBranchExists(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	origNowFn := sessionTimestampFn
	sessionTimestampFn = func() int64 { return 100 }
	t.Cleanup(func() { sessionTimestampFn = origNowFn })

	attempts := 0
	branchProbeCalls := 0
	mockGitRun := func(dir string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			attempts++
			if attempts == 1 {
				return "", errors.New("fatal: cannot lock ref 'refs/heads/gromit/review-100': File exists")
			}
			return "", nil
		}
		if len(args) == 5 &&
			args[0] == "show-ref" &&
			args[1] == "--verify" &&
			args[2] == "--quiet" &&
			args[3] == "refs/heads/gromit/review-100" {
			branchProbeCalls++
			return "", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	got, err := m.CreateSessionWorktree("review")
	if err != nil {
		t.Fatalf("CreateSessionWorktree() error = %v, want nil after retry", err)
	}
	if got == nil {
		t.Fatal("CreateSessionWorktree() returned nil session")
	}
	if attempts != 2 {
		t.Fatalf("expected retry after confirmed branch contention, got %d worktree add attempts", attempts)
	}
	if branchProbeCalls != 1 {
		t.Fatalf("expected one branch-exists fallback probe, got %d", branchProbeCalls)
	}
}

func TestCreateSessionWorktree_RetriesWhenLockRefAmbiguousAndWorktreeAlreadyRegistered(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	origNowFn := sessionTimestampFn
	sessionTimestampFn = func() int64 { return 100 }
	t.Cleanup(func() { sessionTimestampFn = origNowFn })

	attempts := 0
	branchProbeCalls := 0
	worktreeProbeCalls := 0
	targetWorktree := sessionWorktreeDir(mainDir, "review", 100)
	mockGitRun := func(dir string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			attempts++
			if attempts == 1 {
				return "", errors.New("fatal: cannot lock ref 'refs/heads/gromit/review-100': File exists")
			}
			return "", nil
		}
		if len(args) == 5 &&
			args[0] == "show-ref" &&
			args[1] == "--verify" &&
			args[2] == "--quiet" &&
			args[3] == "refs/heads/gromit/review-100" {
			branchProbeCalls++
			return "", errors.New("exit status 1")
		}
		if len(args) == 3 &&
			args[0] == "worktree" &&
			args[1] == "list" &&
			args[2] == "--porcelain" {
			worktreeProbeCalls++
			return "worktree " + targetWorktree + "\nHEAD abc\nbranch refs/heads/main\n", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	got, err := m.CreateSessionWorktree("review")
	if err != nil {
		t.Fatalf("CreateSessionWorktree() error = %v, want nil after retry", err)
	}
	if got == nil {
		t.Fatal("CreateSessionWorktree() returned nil session")
	}
	if attempts != 2 {
		t.Fatalf("expected retry after confirmed worktree contention, got %d worktree add attempts", attempts)
	}
	if branchProbeCalls != 1 {
		t.Fatalf("expected one branch-exists probe, got %d", branchProbeCalls)
	}
	if worktreeProbeCalls != 1 {
		t.Fatalf("expected one worktree-list probe, got %d", worktreeProbeCalls)
	}
}

func TestCreateSessionWorktree_RetriesWhenLockRefAmbiguousExistsCannotCreateAndBranchExists(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	origNowFn := sessionTimestampFn
	sessionTimestampFn = func() int64 { return 100 }
	t.Cleanup(func() { sessionTimestampFn = origNowFn })

	attempts := 0
	branchProbeCalls := 0
	mockGitRun := func(dir string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			attempts++
			if attempts == 1 {
				return "", errors.New("fatal: cannot lock ref 'refs/heads/gromit/review-100': 'refs/heads/gromit/review-100' exists; cannot create 'refs/heads/gromit/review-100'")
			}
			return "", nil
		}
		if len(args) == 5 &&
			args[0] == "show-ref" &&
			args[1] == "--verify" &&
			args[2] == "--quiet" &&
			args[3] == "refs/heads/gromit/review-100" {
			branchProbeCalls++
			return "", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	got, err := m.CreateSessionWorktree("review")
	if err != nil {
		t.Fatalf("CreateSessionWorktree() error = %v, want nil after retry", err)
	}
	if got == nil {
		t.Fatal("CreateSessionWorktree() returned nil session")
	}
	if attempts != 2 {
		t.Fatalf("expected retry after confirmed branch contention, got %d worktree add attempts", attempts)
	}
	if branchProbeCalls != 1 {
		t.Fatalf("expected one branch-exists probe, got %d", branchProbeCalls)
	}
}

func TestCreateSessionWorktree_AmbiguousProbeInconclusiveIncludesDecisionReason(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	origNowFn := sessionTimestampFn
	sessionTimestampFn = func() int64 { return 100 }
	t.Cleanup(func() { sessionTimestampFn = origNowFn })

	mockGitRun := func(dir string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			return "", errors.New("fatal: cannot lock ref 'refs/heads/gromit/review-100': File exists")
		}
		if len(args) >= 4 && args[0] == "show-ref" && args[1] == "--verify" {
			return "", errors.New("exit status 1")
		}
		if len(args) == 3 && args[0] == "worktree" && args[1] == "list" && args[2] == "--porcelain" {
			return "worktree " + mainDir + "\nHEAD abc\nbranch refs/heads/main\n", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	_, err = m.CreateSessionWorktree("review")
	if err == nil {
		t.Fatal("CreateSessionWorktree() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "ambiguous_probe_inconclusive") {
		t.Fatalf("expected error to include structured decision reason, got: %v", err)
	}
}

func TestCreateSessionWorktree_RetriesWhenKnownContentionOnlyInGitOutput(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	origNowFn := sessionTimestampFn
	sessionTimestampFn = func() int64 { return 100 }
	t.Cleanup(func() { sessionTimestampFn = origNowFn })

	attempts := 0
	mockGitRun := func(dir string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			attempts++
			if attempts == 1 {
				return "fatal: a branch named 'gromit/review-100' already exists", errors.New("exit status 128")
			}
			return "", nil
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	got, err := m.CreateSessionWorktree("review")
	if err != nil {
		t.Fatalf("CreateSessionWorktree() error = %v, want nil after retry", err)
	}
	if got == nil {
		t.Fatal("CreateSessionWorktree() returned nil session")
	}
	if attempts != 2 {
		t.Fatalf("expected retry based on git output contention signature, got %d attempts", attempts)
	}
}

func TestCreateSessionWorktree_FailsImmediatelyOnNonContentionAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	attempts := 0
	mockGitRun := func(dir string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			attempts++
			return "", errors.New("fatal: remote 'origin' already exists")
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	_, err = m.CreateSessionWorktree("review")
	if err == nil {
		t.Fatal("CreateSessionWorktree() error = nil, want non-nil")
	}
	if attempts != 1 {
		t.Fatalf("expected exactly 1 worktree add attempt, got %d", attempts)
	}
	if !strings.Contains(err.Error(), "remote 'origin' already exists") {
		t.Fatalf("expected error to preserve root cause, got: %v", err)
	}
}

func TestCreateSessionWorktree_RetryExhaustionOnWorktreeCollision(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	attempts := 0
	mockGitRun := func(dir string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			attempts++
			return "", errors.New("fatal: '/tmp/repo-gromit-review-100' is already registered as a worktree")
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	_, err = m.CreateSessionWorktree("review")
	if err == nil {
		t.Fatal("CreateSessionWorktree() error = nil, want non-nil")
	}
	if attempts != maxSessionCreateRetries {
		t.Fatalf("expected %d retries for worktree collision, got %d", maxSessionCreateRetries, attempts)
	}
	if !strings.Contains(err.Error(), "already registered as a worktree") {
		t.Fatalf("expected error to preserve root cause, got: %v", err)
	}
}

func TestCreateSessionWorktree_MixedRetryableThenNonRetryableStopsImmediately(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatalf("failed to create main dir: %v", err)
	}

	attempts := 0
	mockGitRun := func(dir string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			attempts++
			switch attempts {
			case 1:
				return "", errors.New("fatal: a branch named 'gromit/review-100' already exists")
			case 2:
				return "", errors.New("fatal: reference already exists in packed-refs")
			default:
				return "", nil
			}
		}
		return "", nil
	}

	m, err := NewManager(mainDir, WithGitRunFn(mockGitRun))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	_, err = m.CreateSessionWorktree("review")
	if err == nil {
		t.Fatal("CreateSessionWorktree() error = nil, want non-nil")
	}
	if attempts != 2 {
		t.Fatalf("expected to stop after non-retryable second failure, got %d attempts", attempts)
	}
	if !strings.Contains(err.Error(), "reference already exists in packed-refs") {
		t.Fatalf("expected error to preserve non-retryable root cause, got: %v", err)
	}
}

// contains checks if a string slice contains a specific string.
func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

func TestSessionWorktreeContentionContractDocumented(t *testing.T) {
	decisionPath := filepath.Join("..", "..", "docs", "plans", "2026-02-23-session-worktree-contention-contract.md")
	content, err := os.ReadFile(decisionPath)
	if err != nil {
		t.Fatalf("read decision record %q: %v", decisionPath, err)
	}

	doc := string(content)
	requiredSnippets := []string{
		"Retryable contention classes",
		"Terminal (non-retryable) classes",
		"Ambiguous failure handling",
		"Cross-version and environment behavior",
		"gromit-1fjzj",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(doc, snippet) {
			t.Fatalf("decision record missing required section/snippet %q", snippet)
		}
	}
}
