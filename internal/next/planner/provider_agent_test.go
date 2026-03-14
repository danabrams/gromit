package planner

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/provider"
)

// mockInvoker satisfies llmadapter.Invoker for testing.
type mockInvoker struct {
	result *provider.Result
	err    error
	// capture what was passed
	calledWithPrompt string
}

func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	m.calledWithPrompt = prompt
	return m.result, m.err
}

func (m *mockInvoker) InvokeInDir(ctx context.Context, prompt string, _ string) (*provider.Result, error) {
	return m.Invoke(ctx, prompt)
}

func TestProviderPlanAgent_SuccessfulInvocation(t *testing.T) {
	mock := &mockInvoker{
		result: &provider.Result{
			Output:       "plan output here",
			InputTokens:  100,
			OutputTokens: 50,
			CostUSD:      0.005,
			Model:        "opus",
			Duration:     2500 * time.Millisecond,
		},
	}

	agent := NewProviderPlanAgent(mock, "sonnet")
	result, err := agent.Invoke(context.Background(), "generate a plan", "high")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Output != "plan output here" {
		t.Errorf("Output = %q, want %q", result.Output, "plan output here")
	}
	if result.TokensIn != 100 {
		t.Errorf("TokensIn = %d, want 100", result.TokensIn)
	}
	if result.TokensOut != 50 {
		t.Errorf("TokensOut = %d, want 50", result.TokensOut)
	}
	if result.Cost != 0.005 {
		t.Errorf("Cost = %f, want 0.005", result.Cost)
	}
	if result.Model != "opus" {
		t.Errorf("Model = %q, want %q", result.Model, "opus")
	}
	if result.Duration != 2500 {
		t.Errorf("Duration = %d, want 2500", result.Duration)
	}
	if mock.calledWithPrompt != "generate a plan" {
		t.Errorf("invoker received prompt %q, want %q", mock.calledWithPrompt, "generate a plan")
	}
}

func TestProviderPlanAgent_ErrorPropagation(t *testing.T) {
	expectedErr := errors.New("provider unavailable")
	mock := &mockInvoker{err: expectedErr}

	agent := NewProviderPlanAgent(mock, "sonnet")
	_, err := agent.Invoke(context.Background(), "some prompt", "medium")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestProviderPlanAgent_TierMismatchLogsOnce(t *testing.T) {
	mock := &mockInvoker{
		result: &provider.Result{
			Output:       "output",
			InputTokens:  10,
			OutputTokens: 5,
			CostUSD:      0.001,
			Model:        "sonnet",
			Duration:     100 * time.Millisecond,
		},
	}

	// Capture log output.
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	log.SetFlags(0)

	agent := NewProviderPlanAgent(mock, "sonnet")

	// Matching tier — no warning expected.
	_, err := agent.Invoke(context.Background(), "p1", "sonnet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no log for matching tier, got %q", buf.String())
	}

	// Mismatched tier — warning expected.
	_, err = agent.Invoke(context.Background(), "p2", "opus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logged := buf.String()
	if !strings.Contains(logged, "tier mismatch") {
		t.Errorf("expected tier mismatch log, got %q", logged)
	}
	if !strings.Contains(logged, `"opus"`) || !strings.Contains(logged, `"sonnet"`) {
		t.Errorf("expected log to mention both tiers, got %q", logged)
	}

	// Second mismatch — should NOT log again (sync.Once).
	buf.Reset()
	_, err = agent.Invoke(context.Background(), "p3", "haiku")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no second log (sync.Once), got %q", buf.String())
	}
}

func TestProviderPlanAgent_EmptyTierNoWarning(t *testing.T) {
	mock := &mockInvoker{
		result: &provider.Result{
			Output:       "output",
			InputTokens:  10,
			OutputTokens: 5,
			CostUSD:      0.001,
			Model:        "sonnet",
			Duration:     100 * time.Millisecond,
		},
	}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	log.SetFlags(0)

	agent := NewProviderPlanAgent(mock, "sonnet")

	// Empty tier should never trigger a warning.
	_, err := agent.Invoke(context.Background(), "p", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no log for empty tier, got %q", buf.String())
	}
}

func TestProviderPlanAgent_PartialResultPreservedOnError(t *testing.T) {
	expectedErr := errors.New("partial failure")
	mock := &mockInvoker{
		result: &provider.Result{
			Output:       "partial output",
			InputTokens:  200,
			OutputTokens: 75,
			CostUSD:      0.01,
			Model:        "sonnet",
			Duration:     1500 * time.Millisecond,
		},
		err: expectedErr,
	}

	agent := NewProviderPlanAgent(mock, "sonnet")
	result, err := agent.Invoke(context.Background(), "generate a plan", "high")

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if result.Output != "partial output" {
		t.Errorf("Output = %q, want %q", result.Output, "partial output")
	}
	if result.TokensIn != 200 {
		t.Errorf("TokensIn = %d, want 200", result.TokensIn)
	}
	if result.TokensOut != 75 {
		t.Errorf("TokensOut = %d, want 75", result.TokensOut)
	}
	if result.Cost != 0.01 {
		t.Errorf("Cost = %f, want 0.01", result.Cost)
	}
	if result.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", result.Model, "sonnet")
	}
}

func TestProviderPlanAgent_NilResultReturnsError(t *testing.T) {
	mock := &mockInvoker{
		result: nil,
		err:    nil,
	}

	agent := NewProviderPlanAgent(mock, "sonnet")
	_, err := agent.Invoke(context.Background(), "generate a plan", "high")

	if err == nil {
		t.Fatal("expected error for nil result, got nil")
	}
	if err.Error() != "planner: provider returned nil result" {
		t.Errorf("error = %q, want %q", err.Error(), "planner: provider returned nil result")
	}
}

func TestProviderPlanAgent_Invoke_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mock := &mockInvoker{
		result: nil,
		err:    context.Canceled,
	}

	agent := NewProviderPlanAgent(mock, "sonnet")
	_, err := agent.Invoke(ctx, "generate a plan", "high")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestProviderPlanAgent_CompileTimeInterfaceCheck(t *testing.T) {
	// This test verifies that ProviderPlanAgent satisfies Agent at compile time.
	// The actual compile-time check is: var _ Agent = (*ProviderPlanAgent)(nil)
	// in the implementation file. This test just confirms the type assertion works.
	var agent Agent = &ProviderPlanAgent{}
	_ = agent
}
