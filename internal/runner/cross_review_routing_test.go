package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestBeadContextHasBuildProviderField verifies that BeadContext has a
// BuildProvider string field that tracks which provider performed the build.
func TestBeadContextHasBuildProviderField(t *testing.T) {
	bc := &runtypes.BeadContext{
		BuildProvider: "claude",
	}

	if bc.BuildProvider != "claude" {
		t.Errorf("BeadContext.BuildProvider = %q, want %q", bc.BuildProvider, "claude")
	}

	bc.BuildProvider = "openai"
	if bc.BuildProvider != "openai" {
		t.Errorf("BeadContext.BuildProvider = %q after reassignment, want %q", bc.BuildProvider, "openai")
	}
}

// TestExecuteClaudeInvocationSetsBuildProvider verifies that after
// executeClaudeInvocation completes, BeadContext.BuildProvider is set
// to the name of the provider that performed the build.
func TestExecuteClaudeInvocationSetsBuildProvider(t *testing.T) {
	claudeProv := &mockProviderWithRouterTracking{
		name: "test-claude",
		streamRunResult: &provider.Result{
			Success: true,
			Model:   "sonnet",
			Output:  "build done",
		},
	}

	mockRouter := provider.NewSingleProviderRouter(claudeProv)

	cfg := &config.Config{}
	cfg.SetDefaults()

	var buf strings.Builder
	r := &Runner{
		cfg:     cfg,
		router:  mockRouter,
		invoker: newInvokerForTest(mockRouter, &buf, nil),
		output:  &buf,
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "test-1", Priority: 1},
		Result: &IterationResult{
			BeadID: "test-1",
		},
		Tier:        provider.TierMedium,
		BuildPrompt: "test prompt",
	}

	ctx := context.Background()
	_, _, _, _ = r.executeClaudeInvocation(ctx, bc)

	if bc.BuildProvider != "test-claude" {
		t.Errorf("bc.BuildProvider = %q after executeClaudeInvocation, want %q",
			bc.BuildProvider, "test-claude")
	}
}

// TestCrossReviewRoutesToOppositeProvider verifies the end-to-end behavior:
// when routing.phase_preferences.review is "cross" and the build was performed
// by provider "claude", the review is routed to "openai" (and vice versa).
func TestCrossReviewRoutesToOppositeProvider(t *testing.T) {
	// Track which provider the review was routed to
	var reviewProviderName string

	claudeProv := &mockProviderWithRouterTracking{
		name: "claude",
		runFn: func(ctx context.Context, p, tier string) (*provider.Result, error) {
			reviewProviderName = "claude"
			return &provider.Result{
				Success: true,
				Model:   "sonnet",
				Output:  `{"summary": "looks good", "issues": [], "suggestions": []}`,
			}, nil
		},
	}
	openaiProv := &mockProviderWithRouterTracking{
		name: "openai",
		runFn: func(ctx context.Context, p, tier string) (*provider.Result, error) {
			reviewProviderName = "openai"
			return &provider.Result{
				Success: true,
				Model:   "gpt-4o",
				Output:  `{"summary": "looks good", "issues": [], "suggestions": []}`,
			}, nil
		},
	}

	stateFile := &crossReviewMockStateFile{}

	// Router with "cross" review preference
	router := provider.NewRouter(
		map[string]provider.Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		map[string]string{
			"review": "cross",
			"build":  "claude",
		},
		map[string]int{"claude": 50, "openai": 50},
		0,
		stateFile,
	)

	cfg := &config.Config{
		Review: config.ReviewConfig{
			Enabled: true,
			Timeout: 60,
		},
		Routing: config.RoutingConfig{
			PhasePreferences: map[string]string{
				"review": "cross",
			},
		},
	}
	cfg.SetDefaults()

	mockRend := &mockPromptRenderer{
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review this code", nil
		},
	}

	var buf strings.Builder
	r := &Runner{
		cfg:      cfg,
		router:   router,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(commit string) (string, error) {
			return "diff --git a/code.go b/code.go\n+println(\"hello\")", nil
		},
	}

	b := &bead.Bead{ID: "test-1", Priority: 1}

	// The build was done by "claude" (model "sonnet"), so cross-review
	// should route to "openai".
	_, _ = r.runLightReview(context.Background(), b, nil, "abc123", "sonnet", 1, time.Time{}, "claude")

	if reviewProviderName != "openai" {
		t.Errorf("cross-review routed to %q, want %q (opposite of build provider 'claude')",
			reviewProviderName, "openai")
	}
}

// TestCrossReviewReverseDirection verifies that when the build was done by
// "openai", the cross-review routes to "claude".
func TestCrossReviewReverseDirection(t *testing.T) {
	var reviewProviderName string

	claudeProv := &mockProviderWithRouterTracking{
		name: "claude",
		runFn: func(ctx context.Context, p, tier string) (*provider.Result, error) {
			reviewProviderName = "claude"
			return &provider.Result{
				Success: true,
				Model:   "sonnet",
				Output:  `{"summary": "looks good", "issues": [], "suggestions": []}`,
			}, nil
		},
	}
	openaiProv := &mockProviderWithRouterTracking{
		name: "openai",
		runFn: func(ctx context.Context, p, tier string) (*provider.Result, error) {
			reviewProviderName = "openai"
			return &provider.Result{
				Success: true,
				Model:   "gpt-4o",
				Output:  `{"summary": "looks good", "issues": [], "suggestions": []}`,
			}, nil
		},
	}

	stateFile := &crossReviewMockStateFile{}

	router := provider.NewRouter(
		map[string]provider.Provider{
			"claude": claudeProv,
			"openai": openaiProv,
		},
		map[string]string{
			"review": "cross",
			"build":  "openai",
		},
		map[string]int{"claude": 50, "openai": 50},
		0,
		stateFile,
	)

	cfg := &config.Config{
		Review: config.ReviewConfig{
			Enabled: true,
			Timeout: 60,
		},
		Routing: config.RoutingConfig{
			PhasePreferences: map[string]string{
				"review": "cross",
			},
		},
	}
	cfg.SetDefaults()

	mockRend := &mockPromptRenderer{
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			return "review this code", nil
		},
	}

	var buf strings.Builder
	r := &Runner{
		cfg:      cfg,
		router:   router,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(commit string) (string, error) {
			return "diff --git a/code.go b/code.go\n+changes", nil
		},
	}

	b := &bead.Bead{ID: "test-2", Priority: 1}

	// Build was by "openai" (model "gpt-4o"), cross-review should go to "claude"
	_, _ = r.runLightReview(context.Background(), b, nil, "def456", "gpt-4o", 1, time.Time{}, "openai")

	if reviewProviderName != "claude" {
		t.Errorf("cross-review routed to %q, want %q (opposite of build provider 'openai')",
			reviewProviderName, "claude")
	}
}

// crossReviewMockStateFile implements provider.StateFile for cross-review tests
type crossReviewMockStateFile struct{}

func (m *crossReviewMockStateFile) IncrementProviderCount(prov string)                  {}
func (m *crossReviewMockStateFile) GetProviderCounts() map[string]int                   { return make(map[string]int) }
func (m *crossReviewMockStateFile) IsProviderAvailable(prov string) bool                { return true }
func (m *crossReviewMockStateFile) SetProviderUnavailable(prov string, until time.Time) {}
