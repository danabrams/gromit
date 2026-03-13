package planner

import (
	"context"
	"errors"
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

	agent := NewProviderPlanAgent(mock)
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

	agent := NewProviderPlanAgent(mock)
	_, err := agent.Invoke(context.Background(), "some prompt", "medium")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestProviderPlanAgent_TierIgnored(t *testing.T) {
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

	agent := NewProviderPlanAgent(mock)
	prompt := "same prompt"

	// Call with different tiers — invoker should receive the same prompt regardless.
	for _, tier := range []string{"low", "medium", "high", "xhigh", ""} {
		mock.calledWithPrompt = ""
		result, err := agent.Invoke(context.Background(), prompt, tier)
		if err != nil {
			t.Fatalf("tier=%q: unexpected error: %v", tier, err)
		}
		if mock.calledWithPrompt != prompt {
			t.Errorf("tier=%q: invoker received prompt %q, want %q", tier, mock.calledWithPrompt, prompt)
		}
		if result.Output != "output" {
			t.Errorf("tier=%q: Output = %q, want %q", tier, result.Output, "output")
		}
	}
}

func TestProviderPlanAgent_CompileTimeInterfaceCheck(t *testing.T) {
	// This test verifies that ProviderPlanAgent satisfies Agent at compile time.
	// The actual compile-time check is: var _ Agent = (*ProviderPlanAgent)(nil)
	// in the implementation file. This test just confirms the type assertion works.
	var agent Agent = &ProviderPlanAgent{}
	_ = agent
}
