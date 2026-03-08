package debug

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestApplyFix_CreatesCodeFixFromContext applies a code fix and validates it passes.
func TestApplyFix_CreatesCodeFixFromContext(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a test file with a simple error
	testFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc broken() int {\n  return // missing value\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a fix context describing what's wrong
	fixCtx := &FixContext{
		FailedStage:   "build",
		ErrorMsg:      "syntax error: missing return value",
		FilesInvolved: []string{"main.go"},
		WorktreeRoot:  tmpDir,
	}

	// Apply the fix - should succeed with a validated result
	result, err := ApplyFix(ctx, fixCtx)
	if err != nil {
		t.Fatalf("ApplyFix failed: %v", err)
	}
	if !result.Applied {
		t.Error("result.Applied = false, want true")
	}
}

// TestApplyFix_ReturnsErrorForMissingContext returns error when FixContext is nil.
func TestApplyFix_ReturnsErrorForMissingContext(t *testing.T) {
	ctx := context.Background()
	_, err := ApplyFix(ctx, nil)
	if err == nil {
		t.Error("expected error for nil FixContext, got nil")
	}
}

func TestApplyFix_ChecksOutFailureCommitAndAppliesPatch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()
	runGitFix(t, tmpDir, "init")
	runGitFix(t, tmpDir, "config", "user.email", "tester@example.com")
	runGitFix(t, tmpDir, "config", "user.name", "Test User")

	testFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc answer() int { return 0 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFix(t, tmpDir, "add", "main.go")
	runGitFix(t, tmpDir, "commit", "-m", "broken commit")

	failureHash := strings.TrimSpace(runGitFix(t, tmpDir, "rev-parse", "HEAD"))

	if err := os.WriteFile(testFile, []byte("package main\n\nfunc answer() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFix(t, tmpDir, "add", "main.go")
	runGitFix(t, tmpDir, "commit", "-m", "later commit")

	patch := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,3 @@\n package main\n \n-func answer() int { return 0 }\n+func answer() int { return 42 }\n"
	fixCtx := &FixContext{
		WorktreeRoot:  tmpDir,
		FailureCommit: failureHash,
		CodePatch:     patch,
	}

	result, err := ApplyFix(ctx, fixCtx)
	if err != nil {
		t.Fatalf("ApplyFix() error = %v", err)
	}
	if !result.Applied {
		t.Fatal("result.Applied = false, want true")
	}

	gotHead := strings.TrimSpace(runGitFix(t, tmpDir, "rev-parse", "HEAD"))
	if gotHead != failureHash {
		t.Fatalf("HEAD after ApplyFix = %q, want %q", gotHead, failureHash)
	}

	updated, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "return 42") {
		t.Fatalf("patched file missing fix, got:\n%s", string(updated))
	}
}

func runGitFix(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return string(out)
}
