package debug

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateFix_RunsStageAfterFix validates that a fix passes validation.
func TestValidateFix_RunsStageAfterFix(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a simple file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("valid content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create validation context - use a simple command that will pass
	validCtx := &ValidateContext{
		WorktreeRoot: tmpDir,
		FailedStage:  "build",
		ValidateCmd:  "test -f test.txt",
	}

	// Validate the fix
	result, err := ValidateFix(ctx, validCtx)
	if err != nil {
		t.Fatalf("ValidateFix failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("result.Passed = false, want true; output: %s", result.Output)
	}
}

// TestValidateFix_CaptsuresFailureOutput captures output when validation fails.
func TestValidateFix_CapturesFailureOutput(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a file with a syntax error
	testFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc broken() {\n  return // missing value\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	validCtx := &ValidateContext{
		WorktreeRoot: tmpDir,
		FailedStage:  "build",
		ValidateCmd:  "go build -v",
	}

	result, err := ValidateFix(ctx, validCtx)
	if err != nil {
		// Non-nil error is OK - validation failed as expected
	}

	if result.Passed {
		t.Error("result.Passed = true, want false for broken code")
	}
	if result.Output == "" && result.Error == "" {
		t.Error("expected output or error from failed validation")
	}
}

// TestValidateFix_ReturnsErrorForNilContext returns error for nil context.
func TestValidateFix_ReturnsErrorForNilContext(t *testing.T) {
	ctx := context.Background()
	_, err := ValidateFix(ctx, nil)
	if err == nil {
		t.Error("expected error for nil ValidateContext, got nil")
	}
}

func TestValidateFix_ReRunsFailedBuildStageWhenCommandMissing(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	goMod := "module example.com/debugvalidate\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	brokenSource := "package main\n\nfunc main() {\n\tmissing(\n}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(brokenSource), 0o644); err != nil {
		t.Fatal(err)
	}

	validCtx := &ValidateContext{
		WorktreeRoot: tmpDir,
		FailedStage:  "build",
	}

	result, err := ValidateFix(ctx, validCtx)
	if err != nil {
		t.Fatalf("ValidateFix() error = %v", err)
	}
	if result.Passed {
		t.Fatalf("result.Passed = true, want false for failed build stage rerun; output=%q error=%q", result.Output, result.Error)
	}
	if result.Output == "" && result.Error == "" {
		t.Fatal("expected failure details from rerun build stage")
	}
}

func TestValidateAndCommitFix_RerunsStageValidationAndCommitsFix(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	trace := StageTrace{
		StageName: "validate",
		BeadID:    "bead-1",
		Iteration: 2,
		Validation: &ValidationTrace{
			Commands: []string{"printf stage-fixed"},
		},
	}
	committer := &recordingStageCommitter{t: t}
	validCtx := &ValidateContext{
		WorktreeRoot: tmpDir,
		StageTrace:   &trace,
	}

	result, err := ValidateAndCommitFix(ctx, validCtx, committer)
	if err != nil {
		t.Fatalf("ValidateAndCommitFix() error = %v", err)
	}
	if !result.Passed {
		t.Fatalf("result.Passed = false, want true; output: %s", result.Output)
	}
	if !strings.Contains(result.Output, "stage-fixed") {
		t.Fatalf("expected stage command output, got %q", result.Output)
	}
	if !committer.called {
		t.Fatal("expected stage commit to run")
	}
	if committer.stageName != "validate" {
		t.Fatalf("stageName = %q, want %q", committer.stageName, "validate")
	}
	if committer.iteration != 3 {
		t.Fatalf("iteration = %d, want %d", committer.iteration, 3)
	}
}

type recordingStageCommitter struct {
	t         *testing.T
	called    bool
	beadID    string
	stageName string
	iteration int
	decision  string
}

func (r *recordingStageCommitter) CommitStage(ctx context.Context, worktree, beadID, stageName string, iteration int, decision string) error {
	if r.called {
		r.t.Fatalf("CommitStage called multiple times")
	}
	r.called = true
	r.beadID = beadID
	r.stageName = stageName
	r.iteration = iteration
	r.decision = decision
	return nil
}
