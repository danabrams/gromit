package debug

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplySystemicFix_RecommendsHumanReviewForSystemicPatch(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	fragmentDir := filepath.Join(tmpDir, ".gromit", "fragments")
	if err := os.MkdirAll(fragmentDir, 0o755); err != nil {
		t.Fatalf("setup fragments dir: %v", err)
	}

	fragmentPath := filepath.Join(fragmentDir, "build.md")
	if err := os.WriteFile(fragmentPath, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("write fragment: %v", err)
	}

	patch := strings.Join([]string{
		"diff --git a/.gromit/fragments/build.md b/.gromit/fragments/build.md",
		"--- a/.gromit/fragments/build.md",
		"+++ b/.gromit/fragments/build.md",
		"@@ -1 +1 @@",
		"-old",
		"+new",
		"",
	}, "\n")

	req := &SystemicFixInput{
		FixCtx: &FixContext{
			WorktreeRoot: tmpDir,
			CodePatch:    patch,
			ErrorMsg:     "Prompt fragment is ambiguous and needs clarity",
		},
		RootCause:     RootCauseUnclearBead,
		FailureSignal: "Prompt fragment is ambiguous and needs clarity",
	}

	result, err := ApplySystemicFix(ctx, req)
	if err != nil {
		t.Fatalf("ApplySystemicFix() error = %v", err)
	}
	if result == nil {
		t.Fatalf("result = nil")
	}
	if result.Applied {
		t.Fatalf("expected systemic patch not to apply automatically")
	}
	if result.SystemicRecommendation == "" {
		t.Fatalf("systemic recommendation empty, want guidance")
	}
	if !strings.Contains(strings.ToLower(result.SystemicRecommendation), "human review") {
		t.Fatalf("recommendation = %q, want human-review reminder", result.SystemicRecommendation)
	}

	updated, err := os.ReadFile(fragmentPath)
	if err != nil {
		t.Fatalf("read fragment: %v", err)
	}
	if strings.Contains(string(updated), "new") {
		t.Fatalf("fragment unexpectedly changed, got %q", string(updated))
	}
}

func TestApplySystemicFix_FiltersSystemicSections(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()
	runGitFix(t, tmpDir, "init")
	runGitFix(t, tmpDir, "config", "user.email", "tester@example.com")
	runGitFix(t, tmpDir, "config", "user.name", "Test User")

	mainPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainPath, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	fragmentsDir := filepath.Join(tmpDir, ".gromit", "fragments")
	if err := os.MkdirAll(fragmentsDir, 0o755); err != nil {
		t.Fatalf("create fragments dir: %v", err)
	}
	fragmentPath := filepath.Join(fragmentsDir, "build.md")
	if err := os.WriteFile(fragmentPath, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("write fragment: %v", err)
	}
	runGitFix(t, tmpDir, "add", ".")
	runGitFix(t, tmpDir, "commit", "-m", "initial state")

	patch := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,3 +1,5 @@",
		" package main",
		" ",
		"-func main() {}",
		"+func main() {",
		"+    println(\"fixed\")",
		"+}",
		"",
		"diff --git a/.gromit/fragments/build.md b/.gromit/fragments/build.md",
		"--- a/.gromit/fragments/build.md",
		"+++ b/.gromit/fragments/build.md",
		"@@ -1 +1 @@",
		"-old",
		"+new",
		"",
	}, "\n")

	req := &SystemicFixInput{
		FixCtx: &FixContext{
			WorktreeRoot: tmpDir,
			CodePatch:    patch,
		},
		RootCause:     RootCauseUnclearBead,
		FailureSignal: "Prompt fragment is ambiguous and needs clarity",
	}

	result, err := ApplySystemicFix(ctx, req)
	if err != nil {
		t.Fatalf("ApplySystemicFix() error = %v", err)
	}
	if !result.Applied {
		t.Fatalf("expected fix to apply, got %+v", result)
	}
	if result.SystemicRecommendation == "" {
		t.Fatal("expected systemic recommendation, got empty")
	}
	if !strings.Contains(strings.ToLower(result.SystemicRecommendation), "human review") {
		t.Fatalf("recommendation = %q, want human-review reminder", result.SystemicRecommendation)
	}

	mainContent, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(mainContent), "fixed") {
		t.Fatalf("main.go not updated, got %q", string(mainContent))
	}

	fragmentContent, err := os.ReadFile(fragmentPath)
	if err != nil {
		t.Fatalf("read fragment: %v", err)
	}
	if strings.Contains(string(fragmentContent), "new") {
		t.Fatalf("fragment unexpectedly changed, got %q", string(fragmentContent))
	}
}
