package specloop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/provider"
)

// mockInvoker captures the prompt and returns a configured result.
type mockInvoker struct {
	capturedPrompt  string
	capturedDir     string
	usedInvokeInDir bool
	result          *provider.Result
	err             error
}

func (m *mockInvoker) Invoke(_ context.Context, prompt string) (*provider.Result, error) {
	m.capturedPrompt = prompt
	return m.result, m.err
}

func (m *mockInvoker) InvokeInDir(_ context.Context, prompt string, dir string) (*provider.Result, error) {
	m.capturedPrompt = prompt
	m.capturedDir = dir
	m.usedInvokeInDir = true
	return m.result, m.err
}

func TestProviderTaskRunner_CompileTimeInterfaceCheck(t *testing.T) {
	// Compile-time check is in provider_taskrunner.go:
	// var _ TaskRunner = (*ProviderTaskRunner)(nil)
	// This test just documents it exists; compilation proves it.
	var _ TaskRunner = (*ProviderTaskRunner)(nil)
}

func TestProviderTaskRunner_RunTask_SuccessMappedToDone(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:      true,
			Output:       "done",
			Model:        "sonnet",
			CostUSD:      0.05,
			InputTokens:  1000,
			OutputTokens: 500,
			Duration:     2 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{
		TaskID:    "t-1",
		Objective: "implement feature X",
	}

	result, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "done" {
		t.Errorf("expected status 'done', got %q", result.Status)
	}
}

func TestProviderTaskRunner_RunTask_FailureMappedToFailed(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:      false,
			Output:       "error occurred",
			Model:        "sonnet",
			CostUSD:      0.03,
			InputTokens:  800,
			OutputTokens: 200,
			Duration:     1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{TaskID: "t-2", Objective: "fix bug Y"}

	result, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", result.Status)
	}
}

func TestProviderTaskRunner_RunTask_TokensUsedIsSum(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:      true,
			Model:        "sonnet",
			InputTokens:  1234,
			OutputTokens: 5678,
			Duration:     1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{TaskID: "t-3", Objective: "anything"}

	result, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TokensUsed != 1234+5678 {
		t.Errorf("expected TokensUsed=%d, got %d", 1234+5678, result.TokensUsed)
	}
}

func TestProviderTaskRunner_RunTask_PromptIncludesObjectiveAndArea(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{
		TaskID:              "t-4",
		Objective:           "implement the frobnicator",
		ExpectedTouchedArea: []string{"internal/frob/", "cmd/frob/"},
		ProofChecks:         []string{"go test ./internal/frob/..."},
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(inv.capturedPrompt, "implement the frobnicator") {
		t.Error("prompt does not contain task objective")
	}
	if !strings.Contains(inv.capturedPrompt, "internal/frob/") {
		t.Error("prompt does not contain expected touched area")
	}
	if !strings.Contains(inv.capturedPrompt, "go test ./internal/frob/...") {
		t.Error("prompt does not contain proof checks")
	}
}

func TestProviderTaskRunner_RunTask_MapsAllResultFields(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:      true,
			Model:        "opus",
			CostUSD:      0.12,
			InputTokens:  2000,
			OutputTokens: 3000,
			Duration:     3500 * time.Millisecond,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{TaskID: "t-5", Objective: "test"}

	result, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != "opus" {
		t.Errorf("expected Model='opus', got %q", result.Model)
	}
	if result.Cost != 0.12 {
		t.Errorf("expected Cost=0.12, got %f", result.Cost)
	}
	if result.DurationMs != 3500 {
		t.Errorf("expected DurationMs=3500, got %d", result.DurationMs)
	}
	if result.FilesChanged == nil || len(result.FilesChanged) != 0 {
		t.Errorf("expected empty FilesChanged slice, got %v", result.FilesChanged)
	}
}

func TestProviderTaskRunner_RepairTask_PromptIncludesFailures(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{
		TaskID:    "t-6",
		Objective: "fix the widget",
	}
	failures := []string{"test_foo failed: assertion error", "lint: unused variable"}

	_, err := runner.RepairTask(context.Background(), task, failures)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(inv.capturedPrompt, "test_foo failed: assertion error") {
		t.Error("repair prompt does not contain first failure")
	}
	if !strings.Contains(inv.capturedPrompt, "lint: unused variable") {
		t.Error("repair prompt does not contain second failure")
	}
	if !strings.Contains(inv.capturedPrompt, "fix the widget") {
		t.Error("repair prompt does not contain task objective")
	}
}

func TestProviderTaskRunner_TaskPrompt_WithArchitectureConstraints(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	runner.SetContextProvider(func() TaskContext {
		return TaskContext{
			ArchitectureConstraints: []string{"use NormalizeNilFields for cross-package types", "separate validation in haiku invocation"},
		}
	})
	task := runstore.Task{
		TaskID:          "t-arch-1",
		Objective:       "implement with conventions",
		SpecConstraints: "- do not modify existing test files",
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(inv.capturedPrompt, "### Architecture Conventions") {
		t.Error("prompt does not contain '### Architecture Conventions' section header")
	}
	if !strings.Contains(inv.capturedPrompt, "- use NormalizeNilFields for cross-package types") {
		t.Error("prompt does not contain first architecture constraint with '- ' prefix")
	}
	if !strings.Contains(inv.capturedPrompt, "- separate validation in haiku invocation") {
		t.Error("prompt does not contain second architecture constraint with '- ' prefix")
	}
	specIdx := strings.Index(inv.capturedPrompt, "### Spec Constraints")
	archIdx := strings.Index(inv.capturedPrompt, "### Architecture Conventions")
	if specIdx == -1 {
		t.Error("prompt does not contain '### Spec Constraints' section")
	} else if archIdx <= specIdx {
		t.Errorf("expected '### Architecture Conventions' (idx %d) to appear after '### Spec Constraints' (idx %d)", archIdx, specIdx)
	}
}

func TestProviderTaskRunner_TaskPrompt_WithoutArchitectureConstraints(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })

	// Test with nil ArchitectureConstraints (no context provider set)
	taskNil := runstore.Task{
		TaskID:    "t-arch-2",
		Objective: "implement without constraints",
	}

	_, err := runner.RunTask(context.Background(), taskNil)
	if err != nil {
		t.Fatalf("unexpected error with nil constraints: %v", err)
	}

	if strings.Contains(inv.capturedPrompt, "### Architecture Conventions") {
		t.Error("prompt should not contain '### Architecture Conventions' when constraints are nil")
	}

	// Test with empty ArchitectureConstraints
	inv2 := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner2 := NewProviderTaskRunner(inv2, func() string { return "" })
	runner2.SetContextProvider(func() TaskContext {
		return TaskContext{
			ArchitectureConstraints: []string{},
		}
	})
	taskEmpty := runstore.Task{
		TaskID:    "t-arch-3",
		Objective: "implement without constraints",
	}

	_, err = runner2.RunTask(context.Background(), taskEmpty)
	if err != nil {
		t.Fatalf("unexpected error with empty constraints: %v", err)
	}

	if strings.Contains(inv2.capturedPrompt, "### Architecture Conventions") {
		t.Error("prompt should not contain '### Architecture Conventions' when constraints are empty")
	}
}

func TestProviderTaskRunner_RepairTask_MapsResultCorrectly(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:      true,
			Model:        "opus",
			CostUSD:      0.08,
			InputTokens:  1500,
			OutputTokens: 2500,
			Duration:     2 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{TaskID: "t-7", Objective: "repair something"}

	result, err := runner.RepairTask(context.Background(), task, []string{"failure1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "done" {
		t.Errorf("expected status 'done', got %q", result.Status)
	}
	if result.TokensUsed != 4000 {
		t.Errorf("expected TokensUsed=4000, got %d", result.TokensUsed)
	}
	if result.Cost != 0.08 {
		t.Errorf("expected Cost=0.08, got %f", result.Cost)
	}
	if result.Model != "opus" {
		t.Errorf("expected Model='opus', got %q", result.Model)
	}
}

func TestProviderTaskRunner_RunTask_NilResult(t *testing.T) {
	inv := &mockInvoker{
		result: nil,
		err:    nil,
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{TaskID: "t-nil-run", Objective: "should handle nil"}

	result, err := runner.RunTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for nil result, got nil")
	}
	if !strings.Contains(err.Error(), "nil result") {
		t.Errorf("expected nil-result error message, got %q", err.Error())
	}
	// Zero-value TaskResult with initialized FilesChanged slice
	if result.Status != "" {
		t.Errorf("expected empty Status, got %q", result.Status)
	}
	if result.TokensUsed != 0 {
		t.Errorf("expected TokensUsed=0, got %d", result.TokensUsed)
	}
	if result.FilesChanged == nil {
		t.Error("expected non-nil FilesChanged slice")
	}
	if len(result.FilesChanged) != 0 {
		t.Errorf("expected empty FilesChanged, got %v", result.FilesChanged)
	}
}

func TestProviderTaskRunner_RepairTask_NilResult(t *testing.T) {
	inv := &mockInvoker{
		result: nil,
		err:    nil,
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{TaskID: "t-nil-repair", Objective: "should handle nil"}

	result, err := runner.RepairTask(context.Background(), task, []string{"some failure"})
	if err == nil {
		t.Fatal("expected error for nil result, got nil")
	}
	if !strings.Contains(err.Error(), "nil result") {
		t.Errorf("expected nil-result error message, got %q", err.Error())
	}
	// Zero-value TaskResult with initialized FilesChanged slice
	if result.Status != "" {
		t.Errorf("expected empty Status, got %q", result.Status)
	}
	if result.TokensUsed != 0 {
		t.Errorf("expected TokensUsed=0, got %d", result.TokensUsed)
	}
	if result.FilesChanged == nil {
		t.Error("expected non-nil FilesChanged slice")
	}
	if len(result.FilesChanged) != 0 {
		t.Errorf("expected empty FilesChanged, got %v", result.FilesChanged)
	}
}

func TestProviderTaskRunner_RunTask_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inv := &mockInvoker{
		result: nil,
		err:    context.Canceled,
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{TaskID: "t-ctx", Objective: "should be cancelled"}

	_, err := runner.RunTask(ctx, task)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestProviderTaskRunner_RepairTask_EmptyFailures(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{
		TaskID:    "t-empty-failures",
		Objective: "fix the widget",
	}

	_, err := runner.RepairTask(context.Background(), task, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(inv.capturedPrompt, "Failures to Address") {
		t.Error("prompt should not contain 'Failures to Address' header when failures slice is empty")
	}
}

func TestProviderTaskRunner_RunTask_MinimalTask(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{
		TaskID:    "t-minimal",
		Objective: "do something simple",
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(inv.capturedPrompt, "Expected Touched Area") {
		t.Error("prompt should not contain 'Expected Touched Area' when ExpectedTouchedArea is empty")
	}
	if strings.Contains(inv.capturedPrompt, "Proof Checks") {
		t.Error("prompt should not contain 'Proof Checks' when ProofChecks is empty")
	}
	if !strings.Contains(inv.capturedPrompt, "do something simple") {
		t.Error("prompt should still contain the objective")
	}
}

func TestProviderTaskRunner_InvokerErrorPropagated(t *testing.T) {
	expectedErr := errors.New("connection timeout")
	inv := &mockInvoker{
		result: &provider.Result{
			InputTokens:  100,
			OutputTokens: 0,
			CostUSD:      0.01,
		},
		err: expectedErr,
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{TaskID: "t-8", Objective: "something"}

	result, err := runner.RunTask(context.Background(), task)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
	// Partial result should still have token/cost info
	if result.TokensUsed != 100 {
		t.Errorf("expected partial TokensUsed=100, got %d", result.TokensUsed)
	}
}

func TestProviderTaskRunner_RunTask_UsesInvokeInDirWhenWorkDirSet(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "/tmp/workdir" })
	task := runstore.Task{TaskID: "t-dir-1", Objective: "implement feature"}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inv.usedInvokeInDir {
		t.Error("expected InvokeInDir to be called when workDir is set")
	}
	if inv.capturedDir != "/tmp/workdir" {
		t.Errorf("expected dir '/tmp/workdir', got %q", inv.capturedDir)
	}
}

func TestProviderTaskRunner_RunTask_UsesInvokeWhenWorkDirEmpty(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{TaskID: "t-dir-2", Objective: "implement feature"}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.usedInvokeInDir {
		t.Error("expected Invoke (not InvokeInDir) to be called when workDir is empty")
	}
}

func TestTaskPrompt_SpecConstraintsIncluded(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{
		TaskID:          "t-sc-1",
		Objective:       "implement the widget",
		SpecConstraints: "## Out-of-Scope\n- Do NOT modify any existing test files\n",
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(inv.capturedPrompt, "Spec Constraints") {
		t.Error("prompt does not contain 'Spec Constraints' header")
	}
	if !strings.Contains(inv.capturedPrompt, "Do NOT modify any existing test files") {
		t.Error("prompt does not contain the constraint text")
	}
}

func TestTaskPrompt_NoSpecConstraintsWhenEmpty(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{
		TaskID:          "t-sc-2",
		Objective:       "implement the widget",
		SpecConstraints: "",
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(inv.capturedPrompt, "Spec Constraints") {
		t.Error("prompt should not contain 'Spec Constraints' header when SpecConstraints is empty")
	}
}

func TestRenderRepairPrompt_SpecConstraintsIncluded(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{
		TaskID:          "t-sc-3",
		Objective:       "fix the widget",
		SpecConstraints: "## Architectural Constraints\n- All code stays in the `calc` package\n",
	}

	_, err := runner.RepairTask(context.Background(), task, []string{"test failed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(inv.capturedPrompt, "Spec Constraints") {
		t.Error("repair prompt does not contain 'Spec Constraints' header")
	}
	if !strings.Contains(inv.capturedPrompt, "All code stays in the `calc` package") {
		t.Error("repair prompt does not contain the constraint text")
	}
}

func TestTaskPrompt_SpecConstraintsAppearBeforeProofChecks(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{
		TaskID:          "t-order-1",
		Objective:       "implement the widget",
		ProofChecks:     []string{"go test ./..."},
		SpecConstraints: "## Out-of-Scope\n- Do NOT modify any existing test files\n",
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	constraintIdx := strings.Index(inv.capturedPrompt, "Spec Constraints")
	proofIdx := strings.Index(inv.capturedPrompt, "Proof Checks")
	if constraintIdx == -1 {
		t.Fatal("prompt does not contain 'Spec Constraints' header")
	}
	if proofIdx == -1 {
		t.Fatal("prompt does not contain 'Proof Checks' header")
	}
	if constraintIdx > proofIdx {
		t.Errorf("Spec Constraints (pos %d) must appear before Proof Checks (pos %d)", constraintIdx, proofIdx)
	}
}

func TestTaskPrompt_ConstraintPreambleMentionsDeletion(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	task := runstore.Task{
		TaskID:          "t-preamble-1",
		Objective:       "implement the widget",
		SpecConstraints: "## Out-of-Scope\n- Do NOT modify any existing test files\n",
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(inv.capturedPrompt, "deleting") {
		t.Error("constraint preamble should mention 'deleting' as a form of modification")
	}
}

func TestProviderTaskRunner_RepairTask_UsesInvokeInDirWhenWorkDirSet(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "/tmp/repair-dir" })
	task := runstore.Task{TaskID: "t-dir-3", Objective: "fix bug"}

	_, err := runner.RepairTask(context.Background(), task, []string{"test failed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inv.usedInvokeInDir {
		t.Error("expected InvokeInDir to be called when workDir is set")
	}
	if inv.capturedDir != "/tmp/repair-dir" {
		t.Errorf("expected dir '/tmp/repair-dir', got %q", inv.capturedDir)
	}
}

func TestProviderTaskRunner_LazyWorkDirResolution(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}

	// Start with empty workDir, then change it between calls.
	currentDir := ""
	runner := NewProviderTaskRunner(inv, func() string { return currentDir })
	task := runstore.Task{TaskID: "t-lazy", Objective: "lazy dir test"}

	// First call: empty dir → should use Invoke (not InvokeInDir).
	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.usedInvokeInDir {
		t.Error("expected Invoke (not InvokeInDir) when workDirFn returns empty string")
	}

	// Change the resolved directory.
	inv.usedInvokeInDir = false
	inv.capturedDir = ""
	currentDir = "/tmp/lazy-worktree"

	// Second call: non-empty dir → should use InvokeInDir with the new value.
	_, err = runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inv.usedInvokeInDir {
		t.Error("expected InvokeInDir when workDirFn returns non-empty string")
	}
	if inv.capturedDir != "/tmp/lazy-worktree" {
		t.Errorf("expected dir '/tmp/lazy-worktree', got %q", inv.capturedDir)
	}
}

// --- TaskContext tests ---

func TestTaskPrompt_IncludesProjectConventions(t *testing.T) {
	task := runstore.Task{TaskID: "t-ctx-1", Objective: "implement feature"}
	tc := TaskContext{ProjectConventions: "# Gromit\nUse Go idioms.\n"}
	prompt := renderTaskPrompt(task, tc, "")

	if !strings.Contains(prompt, "Project Conventions") {
		t.Error("prompt does not contain 'Project Conventions' header")
	}
	if !strings.Contains(prompt, "Use Go idioms.") {
		t.Error("prompt does not contain CLAUDE.md content")
	}
}

func TestTaskPrompt_OriginalTaskOmitsSpec(t *testing.T) {
	task := runstore.Task{TaskID: "t-ctx-2", Objective: "implement feature"}
	tc := TaskContext{SpecContent: "# Spec 0042\nBuild the frobnicator.\n"}
	prompt := renderTaskPrompt(task, tc, "")

	if strings.Contains(prompt, "Full Spec") {
		t.Error("original task prompt should not contain 'Full Spec' — spec is only for fix/repair tasks")
	}
}

func TestTaskPrompt_FixTaskIncludesSpec(t *testing.T) {
	task := runstore.Task{TaskID: "t-ctx-2b", Kind: "fix", Objective: "fix the frobnicator"}
	tc := TaskContext{SpecContent: "# Spec 0042\nBuild the frobnicator.\n"}
	prompt := renderTaskPrompt(task, tc, "")

	if !strings.Contains(prompt, "Full Spec") {
		t.Error("fix task prompt should contain 'Full Spec' header")
	}
	if !strings.Contains(prompt, "Build the frobnicator.") {
		t.Error("fix task prompt should contain spec content")
	}
}

func TestTaskPrompt_ConventionsAppearBeforeTask(t *testing.T) {
	task := runstore.Task{TaskID: "t-ctx-3", Objective: "do work"}
	tc := TaskContext{ProjectConventions: "conventions here"}
	prompt := renderTaskPrompt(task, tc, "")

	convIdx := strings.Index(prompt, "Project Conventions")
	taskIdx := strings.Index(prompt, "## Task:")

	if convIdx == -1 || taskIdx == -1 {
		t.Fatalf("missing sections: conv=%d task=%d", convIdx, taskIdx)
	}
	if convIdx > taskIdx {
		t.Error("Project Conventions should appear before the task header")
	}
}

func TestTaskPrompt_FixTaskSpecAppearsBeforeTask(t *testing.T) {
	task := runstore.Task{TaskID: "t-ctx-3b", Kind: "fix", Objective: "fix work"}
	tc := TaskContext{
		ProjectConventions: "conventions here",
		SpecContent:        "spec here",
	}
	prompt := renderTaskPrompt(task, tc, "")

	convIdx := strings.Index(prompt, "Project Conventions")
	specIdx := strings.Index(prompt, "Full Spec")
	taskIdx := strings.Index(prompt, "## Fix Task:")

	if convIdx == -1 || specIdx == -1 || taskIdx == -1 {
		t.Fatalf("missing sections: conv=%d spec=%d task=%d", convIdx, specIdx, taskIdx)
	}
	if convIdx > taskIdx {
		t.Error("Project Conventions should appear before the task header")
	}
	if specIdx > taskIdx {
		t.Error("Full Spec should appear before the task header")
	}
}

func TestTaskPrompt_EmptyContextOmitsSections(t *testing.T) {
	task := runstore.Task{TaskID: "t-ctx-4", Objective: "do work"}
	tc := TaskContext{} // empty
	prompt := renderTaskPrompt(task, tc, "")

	if strings.Contains(prompt, "Project Conventions") {
		t.Error("prompt should not contain 'Project Conventions' when empty")
	}
	if strings.Contains(prompt, "Full Spec") {
		t.Error("prompt should not contain 'Full Spec' when empty")
	}
	if !strings.Contains(prompt, "do work") {
		t.Error("prompt should still contain the objective")
	}
}

func TestRenderRepairPrompt_IncludesContext(t *testing.T) {
	task := runstore.Task{TaskID: "t-ctx-5", Objective: "fix bug"}
	tc := TaskContext{ProjectConventions: "conventions", SpecContent: "spec text"}
	prompt := renderRepairPrompt(task, []string{"test failed"}, tc, "")

	if !strings.Contains(prompt, "Project Conventions") {
		t.Error("repair prompt does not contain 'Project Conventions'")
	}
	if !strings.Contains(prompt, "Full Spec") {
		t.Error("repair prompt does not contain 'Full Spec'")
	}
	if !strings.Contains(prompt, "test failed") {
		t.Error("repair prompt does not contain failure")
	}
}

func TestProviderTaskRunner_ContextProviderWired_OriginalTask(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	runner.SetContextProvider(func() TaskContext {
		return TaskContext{
			ProjectConventions: "always write tests",
			SpecContent:        "build the widget",
		}
	})
	task := runstore.Task{TaskID: "t-ctx-6", Objective: "implement"}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(inv.capturedPrompt, "always write tests") {
		t.Error("prompt does not contain project conventions from context provider")
	}
	if strings.Contains(inv.capturedPrompt, "build the widget") {
		t.Error("original task prompt should not contain spec content")
	}
}

func TestProviderTaskRunner_ContextProviderWired_FixTask(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	runner.SetContextProvider(func() TaskContext {
		return TaskContext{
			ProjectConventions: "always write tests",
			SpecContent:        "build the widget",
		}
	})
	task := runstore.Task{TaskID: "t-ctx-6b", Kind: "fix", Objective: "fix it"}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(inv.capturedPrompt, "always write tests") {
		t.Error("prompt does not contain project conventions from context provider")
	}
	if !strings.Contains(inv.capturedPrompt, "build the widget") {
		t.Error("fix task prompt should contain spec content from context provider")
	}
}

func TestProviderTaskRunner_NoContextProviderOmitsSections(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	// No SetContextProvider call — default nil
	task := runstore.Task{TaskID: "t-ctx-7", Objective: "implement"}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(inv.capturedPrompt, "Project Conventions") {
		t.Error("prompt should not contain 'Project Conventions' without context provider")
	}
	if strings.Contains(inv.capturedPrompt, "Full Spec") {
		t.Error("prompt should not contain 'Full Spec' without context provider")
	}
}

func TestFileTaskContextProvider_ReadsFiles(t *testing.T) {
	workDir := t.TempDir()
	runDir := t.TempDir()

	// Create CLAUDE.md
	os.WriteFile(filepath.Join(workDir, "CLAUDE.md"), []byte("# Project Rules\nBe excellent."), 0o644)
	// Create spec.md
	os.WriteFile(filepath.Join(runDir, "spec.md"), []byte("# Spec 42\nDo the thing."), 0o644)

	provider := FileTaskContextProvider(func() string { return workDir }, runDir, "")
	tc := provider()

	if !strings.Contains(tc.ProjectConventions, "Be excellent.") {
		t.Errorf("expected CLAUDE.md content, got %q", tc.ProjectConventions)
	}
	if !strings.Contains(tc.SpecContent, "Do the thing.") {
		t.Errorf("expected spec.md content, got %q", tc.SpecContent)
	}
}

func TestFileTaskContextProvider_MissingFilesGraceful(t *testing.T) {
	emptyDir := t.TempDir()

	provider := FileTaskContextProvider(func() string { return emptyDir }, emptyDir, "")
	tc := provider()

	if tc.ProjectConventions != "" {
		t.Errorf("expected empty ProjectConventions, got %q", tc.ProjectConventions)
	}
	if tc.SpecContent != "" {
		t.Errorf("expected empty SpecContent, got %q", tc.SpecContent)
	}
}

func TestFileTaskContextProvider_EmptyWorkDir(t *testing.T) {
	runDir := t.TempDir()
	os.WriteFile(filepath.Join(runDir, "spec.md"), []byte("spec content"), 0o644)

	provider := FileTaskContextProvider(func() string { return "" }, runDir, "")
	tc := provider()

	if tc.ProjectConventions != "" {
		t.Errorf("expected empty ProjectConventions when workDir is empty, got %q", tc.ProjectConventions)
	}
	if tc.SpecContent != "spec content" {
		t.Errorf("expected spec content, got %q", tc.SpecContent)
	}
}

func TestShellTaskInspector_LazyWorkDirResolution(t *testing.T) {
	// Use two different temp dirs to verify the function is called each time.
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	currentDir := dir1
	inspector := NewShellTaskInspector(func() string { return currentDir })

	task := runstore.Task{
		TaskID:      "t-lazy-inspect",
		ProofChecks: []string{"true"},
	}

	// First call uses dir1.
	result := inspector.Inspect(context.Background(), task)
	if !result.Pass {
		t.Error("expected pass with 'true' command")
	}

	// Switch to dir2 and verify it still works (proves func is called lazily).
	currentDir = dir2
	result = inspector.Inspect(context.Background(), task)
	if !result.Pass {
		t.Error("expected pass with 'true' command after switching dir")
	}
}

// --- renderCurrentFileContents tests ---

func TestTaskPrompt_IncludesExistingFileContents(t *testing.T) {
	workDir := t.TempDir()

	// Create a file that exists in the worktree.
	os.MkdirAll(filepath.Join(workDir, "internal", "frob"), 0o755)
	os.WriteFile(filepath.Join(workDir, "internal", "frob", "frob.go"), []byte("package frob\n\nfunc Frobnicate() {}\n"), 0o644)

	task := runstore.Task{
		TaskID:              "t-file-contents",
		Objective:           "edit the frobnicator",
		ExpectedTouchedArea: []string{"internal/frob/frob.go", "internal/frob/new_file.go"},
	}
	tc := TaskContext{}
	prompt := renderTaskPrompt(task, tc, workDir)

	// Existing file should appear in the prompt.
	if !strings.Contains(prompt, "Current File Contents") {
		t.Error("prompt does not contain 'Current File Contents' header")
	}
	if !strings.Contains(prompt, "internal/frob/frob.go") {
		t.Error("prompt does not contain file path header for existing file")
	}
	if !strings.Contains(prompt, "func Frobnicate()") {
		t.Error("prompt does not contain content of existing file")
	}

	// Non-existent file should NOT appear in the file contents section.
	// (It will appear in Expected Touched Area, so check specifically for a code fence with its content.)
	fileContentsIdx := strings.Index(prompt, "Current File Contents")
	afterContents := prompt[fileContentsIdx:]
	if strings.Contains(afterContents, "new_file.go") {
		t.Error("Current File Contents section should not contain non-existent file")
	}
}

func TestRenderCurrentFileContents_TruncatesLongFiles(t *testing.T) {
	workDir := t.TempDir()

	// Create a file with more than maxFilePreviewLines lines.
	var lines []string
	for i := 0; i < maxFilePreviewLines+50; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	os.WriteFile(filepath.Join(workDir, "big.go"), []byte(strings.Join(lines, "\n")), 0o644)

	var b strings.Builder
	renderCurrentFileContents(&b, []string{"big.go"}, workDir)
	output := b.String()

	if !strings.Contains(output, "... (truncated)") {
		t.Error("long file should be truncated with indicator")
	}
	// Line 0 should be present, line at maxFilePreviewLines+40 should not.
	if !strings.Contains(output, "line 0") {
		t.Error("first line should be present")
	}
	if strings.Contains(output, fmt.Sprintf("line %d", maxFilePreviewLines+40)) {
		t.Error("lines beyond maxFilePreviewLines should not be present")
	}
}

func TestRenderCurrentFileContents_SkipsDirectories(t *testing.T) {
	workDir := t.TempDir()
	os.MkdirAll(filepath.Join(workDir, "internal", "frob"), 0o755)

	var b strings.Builder
	renderCurrentFileContents(&b, []string{"internal/frob/"}, workDir)
	output := b.String()

	if output != "" {
		t.Errorf("expected empty output for directory-only areas, got %q", output)
	}
}

func TestRenderCurrentFileContents_EmptyWorkDir(t *testing.T) {
	var b strings.Builder
	renderCurrentFileContents(&b, []string{"some/file.go"}, "")
	if b.String() != "" {
		t.Error("expected empty output when workDir is empty")
	}
}

func TestRenderCurrentFileContents_EmptyAreas(t *testing.T) {
	var b strings.Builder
	renderCurrentFileContents(&b, []string{}, t.TempDir())
	if b.String() != "" {
		t.Error("expected empty output when areas is empty")
	}
}

// --- renderContextSections tests ---

func TestRenderContextSections_Doctrine(t *testing.T) {
	tc := TaskContext{
		Doctrine: "Always validate user input at system boundaries.\nLog all errors with sufficient context for debugging.",
	}

	var b strings.Builder
	renderContextSections(&b, tc, false)
	output := b.String()

	if !strings.Contains(output, "### Doctrine") {
		t.Error("output does not contain Doctrine section header")
	}
	if !strings.Contains(output, "Always validate user input at system boundaries") {
		t.Error("output does not contain doctrine content")
	}
}

func TestRenderContextSections_KnownGaps(t *testing.T) {
	testCases := []struct {
		name          string
		includeSpec   bool
		knownGaps     string
		shouldInclude bool
	}{
		{
			name:          "fix task with known gaps",
			includeSpec:   true,
			knownGaps:     "Database connection may fail under high load.",
			shouldInclude: true,
		},
		{
			name:          "original task with known gaps (should be omitted)",
			includeSpec:   false,
			knownGaps:     "Database connection may fail under high load.",
			shouldInclude: false,
		},
		{
			name:          "fix task without known gaps",
			includeSpec:   true,
			knownGaps:     "",
			shouldInclude: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := TaskContext{
				KnownGaps: tc.knownGaps,
			}

			var b strings.Builder
			renderContextSections(&b, ctx, tc.includeSpec)
			output := b.String()

			if tc.shouldInclude {
				if !strings.Contains(output, "### Known Validation Gaps") {
					t.Error("output should contain Known Validation Gaps section header")
				}
				if !strings.Contains(output, tc.knownGaps) {
					t.Error("output should contain known gaps content")
				}
			} else {
				if strings.Contains(output, "### Known Validation Gaps") {
					t.Error("output should not contain Known Validation Gaps section header")
				}
			}
		})
	}
}

func TestTaskPrompt_IncludesDoctrine(t *testing.T) {
	testCases := []struct {
		name string
		kind string
	}{
		{
			name: "original task includes doctrine",
			kind: "",
		},
		{
			name: "fix task includes doctrine",
			kind: "fix",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			doctrineText := "Cache invalidation is the hardest problem in computer science."
			taskContext := TaskContext{
				Doctrine: doctrineText,
			}
			task := runstore.Task{
				TaskID:    "t-doctrine-test",
				Objective: "implement something",
				Kind:      tc.kind,
			}

			prompt := renderTaskPrompt(task, taskContext, "")

			if !strings.Contains(prompt, "### Doctrine") {
				t.Error("prompt should contain Doctrine section header")
			}
			if !strings.Contains(prompt, doctrineText) {
				t.Error("prompt should contain doctrine text")
			}
		})
	}
}

func TestFileTaskContextProvider_LoadsDoctrine(t *testing.T) {
	workDir := t.TempDir()
	runDir := t.TempDir()
	cellPath := t.TempDir()
	doctrineDir := filepath.Join(cellPath, "doctrine")

	os.MkdirAll(doctrineDir, 0o755)

	// Create doctrine rules file
	doctrineJSON := `{
  "rules": [
    {
      "id": "rule-1",
      "summary": "Use meaningful variable names",
      "scope": "*",
      "source": "declared",
      "created_at": "2026-03-01T00:00:00Z",
      "status": "active",
      "superseded_by": ""
    }
  ]
}`
	os.WriteFile(filepath.Join(doctrineDir, "rules.json"), []byte(doctrineJSON), 0o644)

	provider := FileTaskContextProvider(func() string { return workDir }, runDir, cellPath)
	tc := provider()

	if !strings.Contains(tc.Doctrine, "Use meaningful variable names") {
		t.Errorf("expected doctrine content, got %q", tc.Doctrine)
	}
}

func TestFileTaskContextProvider_LoadsValidationGapPlaybook(t *testing.T) {
	workDir := t.TempDir()
	runDir := t.TempDir()
	cellPath := t.TempDir()
	playbookDir := filepath.Join(cellPath, "playbook")

	os.MkdirAll(playbookDir, 0o755)

	// Create playbook entries file with validation_gap type
	playbookJSON := `[
  {
    "id": "pb-abc123def",
    "type": "validation_gap",
    "title": "Missing test coverage",
    "content": "Tests need coverage for edge cases",
    "rationale": "Prevent regressions",
    "status": "active",
    "source_proposal_id": "proposal-1",
    "source_run_id": "",
    "source_spec_id": "",
    "created_at": "2026-03-01T00:00:00Z",
    "superseded_by": ""
  },
  {
    "id": "pb-def456ghi",
    "type": "doctrine_rule",
    "title": "Code style",
    "content": "Follow Go conventions",
    "rationale": "Consistency",
    "status": "active",
    "source_proposal_id": "proposal-2",
    "source_run_id": "",
    "source_spec_id": "",
    "created_at": "2026-03-01T00:00:00Z",
    "superseded_by": ""
  }
]`
	os.WriteFile(filepath.Join(playbookDir, "entries.json"), []byte(playbookJSON), 0o644)

	provider := FileTaskContextProvider(func() string { return workDir }, runDir, cellPath)
	tc := provider()

	// Should only include validation_gap, not doctrine_rule
	if !strings.Contains(tc.KnownGaps, "Missing test coverage") {
		t.Errorf("expected validation_gap content, got %q", tc.KnownGaps)
	}
	if strings.Contains(tc.KnownGaps, "Code style") {
		t.Errorf("should not include non-validation_gap entries, got %q", tc.KnownGaps)
	}
}

func TestTaskPrompt_ValidationGaps(t *testing.T) {
	workDir := t.TempDir()
	runDir := t.TempDir()
	cellPath := t.TempDir()
	playbookDir := filepath.Join(cellPath, "playbook")

	os.MkdirAll(playbookDir, 0o755)

	// Create playbook entries with active validation_gap
	playbookJSON := `[
  {
    "id": "pb-gap001",
    "type": "validation_gap",
    "title": "Error handling incomplete",
    "content": "Add error recovery for network failures",
    "rationale": "Improve robustness",
    "status": "active",
    "source_proposal_id": "proposal-1",
    "source_run_id": "",
    "source_spec_id": "",
    "created_at": "2026-03-01T00:00:00Z",
    "superseded_by": ""
  }
]`
	os.WriteFile(filepath.Join(playbookDir, "entries.json"), []byte(playbookJSON), 0o644)

	ctxProvider := FileTaskContextProvider(func() string { return workDir }, runDir, cellPath)

	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return workDir })
	runner.SetContextProvider(ctxProvider)

	// Fix tasks should include validation gaps in prompt
	task := runstore.Task{
		TaskID:    "t-fix-1",
		Objective: "fix the error handling",
		Kind:      "fix",
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(inv.capturedPrompt, "### Known Validation Gaps") {
		t.Error("prompt should contain 'Known Validation Gaps' section header for fix task")
	}
	if !strings.Contains(inv.capturedPrompt, "Error handling incomplete") {
		t.Error("prompt should contain validation gap title")
	}
	if !strings.Contains(inv.capturedPrompt, "Add error recovery for network failures") {
		t.Error("prompt should contain validation gap content")
	}
	if !strings.Contains(inv.capturedPrompt, "Improve robustness") {
		t.Error("prompt should contain validation gap rationale")
	}
}

func TestRenderContextSections_SupersededExcluded(t *testing.T) {
	workDir := t.TempDir()
	runDir := t.TempDir()
	cellPath := t.TempDir()
	playbookDir := filepath.Join(cellPath, "playbook")

	os.MkdirAll(playbookDir, 0o755)

	// Create playbook entries with both active and superseded validation gaps
	playbookJSON := `[
  {
    "id": "pb-gap001",
    "type": "validation_gap",
    "title": "Error handling incomplete",
    "content": "Add error recovery for network failures",
    "rationale": "Improve robustness",
    "status": "active",
    "source_proposal_id": "proposal-1",
    "source_run_id": "",
    "source_spec_id": "",
    "created_at": "2026-03-01T00:00:00Z",
    "superseded_by": ""
  },
  {
    "id": "pb-gap002",
    "type": "validation_gap",
    "title": "Old logging gap",
    "content": "Previously identified logging issue",
    "rationale": "Resolved in later iteration",
    "status": "active",
    "source_proposal_id": "proposal-2",
    "source_run_id": "",
    "source_spec_id": "",
    "created_at": "2026-03-01T00:00:00Z",
    "superseded_by": "pb-gap001"
  }
]`
	os.WriteFile(filepath.Join(playbookDir, "entries.json"), []byte(playbookJSON), 0o644)

	ctxProvider := FileTaskContextProvider(func() string { return workDir }, runDir, cellPath)

	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return workDir })
	runner.SetContextProvider(ctxProvider)

	task := runstore.Task{
		TaskID:    "t-fix-1",
		Objective: "fix the error handling",
		Kind:      "fix",
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Active gap should be included
	if !strings.Contains(inv.capturedPrompt, "Error handling incomplete") {
		t.Error("prompt should contain active validation gap")
	}

	// Superseded gap should NOT be included
	if strings.Contains(inv.capturedPrompt, "Old logging gap") {
		t.Error("prompt should NOT contain superseded validation gap")
	}
	if strings.Contains(inv.capturedPrompt, "Previously identified logging issue") {
		t.Error("prompt should NOT contain superseded gap content")
	}
}

func TestRenderContextSections_ValidationGapsOnlyInRepairContext(t *testing.T) {
	validationGapText := "- **Test gap**: Missing test coverage"

	tests := []struct {
		name        string
		includeSpec bool
		expectGaps  bool
	}{
		{
			name:        "includeSpec=true (repair task)",
			includeSpec: true,
			expectGaps:  true,
		},
		{
			name:        "includeSpec=false (regular task)",
			includeSpec: false,
			expectGaps:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taskContext := TaskContext{
				KnownGaps: validationGapText,
			}
			var b strings.Builder
			renderContextSections(&b, taskContext, tc.includeSpec)
			prompt := b.String()

			if tc.expectGaps {
				if !strings.Contains(prompt, "### Known Validation Gaps") {
					t.Error("prompt should contain Known Validation Gaps section when includeSpec=true")
				}
				if !strings.Contains(prompt, validationGapText) {
					t.Error("prompt should contain validation gap text when includeSpec=true")
				}
			} else {
				if strings.Contains(prompt, "### Known Validation Gaps") {
					t.Error("prompt should not contain Known Validation Gaps section when includeSpec=false")
				}
			}
		})
	}
}

func TestFileTaskContextProvider_FiltersSuperseededDoctrineRules(t *testing.T) {
	workDir := t.TempDir()
	runDir := t.TempDir()
	cellPath := t.TempDir()
	doctrineDir := filepath.Join(cellPath, "doctrine")

	os.MkdirAll(doctrineDir, 0o755)

	// Create doctrine rules with both active and superseded rules
	doctrineJSON := `{
  "rules": [
    {
      "id": "rule-1",
      "summary": "Use meaningful variable names",
      "scope": "*",
      "source": "declared",
      "created_at": "2026-03-01T00:00:00Z",
      "status": "active",
      "superseded_by": ""
    },
    {
      "id": "rule-2",
      "summary": "Old naming convention",
      "scope": "*",
      "source": "declared",
      "created_at": "2026-03-01T00:00:00Z",
      "status": "active",
      "superseded_by": "rule-1"
    }
  ]
}`
	os.WriteFile(filepath.Join(doctrineDir, "rules.json"), []byte(doctrineJSON), 0o644)

	provider := FileTaskContextProvider(func() string { return workDir }, runDir, cellPath)
	tc := provider()

	// Active rule should be included
	if !strings.Contains(tc.Doctrine, "Use meaningful variable names") {
		t.Error("prompt should contain active doctrine rule")
	}

	// Superseded rule should NOT be included
	if strings.Contains(tc.Doctrine, "Old naming convention") {
		t.Error("prompt should NOT contain superseded doctrine rule")
	}
}

func TestFileTaskContextProvider_TreatsEmptyStatusAsActive(t *testing.T) {
	workDir := t.TempDir()
	runDir := t.TempDir()
	cellPath := t.TempDir()
	doctrineDir := filepath.Join(cellPath, "doctrine")

	os.MkdirAll(doctrineDir, 0o755)

	// Create doctrine rules with one having empty status (pre-existing rule) and one explicit active
	doctrineJSON := `{
  "rules": [
    {
      "id": "rule-legacy",
      "summary": "Pre-existing rule with no status field",
      "scope": "*",
      "source": "declared",
      "created_at": "2026-03-01T00:00:00Z",
      "status": "",
      "superseded_by": ""
    },
    {
      "id": "rule-new",
      "summary": "Explicitly active rule",
      "scope": "*",
      "source": "declared",
      "created_at": "2026-03-02T00:00:00Z",
      "status": "active",
      "superseded_by": ""
    }
  ]
}`
	os.WriteFile(filepath.Join(doctrineDir, "rules.json"), []byte(doctrineJSON), 0o644)

	provider := FileTaskContextProvider(func() string { return workDir }, runDir, cellPath)
	tc := provider()

	// Empty status rule should be included for backward compatibility
	if !strings.Contains(tc.Doctrine, "Pre-existing rule with no status field") {
		t.Error("prompt should contain pre-existing rule with empty status (backward compatibility)")
	}

	// Explicit active rule should also be included
	if !strings.Contains(tc.Doctrine, "Explicitly active rule") {
		t.Error("prompt should contain explicitly active rule")
	}
}

// TestApplyRunStateConstraints_NilRS verifies that ApplyRunStateConstraints returns
// the TaskContext unchanged when rs is nil.
func TestApplyRunStateConstraints_NilRS(t *testing.T) {
	tc := TaskContext{
		ProjectConventions: "some conventions",
	}
	result := ApplyRunStateConstraints(tc, nil)
	if result.ProjectConventions != tc.ProjectConventions {
		t.Error("ApplyRunStateConstraints with nil rs should return tc unchanged")
	}
	if len(result.ArchitectureConstraints) != 0 {
		t.Error("ApplyRunStateConstraints with nil rs should not set ArchitectureConstraints")
	}
}

// TestApplyRunStateConstraints_NilConstraints verifies that ApplyRunStateConstraints
// returns the TaskContext unchanged when constraints is nil.
func TestApplyRunStateConstraints_NilConstraints(t *testing.T) {
	tc := TaskContext{
		ProjectConventions: "some conventions",
	}
	result := ApplyRunStateConstraints(tc, nil)
	if result.ProjectConventions != tc.ProjectConventions {
		t.Error("ApplyRunStateConstraints with nil constraints should return tc unchanged")
	}
	if len(result.ArchitectureConstraints) != 0 {
		t.Error("ApplyRunStateConstraints with nil constraints should not set ArchitectureConstraints")
	}
}

// TestApplyRunStateConstraints_EmptyConstraints verifies that ApplyRunStateConstraints
// returns the TaskContext unchanged when constraints is empty.
func TestApplyRunStateConstraints_EmptyConstraints(t *testing.T) {
	tc := TaskContext{
		ProjectConventions: "some conventions",
	}
	result := ApplyRunStateConstraints(tc, []string{})
	if result.ProjectConventions != tc.ProjectConventions {
		t.Error("ApplyRunStateConstraints with empty constraints should return tc unchanged")
	}
	if len(result.ArchitectureConstraints) != 0 {
		t.Error("ApplyRunStateConstraints with empty constraints should not set ArchitectureConstraints")
	}
}

// TestApplyRunStateConstraints_PreExistingConstraintsPreserved verifies that
// ApplyRunStateConstraints preserves pre-existing constraints in TaskContext
// when the passed constraints slice is empty.
func TestApplyRunStateConstraints_PreExistingConstraintsPreserved(t *testing.T) {
	tc := TaskContext{ArchitectureConstraints: []string{"pre-existing"}}
	result := ApplyRunStateConstraints(tc, []string{})
	if len(result.ArchitectureConstraints) != 1 {
		t.Errorf("expected 1 constraint, got %d", len(result.ArchitectureConstraints))
	}
	if result.ArchitectureConstraints[0] != "pre-existing" {
		t.Errorf("expected 'pre-existing', got %q", result.ArchitectureConstraints[0])
	}
}

// TestApplyRunStateConstraints_NonEmptyConstraints verifies that ApplyRunStateConstraints
// appends constraints when tc has no pre-existing constraints.
func TestApplyRunStateConstraints_NonEmptyConstraints(t *testing.T) {
	tc := TaskContext{
		ProjectConventions: "some conventions",
	}
	constraints := []string{"use NormalizeNilFields for cross-package types", "separate validation in haiku"}
	result := ApplyRunStateConstraints(tc, constraints)
	if result.ProjectConventions != tc.ProjectConventions {
		t.Error("ApplyRunStateConstraints should preserve ProjectConventions")
	}
	if len(result.ArchitectureConstraints) != len(constraints) {
		t.Errorf("ApplyRunStateConstraints should append ArchitectureConstraints; got %d, want %d", len(result.ArchitectureConstraints), len(constraints))
	}
	for i, c := range constraints {
		if result.ArchitectureConstraints[i] != c {
			t.Errorf("ApplyRunStateConstraints constraint mismatch at index %d: got %q, want %q", i, result.ArchitectureConstraints[i], c)
		}
	}
}

// TestApplyRunStateConstraints_MergeWithExisting verifies that ApplyRunStateConstraints
// merges constraints, appending and deduplicating.
func TestApplyRunStateConstraints_MergeWithExisting(t *testing.T) {
	tc := TaskContext{
		ProjectConventions:      "some conventions",
		ArchitectureConstraints: []string{"existing constraint 1", "existing constraint 2"},
	}
	constraints := []string{"existing constraint 2", "new constraint 3", "new constraint 4"}
	result := ApplyRunStateConstraints(tc, constraints)
	if result.ProjectConventions != tc.ProjectConventions {
		t.Error("ApplyRunStateConstraints should preserve ProjectConventions")
	}
	// Should have: existing 1, existing 2, new 3, new 4 (deduplicated)
	expected := []string{"existing constraint 1", "existing constraint 2", "new constraint 3", "new constraint 4"}
	if len(result.ArchitectureConstraints) != len(expected) {
		t.Errorf("ApplyRunStateConstraints should merge constraints with dedup; got %d, want %d", len(result.ArchitectureConstraints), len(expected))
	}
	for i, c := range expected {
		if result.ArchitectureConstraints[i] != c {
			t.Errorf("ApplyRunStateConstraints constraint mismatch at index %d: got %q, want %q", i, result.ArchitectureConstraints[i], c)
		}
	}
}

// TestProviderTaskRunner_RepairTask_WithArchitectureConstraints verifies that the
// repair prompt includes Architecture Conventions when they are present in TaskContext.
func TestProviderTaskRunner_RepairTask_WithArchitectureConstraints(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	runner.SetContextProvider(func() TaskContext {
		return TaskContext{
			ArchitectureConstraints: []string{"Config.Tier always receives a tier label", "LLMCompleter.Complete uses context.Context as first parameter"},
		}
	})
	task := runstore.Task{
		TaskID:    "t-repair-arch-1",
		Objective: "repair with conventions",
	}
	failures := []string{"type mismatch in Config.Tier", "missing context.Context parameter"}

	_, err := runner.RepairTask(context.Background(), task, failures)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(inv.capturedPrompt, "### Architecture Conventions") {
		t.Error("repair prompt does not contain '### Architecture Conventions' section header")
	}
	if !strings.Contains(inv.capturedPrompt, "- Config.Tier always receives a tier label") {
		t.Error("repair prompt does not contain first architecture constraint with '- ' prefix")
	}
	if !strings.Contains(inv.capturedPrompt, "- LLMCompleter.Complete uses context.Context as first parameter") {
		t.Error("repair prompt does not contain second architecture constraint with '- ' prefix")
	}
}

// TestProviderTaskRunner_RepairTask_WithoutArchitectureConstraints verifies that the
// repair prompt does NOT include an Architecture Conventions section when constraints are empty or nil.
func TestProviderTaskRunner_RepairTask_WithoutArchitectureConstraints(t *testing.T) {
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })

	// Test with nil ArchitectureConstraints (no context provider set)
	taskNil := runstore.Task{
		TaskID:    "t-repair-arch-2",
		Objective: "repair without constraints",
	}
	failuresNil := []string{"test failed"}

	_, err := runner.RepairTask(context.Background(), taskNil, failuresNil)
	if err != nil {
		t.Fatalf("unexpected error with nil constraints: %v", err)
	}

	if strings.Contains(inv.capturedPrompt, "### Architecture Conventions") {
		t.Error("repair prompt should not contain '### Architecture Conventions' when constraints are nil")
	}

	// Test with empty ArchitectureConstraints
	inv2 := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner2 := NewProviderTaskRunner(inv2, func() string { return "" })
	runner2.SetContextProvider(func() TaskContext {
		return TaskContext{
			ArchitectureConstraints: []string{},
		}
	})
	taskEmpty := runstore.Task{
		TaskID:    "t-repair-arch-3",
		Objective: "repair with empty constraints",
	}
	failuresEmpty := []string{"test failed"}

	_, err = runner2.RepairTask(context.Background(), taskEmpty, failuresEmpty)
	if err != nil {
		t.Fatalf("unexpected error with empty constraints: %v", err)
	}

	if strings.Contains(inv2.capturedPrompt, "### Architecture Conventions") {
		t.Error("repair prompt should not contain '### Architecture Conventions' when constraints are empty")
	}
}
