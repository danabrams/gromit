package learnings

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockClaudeRunner is a mock implementation of ClaudeRunner for testing
type mockClaudeRunner struct {
	// FnRun is called when Run() is invoked
	FnRun func(ctx context.Context, prompt string, model string) (*Result, error)
}

func (m *mockClaudeRunner) Run(ctx context.Context, prompt string, model string) (*Result, error) {
	if m.FnRun != nil {
		return m.FnRun(ctx, prompt, model)
	}
	return &Result{Success: true, Output: "specific"}, nil
}

// TestNewLLMFilter_Generic tests that generic learnings are filtered
func TestNewLLMFilter_Generic(t *testing.T) {
	runner := &mockClaudeRunner{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			return &Result{Success: true, Output: "generic"}, nil
		},
	}

	filter := NewLLMFilter(runner, "gromit", "A Go CLI tool for running Claude iterations")

	isGeneric, err := filter("Always verify tests pass before committing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isGeneric {
		t.Error("expected learning to be classified as generic")
	}
}

// TestNewLLMFilter_Specific tests that project-specific learnings are not filtered
func TestNewLLMFilter_Specific(t *testing.T) {
	runner := &mockClaudeRunner{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			return &Result{Success: true, Output: "specific"}, nil
		},
	}

	filter := NewLLMFilter(runner, "gromit", "A Go CLI tool for running Claude iterations")

	isGeneric, err := filter("The runner's escalation chain skips haiku when the bead has complexity:high label")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isGeneric {
		t.Error("expected learning to be classified as specific (not generic)")
	}
}

// TestNewLLMFilter_UsesHaiku tests that the filter uses the haiku model
func TestNewLLMFilter_UsesHaiku(t *testing.T) {
	var capturedModel string
	runner := &mockClaudeRunner{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			capturedModel = model
			return &Result{Success: true, Output: "specific"}, nil
		},
	}

	filter := NewLLMFilter(runner, "gromit", "A Go CLI tool")
	_, err := filter("Some learning content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedModel != "haiku" {
		t.Errorf("expected model 'haiku', got %q", capturedModel)
	}
}

// TestNewLLMFilter_PromptStructure tests that the prompt includes project context
func TestNewLLMFilter_PromptStructure(t *testing.T) {
	var capturedPrompt string
	runner := &mockClaudeRunner{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			capturedPrompt = prompt
			return &Result{Success: true, Output: "specific"}, nil
		},
	}

	filter := NewLLMFilter(runner, "test-project", "A test project description")
	content := "Test learning content"
	_, err := filter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify prompt contains project name, description, and learning content
	if !strings.Contains(capturedPrompt, "test-project") {
		t.Error("prompt should contain project name")
	}
	if !strings.Contains(capturedPrompt, "A test project description") {
		t.Error("prompt should contain project description")
	}
	if !strings.Contains(capturedPrompt, content) {
		t.Error("prompt should contain learning content")
	}
}

// TestNewLLMFilter_ErrorHandling tests error propagation from Claude runner
func TestNewLLMFilter_ErrorHandling(t *testing.T) {
	runner := &mockClaudeRunner{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			return nil, fmt.Errorf("claude invocation failed")
		},
	}

	filter := NewLLMFilter(runner, "gromit", "A Go CLI tool")
	_, err := filter("Some learning content")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "calling claude") {
		t.Errorf("expected error to mention 'calling claude', got: %v", err)
	}
}

// TestNewLLMFilter_NilRunner tests that nil runner is handled
func TestNewLLMFilter_NilRunner(t *testing.T) {
	filter := NewLLMFilter(nil, "gromit", "A Go CLI tool")
	_, err := filter("Some learning content")
	if err == nil {
		t.Fatal("expected error for nil runner, got nil")
	}
	if !strings.Contains(err.Error(), "claude runner is nil") {
		t.Errorf("expected error about nil runner, got: %v", err)
	}
}

// TestParseClassification_Specific tests parsing "specific" classification
func TestParseClassification_Specific(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{"exact match", "specific"},
		{"capitalized", "Specific"},
		{"with whitespace", "  specific  "},
		{"in sentence", "This learning is specific to the project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseClassification(tt.output)
			if result != "specific" {
				t.Errorf("expected 'specific', got %q", result)
			}
		})
	}
}

// TestParseClassification_Generic tests parsing "generic" classification
func TestParseClassification_Generic(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{"exact match", "generic"},
		{"capitalized", "Generic"},
		{"with whitespace", "  generic  "},
		{"in sentence", "This is generic engineering advice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseClassification(tt.output)
			if result != "generic" {
				t.Errorf("expected 'generic', got %q", result)
			}
		})
	}
}

// TestParseClassification_Unknown tests parsing ambiguous output
func TestParseClassification_Unknown(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{"empty", ""},
		{"unrelated", "I don't understand the question"},
		{"ambiguous", "It could be either specific or generic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseClassification(tt.output)
			if result != "unknown" {
				t.Errorf("expected 'unknown', got %q", result)
			}
		})
	}
}

// TestFileWithLLMFilter_Generic tests integration with File.Add() for generic learnings
func TestFileWithLLMFilter_Generic(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Configure filter to classify all learnings as generic
	runner := &mockClaudeRunner{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			return &Result{Success: true, Output: "generic"}, nil
		},
	}
	filter := NewLLMFilter(runner, "gromit", "A Go CLI tool")
	f.SetFilter(filter)

	// Add a learning
	learning, err := f.Add("bead-123", "Always run tests before committing", CategoryPatterns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Learning should be nil (filtered)
	if learning != nil {
		t.Error("expected learning to be nil (filtered)")
	}

	// Should be in archived section, not provisional
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional learnings, got %d", len(f.provisional))
	}
	if len(f.archived) != 1 {
		t.Fatalf("expected 1 archived learning, got %d", len(f.archived))
	}

	// Check archived learning content includes filter reason
	archived := f.archived[0]
	if !strings.Contains(archived.Content, "Always run tests before committing") {
		t.Error("archived learning should contain original content")
	}
	if !strings.Contains(archived.Content, "filtered: generic engineering advice") {
		t.Error("archived learning should contain filter reason")
	}
}

// TestFileWithLLMFilter_Specific tests integration with File.Add() for specific learnings
func TestFileWithLLMFilter_Specific(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Configure filter to classify all learnings as specific
	runner := &mockClaudeRunner{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			return &Result{Success: true, Output: "specific"}, nil
		},
	}
	filter := NewLLMFilter(runner, "gromit", "A Go CLI tool")
	f.SetFilter(filter)

	// Add a learning
	content := "The runner's escalation chain skips haiku for complexity:high beads"
	learning, err := f.Add("bead-456", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Learning should be added to provisional
	if learning == nil {
		t.Fatal("expected learning to be added")
	}
	if len(f.provisional) != 1 {
		t.Errorf("expected 1 provisional learning, got %d", len(f.provisional))
	}
	if len(f.archived) != 0 {
		t.Errorf("expected 0 archived learnings, got %d", len(f.archived))
	}
}

// TestFileWithLLMFilter_ErrorHandling tests that filter errors are propagated
func TestFileWithLLMFilter_ErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Configure filter to return an error
	runner := &mockClaudeRunner{
		FnRun: func(ctx context.Context, prompt string, model string) (*Result, error) {
			return nil, fmt.Errorf("claude API error")
		},
	}
	filter := NewLLMFilter(runner, "gromit", "A Go CLI tool")
	f.SetFilter(filter)

	// Add a learning - should fail with filter error
	_, err := f.Add("bead-789", "Some learning", CategoryPatterns)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "filter function error") {
		t.Errorf("expected filter function error, got: %v", err)
	}
}
