package specloop

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/provider"
)

// mockInvoker captures the prompt and returns a configured result.
type mockInvoker struct {
	capturedPrompt string
	result         *provider.Result
	err            error
}

func (m *mockInvoker) Invoke(_ context.Context, prompt string) (*provider.Result, error) {
	m.capturedPrompt = prompt
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
	runner := NewProviderTaskRunner(inv)
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
