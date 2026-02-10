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

// TestPerformGapAnalysis_ReturnsClaudeOutput verifies the function returns Claude's output
func TestPerformGapAnalysis_ReturnsClaudeOutput(t *testing.T) {
	expectedOutput := "The epic is missing specs for: reporting, notifications, and audit logs."

	mockClient := &mockClaudeClient{
		runFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output:  expectedOutput,
			}, nil
		},
	}

	result, err := performGapAnalysis(mockClient, "haiku", "# Epic", []string{})
	if err != nil {
		t.Fatalf("performGapAnalysis failed: %v", err)
	}

	if result != expectedOutput {
		t.Errorf("expected output %q, got %q", expectedOutput, result)
	}
}

// TestBuildSpecSummaries_CreatesFormattedList verifies spec summaries are properly formatted
func TestBuildSpecSummaries_CreatesFormattedList(t *testing.T) {
	specs := []spec{
		{id: "auth-spec", title: "Authentication System"},
		{id: "api-spec", title: "REST API"},
	}

	summaries := buildSpecSummaries(specs)

	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	expected1 := "auth-spec: Authentication System"
	if summaries[0] != expected1 {
		t.Errorf("expected summary[0] %q, got %q", expected1, summaries[0])
	}

	expected2 := "api-spec: REST API"
	if summaries[1] != expected2 {
		t.Errorf("expected summary[1] %q, got %q", expected2, summaries[1])
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
