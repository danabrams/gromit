package learnings

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
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

// TestFilterProvisional_ArchivesGeneric tests that generic learnings are archived
func TestFilterProvisional_ArchivesGeneric(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add two provisional learnings (without filter)
	f.provisional = []Learning{
		{
			Date:     time.Now(),
			BeadID:   "bead-1",
			Content:  "Always run tests",
			Category: CategoryPatterns,
			Hash:     hashContent("Always run tests"),
		},
		{
			Date:     time.Now(),
			BeadID:   "bead-2",
			Content:  "Use DRY principle",
			Category: CategoryPatterns,
			Hash:     hashContent("Use DRY principle"),
		},
	}

	// Create filter that marks all as generic
	filter := func(content string) (bool, error) {
		return true, nil // All generic
	}

	// Run batch filter
	hashes, err := f.FilterProvisional(filter, map[string]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return 2 evaluated hashes
	if len(hashes) != 2 {
		t.Errorf("expected 2 evaluated hashes, got %d", len(hashes))
	}

	// Both should be archived
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional learnings, got %d", len(f.provisional))
	}
	if len(f.archived) != 2 {
		t.Fatalf("expected 2 archived learnings, got %d", len(f.archived))
	}

	// Check archived content includes filter reason
	for _, archived := range f.archived {
		if !strings.Contains(archived.Content, "filtered: generic engineering advice") {
			t.Error("archived learning should contain filter reason")
		}
	}
}

// TestFilterProvisional_PreservesSpecific tests that specific learnings are preserved
func TestFilterProvisional_PreservesSpecific(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add two provisional learnings
	f.provisional = []Learning{
		{
			Date:     time.Now(),
			BeadID:   "bead-1",
			Content:  "Runner uses escalation chain",
			Category: CategoryPatterns,
			Hash:     hashContent("Runner uses escalation chain"),
		},
		{
			Date:     time.Now(),
			BeadID:   "bead-2",
			Content:  "Bead sizing must be under 2 files",
			Category: CategoryConventions,
			Hash:     hashContent("Bead sizing must be under 2 files"),
		},
	}

	// Create filter that marks all as specific
	filter := func(content string) (bool, error) {
		return false, nil // All specific
	}

	// Run batch filter
	hashes, err := f.FilterProvisional(filter, map[string]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return 2 evaluated hashes
	if len(hashes) != 2 {
		t.Errorf("expected 2 evaluated hashes, got %d", len(hashes))
	}

	// Both should remain in provisional
	if len(f.provisional) != 2 {
		t.Errorf("expected 2 provisional learnings, got %d", len(f.provisional))
	}
	if len(f.archived) != 0 {
		t.Errorf("expected 0 archived learnings, got %d", len(f.archived))
	}
}

// TestFilterProvisional_SkipsAlreadyFiltered tests that already-filtered hashes are skipped
func TestFilterProvisional_SkipsAlreadyFiltered(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	hash1 := hashContent("Learning 1")
	hash2 := hashContent("Learning 2")

	// Add two provisional learnings
	f.provisional = []Learning{
		{
			Date:     time.Now(),
			BeadID:   "bead-1",
			Content:  "Learning 1",
			Category: CategoryPatterns,
			Hash:     hash1,
		},
		{
			Date:     time.Now(),
			BeadID:   "bead-2",
			Content:  "Learning 2",
			Category: CategoryPatterns,
			Hash:     hash2,
		},
	}

	// Track how many times filter is called
	var callCount int
	filter := func(content string) (bool, error) {
		callCount++
		return false, nil
	}

	// Mark first learning as already filtered
	alreadyFiltered := map[string]bool{hash1: true}

	// Run batch filter
	hashes, err := f.FilterProvisional(filter, alreadyFiltered)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only return 1 evaluated hash (the second one)
	if len(hashes) != 1 {
		t.Errorf("expected 1 evaluated hash, got %d", len(hashes))
	}
	if len(hashes) > 0 && hashes[0] != hash2 {
		t.Errorf("expected hash2 to be evaluated, got %s", hashes[0])
	}

	// Filter should only be called once (for the non-skipped learning)
	if callCount != 1 {
		t.Errorf("expected filter to be called 1 time, got %d", callCount)
	}
}

// TestFilterProvisional_MixedResults tests filtering with mixed specific/generic results
func TestFilterProvisional_MixedResults(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add three provisional learnings
	f.provisional = []Learning{
		{
			Date:     time.Now(),
			BeadID:   "bead-1",
			Content:  "Always test",
			Category: CategoryPatterns,
			Hash:     hashContent("Always test"),
		},
		{
			Date:     time.Now(),
			BeadID:   "bead-2",
			Content:  "Runner escalation pattern",
			Category: CategoryPatterns,
			Hash:     hashContent("Runner escalation pattern"),
		},
		{
			Date:     time.Now(),
			BeadID:   "bead-3",
			Content:  "Use DRY",
			Category: CategoryPatterns,
			Hash:     hashContent("Use DRY"),
		},
	}

	// Create filter that marks specific ones as generic
	filter := func(content string) (bool, error) {
		// Mark first and third as generic, second as specific
		return strings.Contains(content, "test") || strings.Contains(content, "DRY"), nil
	}

	// Run batch filter
	hashes, err := f.FilterProvisional(filter, map[string]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return 3 evaluated hashes
	if len(hashes) != 3 {
		t.Errorf("expected 3 evaluated hashes, got %d", len(hashes))
	}

	// One should remain in provisional, two should be archived
	if len(f.provisional) != 1 {
		t.Errorf("expected 1 provisional learning, got %d", len(f.provisional))
	}
	if len(f.archived) != 2 {
		t.Fatalf("expected 2 archived learnings, got %d", len(f.archived))
	}

	// The remaining provisional should be the specific one
	if !strings.Contains(f.provisional[0].Content, "Runner escalation pattern") {
		t.Error("wrong learning remained in provisional")
	}
}

// TestFilterProvisional_ErrorHandling tests error propagation from filter function
func TestFilterProvisional_ErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add a provisional learning
	f.provisional = []Learning{
		{
			Date:     time.Now(),
			BeadID:   "bead-1",
			Content:  "Some content",
			Category: CategoryPatterns,
			Hash:     hashContent("Some content"),
		},
	}

	// Create filter that returns an error
	filter := func(content string) (bool, error) {
		return false, fmt.Errorf("filter error")
	}

	// Run batch filter - should fail
	_, err := f.FilterProvisional(filter, map[string]bool{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "filtering learning") {
		t.Errorf("expected error to mention 'filtering learning', got: %v", err)
	}
}

// TestFilterProvisional_NilFilter tests that nil filter is rejected
func TestFilterProvisional_NilFilter(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Run with nil filter
	_, err := f.FilterProvisional(nil, map[string]bool{})
	if err == nil {
		t.Fatal("expected error for nil filter, got nil")
	}
	if !strings.Contains(err.Error(), "filter function is nil") {
		t.Errorf("expected error about nil filter, got: %v", err)
	}
}

// TestFilterProvisional_NilFile tests that nil file is handled
func TestFilterProvisional_NilFile(t *testing.T) {
	var f *File
	filter := func(content string) (bool, error) {
		return false, nil
	}

	_, err := f.FilterProvisional(filter, map[string]bool{})
	if err == nil {
		t.Fatal("expected error for nil file, got nil")
	}
	if !strings.Contains(err.Error(), "learnings file is nil") {
		t.Errorf("expected error about nil file, got: %v", err)
	}
}

// TestFilterProvisional_EmptyProvisional tests filtering when no provisional learnings exist
func TestFilterProvisional_EmptyProvisional(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// No provisional learnings
	f.provisional = []Learning{}

	filter := func(content string) (bool, error) {
		return true, nil
	}

	// Run batch filter
	hashes, err := f.FilterProvisional(filter, map[string]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return empty list
	if len(hashes) != 0 {
		t.Errorf("expected 0 evaluated hashes, got %d", len(hashes))
	}
}

// TestFilterProvisional_SavesChanges tests that changes are persisted to disk
func TestFilterProvisional_SavesChanges(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add a provisional learning
	f.provisional = []Learning{
		{
			Date:     time.Now(),
			BeadID:   "bead-1",
			Content:  "Generic advice",
			Category: CategoryPatterns,
			Hash:     hashContent("Generic advice"),
		},
	}

	// Save initial state
	if err := f.Save(); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	// Create filter that marks as generic
	filter := func(content string) (bool, error) {
		return true, nil
	}

	// Run batch filter
	_, err := f.FilterProvisional(filter, map[string]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Load from disk to verify persistence
	f2, _ := NewFile(tmpDir)
	if err := f2.Load(); err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	// Should have 0 provisional and 1 archived
	if len(f2.provisional) != 0 {
		t.Errorf("expected 0 provisional learnings after reload, got %d", len(f2.provisional))
	}
	if len(f2.archived) != 1 {
		t.Errorf("expected 1 archived learning after reload, got %d", len(f2.archived))
	}
}
