//go:build acceptance

package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestReviewAcceptanceTests_PassVerdict verifies that reviewAcceptanceTests
// returns a "pass" verdict when the review output contains "VERDICT: PASS".
func TestReviewAcceptanceTests_PassVerdict(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet

	var buf strings.Builder
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output:  "The tests look good.\n\nVERDICT: PASS",
			}, nil
		},
	}

	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRend := &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			return "review prompt", nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(fromCommit string) (string, error) {
			return "diff --git a/test.go b/test.go\n+func TestNew() {}", nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-1",
			Title:       "Add feature X",
			Description: "Implement feature X",
			ExpectedOutputs: []string{
				"Feature X is implemented",
				"Tests pass",
			},
		},
		StartCommit: "abc123",
	}

	verdict, err := r.reviewAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if verdict != "pass" {
		t.Errorf("expected verdict='pass', got %q", verdict)
	}
}

// TestReviewAcceptanceTests_FailVerdict verifies that reviewAcceptanceTests
// returns a "fail" verdict when the review output contains "VERDICT: FAIL".
func TestReviewAcceptanceTests_FailVerdict(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet

	var buf strings.Builder
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output:  "The tests are weak. They test existing behavior.\n\nVERDICT: FAIL\n\nTest 1 does not require new behavior.",
			}, nil
		},
	}

	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRend := &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			return "review prompt", nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(fromCommit string) (string, error) {
			return "diff --git a/test.go b/test.go\n+func TestExisting() {}", nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-2",
			Title:       "Add validation",
			Description: "Add input validation",
			ExpectedOutputs: []string{
				"Invalid inputs are rejected",
			},
		},
		StartCommit: "def456",
	}

	verdict, err := r.reviewAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if verdict != "fail" {
		t.Errorf("expected verdict='fail', got %q", verdict)
	}
}

// TestReviewAcceptanceTests_DefaultsToPassWhenNoVerdict verifies that
// parseReviewVerdict defaults to "pass" when output lacks a verdict marker.
func TestReviewAcceptanceTests_DefaultsToPassWhenNoVerdict(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet
	// Expected failure: parseReviewVerdict function does not exist yet

	var buf strings.Builder
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output:  "The tests look reasonable. No obvious issues.",
			}, nil
		},
	}

	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRend := &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			return "review prompt", nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(fromCommit string) (string, error) {
			return "diff --git a/test.go b/test.go\n+func TestNew() {}", nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-3",
			Title:       "Add helper",
			Description: "Add helper function",
			ExpectedOutputs: []string{
				"Helper function exists",
			},
		},
		StartCommit: "ghi789",
	}

	verdict, err := r.reviewAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should default to pass
	if verdict != "pass" {
		t.Errorf("expected default verdict='pass', got %q", verdict)
	}
}

// TestReviewAcceptanceTests_UsesHaikuTier verifies that the review
// invocation uses TierLow (haiku) regardless of bead priority.
func TestReviewAcceptanceTests_UsesHaikuTier(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet

	var buf strings.Builder
	var capturedTier string

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			capturedTier = model
			return &claude.Result{
				Success: true,
				Output:  "VERDICT: PASS",
			}, nil
		},
	}

	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRend := &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			return "review prompt", nil
		},
	}

	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: "opus",
			P1: "sonnet",
			P2: "haiku",
		},
	}
	cfg.SetDefaults()

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(fromCommit string) (string, error) {
			return "diff --git a/test.go b/test.go\n+func TestNew() {}", nil
		},
	}

	// Use a P0 bead (would normally use opus)
	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-4",
			Title:       "Critical feature",
			Description: "Critical P0 feature",
			Priority:    0,
			ExpectedOutputs: []string{
				"Feature works",
			},
		},
		StartCommit: "jkl012",
		Model:       "opus",
	}

	_, err := r.reviewAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use haiku (low tier) regardless of bead priority
	if capturedTier != provider.TierLow && capturedTier != "haiku" {
		t.Errorf("expected tier='%s' or model='haiku', got %q", provider.TierLow, capturedTier)
	}
}

// TestReviewAcceptanceTests_BuildsContextFromBead verifies that
// reviewAcceptanceTests constructs ReviewAcceptanceTestsContext from bead data.
func TestReviewAcceptanceTests_BuildsContextFromBead(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet
	// Expected failure: ReviewAcceptanceTestsContext type does not exist yet

	var buf strings.Builder
	var capturedContext *prompt.ReviewAcceptanceTestsContext

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output:  "VERDICT: PASS",
			}, nil
		},
	}

	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRend := &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			capturedContext = ctx
			return "review prompt", nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	testDiff := "diff --git a/test.go b/test.go\n+func TestFeature() {}"

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(fromCommit string) (string, error) {
			return testDiff, nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-5",
			Title:       "Add logging",
			Description: "Add structured logging",
			ExpectedOutputs: []string{
				"Logs are structured",
				"Log levels work",
			},
		},
		StartCommit: "mno345",
	}

	_, err := r.reviewAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedContext == nil {
		t.Fatal("expected RenderReviewAcceptanceTests to be called with context")
	}

	if capturedContext.BeadTitle != "Add logging" {
		t.Errorf("expected BeadTitle='Add logging', got %q", capturedContext.BeadTitle)
	}

	if capturedContext.BeadDescription != "Add structured logging" {
		t.Errorf("expected BeadDescription='Add structured logging', got %q", capturedContext.BeadDescription)
	}

	if !strings.Contains(capturedContext.AcceptanceCriteria, "Logs are structured") {
		t.Errorf("expected AcceptanceCriteria to contain 'Logs are structured', got %q", capturedContext.AcceptanceCriteria)
	}

	if !strings.Contains(capturedContext.AcceptanceCriteria, "Log levels work") {
		t.Errorf("expected AcceptanceCriteria to contain 'Log levels work', got %q", capturedContext.AcceptanceCriteria)
	}

	if capturedContext.TestDiff != testDiff {
		t.Errorf("expected TestDiff to match git diff output, got %q", capturedContext.TestDiff)
	}
}

// TestReviewAcceptanceTests_GetsDiffFromStartCommit verifies that
// reviewAcceptanceTests retrieves the git diff from bc.StartCommit.
func TestReviewAcceptanceTests_GetsDiffFromStartCommit(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet

	var buf strings.Builder
	var capturedFromCommit string

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output:  "VERDICT: PASS",
			}, nil
		},
	}

	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRend := &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			return "review prompt", nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(fromCommit string) (string, error) {
			capturedFromCommit = fromCommit
			return "test diff", nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-6",
			Title:       "Add metrics",
			Description: "Add metrics collection",
			ExpectedOutputs: []string{
				"Metrics are collected",
			},
		},
		StartCommit: "startabc123def",
	}

	_, err := r.reviewAcceptanceTests(context.Background(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedFromCommit != "startabc123def" {
		t.Errorf("expected gitDiffFn called with 'startabc123def', got %q", capturedFromCommit)
	}
}

// TestParseReviewVerdict_PassMarker verifies that parseReviewVerdict
// returns "pass" when output contains "VERDICT: PASS".
func TestParseReviewVerdict_PassMarker(t *testing.T) {
	// Expected failure: parseReviewVerdict function does not exist yet

	output := "The tests look good.\n\nVERDICT: PASS\n\nAll criteria covered."
	verdict := parseReviewVerdict(output)

	if verdict != "pass" {
		t.Errorf("expected verdict='pass', got %q", verdict)
	}
}

// TestParseReviewVerdict_FailMarker verifies that parseReviewVerdict
// returns "fail" when output contains "VERDICT: FAIL".
func TestParseReviewVerdict_FailMarker(t *testing.T) {
	// Expected failure: parseReviewVerdict function does not exist yet

	output := "VERDICT: FAIL\n\nTest 1 checks existing behavior."
	verdict := parseReviewVerdict(output)

	if verdict != "fail" {
		t.Errorf("expected verdict='fail', got %q", verdict)
	}
}

// TestParseReviewVerdict_NoMarker verifies that parseReviewVerdict
// defaults to "pass" when no verdict marker is present.
func TestParseReviewVerdict_NoMarker(t *testing.T) {
	// Expected failure: parseReviewVerdict function does not exist yet

	output := "The tests are well-written. Good coverage."
	verdict := parseReviewVerdict(output)

	if verdict != "pass" {
		t.Errorf("expected default verdict='pass', got %q", verdict)
	}
}

// TestParseReviewVerdict_CaseInsensitive verifies that parseReviewVerdict
// recognizes verdict markers case-insensitively.
func TestParseReviewVerdict_CaseInsensitive(t *testing.T) {
	// Expected failure: parseReviewVerdict function does not exist yet

	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "lowercase pass",
			output:   "verdict: pass",
			expected: "pass",
		},
		{
			name:     "uppercase fail",
			output:   "VERDICT: FAIL",
			expected: "fail",
		},
		{
			name:     "mixed case",
			output:   "Verdict: Pass",
			expected: "pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict := parseReviewVerdict(tt.output)
			if verdict != tt.expected {
				t.Errorf("expected verdict=%q, got %q", tt.expected, verdict)
			}
		})
	}
}

// TestReviewAcceptanceTests_PropagatesRendererError verifies that
// reviewAcceptanceTests returns an error when renderer fails.
func TestReviewAcceptanceTests_PropagatesRendererError(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet

	var buf strings.Builder
	mockRend := &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			return "", &mockError{msg: "template render failed"}
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	r := &Runner{
		cfg:      cfg,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(fromCommit string) (string, error) {
			return "test diff", nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-7",
			Title:       "Test",
			Description: "Test",
			ExpectedOutputs: []string{
				"Output",
			},
		},
		StartCommit: "abc123",
	}

	_, err := r.reviewAcceptanceTests(context.Background(), bc)
	if err == nil {
		t.Fatal("expected error when renderer fails")
	}

	if !strings.Contains(err.Error(), "template render failed") {
		t.Errorf("expected error about template render, got: %v", err)
	}
}

// TestReviewAcceptanceTests_PropagatesGitDiffError verifies that
// reviewAcceptanceTests returns an error when git diff fails.
func TestReviewAcceptanceTests_PropagatesGitDiffError(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet

	var buf strings.Builder
	cfg := &config.Config{}
	cfg.SetDefaults()

	r := &Runner{
		cfg:    cfg,
		output: &buf,
		gitDiffFn: func(fromCommit string) (string, error) {
			return "", &mockError{msg: "git diff failed"}
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-8",
			Title:       "Test",
			Description: "Test",
			ExpectedOutputs: []string{
				"Output",
			},
		},
		StartCommit: "abc123",
	}

	_, err := r.reviewAcceptanceTests(context.Background(), bc)
	if err == nil {
		t.Fatal("expected error when git diff fails")
	}

	if !strings.Contains(err.Error(), "git diff") {
		t.Errorf("expected error about git diff, got: %v", err)
	}
}

// TestReviewAcceptanceTests_PropagatesProviderError verifies that
// reviewAcceptanceTests returns an error when the provider invocation fails.
func TestReviewAcceptanceTests_PropagatesProviderError(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet

	var buf strings.Builder
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return nil, &mockError{msg: "provider timeout"}
		},
	}

	mockProvider := &mockProviderForProcess{claudeClient: mockClaude}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRend := &mockPromptRenderer{
		RenderReviewAcceptanceTestsFn: func(ctx *prompt.ReviewAcceptanceTestsContext) (string, error) {
			return "review prompt", nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(fromCommit string) (string, error) {
			return "test diff", nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:          "test-9",
			Title:       "Test",
			Description: "Test",
			ExpectedOutputs: []string{
				"Output",
			},
		},
		StartCommit: "abc123",
	}

	_, err := r.reviewAcceptanceTests(context.Background(), bc)
	if err == nil {
		t.Fatal("expected error when provider fails")
	}

	if !strings.Contains(err.Error(), "provider timeout") {
		t.Errorf("expected error about provider timeout, got: %v", err)
	}
}
