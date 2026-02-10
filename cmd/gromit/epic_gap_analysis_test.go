package main

import (
	"context"
	"strings"
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

// TestPerformGapAnalysis_IncludesEpicContentInPrompt verifies epic content is in the prompt
func TestPerformGapAnalysis_IncludesEpicContentInPrompt(t *testing.T) {
	var capturedPrompt string

	mockClient := &mockClaudeClient{
		runFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			capturedPrompt = prompt
			return &claude.Result{
				Success: true,
				Output:  "Gap analysis result",
			}, nil
		},
	}

	epicContent := "# Test Epic\n\nThis epic describes payment processing."
	specSummaries := []string{"Spec 1: Auth"}

	_, err := performGapAnalysis(mockClient, "haiku", epicContent, specSummaries)
	if err != nil {
		t.Fatalf("performGapAnalysis failed: %v", err)
	}

	if !strings.Contains(capturedPrompt, epicContent) {
		t.Errorf("prompt should contain epic content %q, got: %q", epicContent, capturedPrompt)
	}
}

// TestPerformGapAnalysis_IncludesSpecSummariesInPrompt verifies spec summaries are in the prompt
func TestPerformGapAnalysis_IncludesSpecSummariesInPrompt(t *testing.T) {
	var capturedPrompt string

	mockClient := &mockClaudeClient{
		runFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			capturedPrompt = prompt
			return &claude.Result{
				Success: true,
				Output:  "Gap analysis result",
			}, nil
		},
	}

	epicContent := "# Test Epic"
	specSummaries := []string{
		"auth-spec: Authentication System",
		"api-spec: REST API",
	}

	_, err := performGapAnalysis(mockClient, "haiku", epicContent, specSummaries)
	if err != nil {
		t.Fatalf("performGapAnalysis failed: %v", err)
	}

	for _, summary := range specSummaries {
		if !strings.Contains(capturedPrompt, summary) {
			t.Errorf("prompt should contain spec summary %q, got: %q", summary, capturedPrompt)
		}
	}
}

// TestPerformGapAnalysis_AsksWhatAreasNotCovered verifies the prompt asks about coverage gaps
func TestPerformGapAnalysis_AsksWhatAreasNotCovered(t *testing.T) {
	var capturedPrompt string

	mockClient := &mockClaudeClient{
		runFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			capturedPrompt = prompt
			return &claude.Result{
				Success: true,
				Output:  "Gap analysis result",
			}, nil
		},
	}

	epicContent := "# Test Epic"
	specSummaries := []string{"auth-spec: Auth"}

	_, err := performGapAnalysis(mockClient, "haiku", epicContent, specSummaries)
	if err != nil {
		t.Fatalf("performGapAnalysis failed: %v", err)
	}

	promptLower := strings.ToLower(capturedPrompt)
	hasGapQuestion := strings.Contains(promptLower, "not covered") ||
		strings.Contains(promptLower, "gap") ||
		strings.Contains(promptLower, "missing")

	if !hasGapQuestion {
		t.Errorf("prompt should ask about coverage gaps, got: %q", capturedPrompt)
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
