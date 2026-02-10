package main

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/claude"
)

// TestPerformGapAnalysis_CallsClaudeWithHaikuModel verifies that gap analysis uses haiku model
func TestPerformGapAnalysis_CallsClaudeWithHaikuModel(t *testing.T) {
	var capturedModel string

	mockClient := &mockClaudeClient{
		runFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			capturedModel = model
			return &claude.Result{
				Success: true,
				Output:  "Gap analysis result",
			}, nil
		},
	}

	epicContent := "# Test Epic\n\nThis epic describes features."
	specSummaries := []string{"Spec 1: Auth", "Spec 2: API"}

	_, err := performGapAnalysis(mockClient, "haiku", epicContent, specSummaries)
	if err != nil {
		t.Fatalf("performGapAnalysis failed: %v", err)
	}

	if capturedModel != "haiku" {
		t.Errorf("expected model 'haiku', got %q", capturedModel)
	}
}

// mockClaudeClient implements a minimal claude.Client interface for testing
type mockClaudeClient struct {
	runFn func(ctx context.Context, prompt string, model string) (*claude.Result, error)
}

func (m *mockClaudeClient) Run(ctx context.Context, prompt string, model string) (*claude.Result, error) {
	if m.runFn != nil {
		return m.runFn(ctx, prompt, model)
	}
	return &claude.Result{Success: true, Output: "mock output"}, nil
}
