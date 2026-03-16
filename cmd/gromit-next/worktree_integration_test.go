package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreeIntegration_FullLifecycle(t *testing.T) {
	// Create a real repo with a committed file
	repoDir := initRepoWithFile(t, "main.go", "package main\n")
	ops := &realGitOps{}

	// Create worktree
	wtPath, err := ops.CreateWorktree(repoDir, "gromit/spec-test-run1")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer ops.RemoveWorktree(wtPath)

	// Verify worktree dir exists
	info, err := os.Stat(wtPath)
	if err != nil {
		t.Fatalf("worktree dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("worktree path should be a directory")
	}

	// Verify worktree has the repo contents
	data, err := os.ReadFile(filepath.Join(wtPath, "main.go"))
	if err != nil {
		t.Fatalf("main.go should exist in worktree: %v", err)
	}
	if string(data) != "package main\n" {
		t.Errorf("unexpected worktree file content: %q", string(data))
	}

	// Create a new file in the worktree
	newFile := filepath.Join(wtPath, "worktree_only.txt")
	if err := os.WriteFile(newFile, []byte("only in worktree\n"), 0o644); err != nil {
		t.Fatalf("write new file in worktree: %v", err)
	}

	// Verify the new file does NOT appear in the main repo (isolation)
	mainRepoFile := filepath.Join(repoDir, "worktree_only.txt")
	if _, err := os.Stat(mainRepoFile); !os.IsNotExist(err) {
		t.Errorf("new worktree file should NOT exist in main repo, err=%v", err)
	}

	// Remove worktree
	if err := ops.RemoveWorktree(wtPath); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	// Verify dir is gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree dir should not exist after removal, err=%v", err)
	}
}

func TestWorktreeIntegration_ChangesIsolated(t *testing.T) {
	originalContent := "package main\n\nfunc hello() string { return \"hello\" }\n"
	repoDir := initRepoWithFile(t, "lib.go", originalContent)
	ops := &realGitOps{}

	// Create worktree
	wtPath, err := ops.CreateWorktree(repoDir, "gromit/spec-isolation-test")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer ops.RemoveWorktree(wtPath)

	// Modify the file in the worktree
	modifiedContent := "package main\n\nfunc hello() string { return \"modified\" }\n"
	wtFile := filepath.Join(wtPath, "lib.go")
	if err := os.WriteFile(wtFile, []byte(modifiedContent), 0o644); err != nil {
		t.Fatalf("write modified file: %v", err)
	}

	// Verify main repo still has original content
	mainData, err := os.ReadFile(filepath.Join(repoDir, "lib.go"))
	if err != nil {
		t.Fatalf("read main repo file: %v", err)
	}
	if string(mainData) != originalContent {
		t.Errorf("main repo should have original content, got: %q", string(mainData))
	}

	// Verify worktree has modified content
	wtData, err := os.ReadFile(wtFile)
	if err != nil {
		t.Fatalf("read worktree file: %v", err)
	}
	if string(wtData) != modifiedContent {
		t.Errorf("worktree should have modified content, got: %q", string(wtData))
	}
}
