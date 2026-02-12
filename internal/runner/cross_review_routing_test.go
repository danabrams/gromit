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
)

// TestBeadContextHasBuildProviderField verifies that beadContext has a
// buildProvider string field that tracks which provider performed the build.
// Expected failure: beadContext does not have a buildProvider field yet
func TestBeadContextHasBuildProviderField(t *testing.T) {
	bc := &beadContext{
		buildProvider: "claude",
	}

	if bc.buildProvider != "claude" {
		t.Errorf("beadContext.buildProvider = %q, want %q", bc.buildProvider, "claude")
	}

	bc.buildProvider = "openai"
	if bc.buildProvider != "openai" {
		t.Errorf("beadContext.buildProvider = %q after reassignment, want %q", bc.buildProvider, "openai")
	}
}

// TestExecuteClaudeInvocationSetsBuildProvider verifies that after
// executeClaudeInvocation completes, beadContext.buildProvider is set
// to the name of the provider that performed the build.
// Expected failure: beadContext.buildProvider field does not exist yet,
// and executeClaudeInvocation does not set it
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
		cfg:    cfg,
		router: mockRouter,
		output: &buf,
	}

	bc := &beadContext{
		bead: &bead.Bead{ID: "test-1", Priority: 1},
		result: &IterationResult{
			BeadID: "test-1",
		},
		tier:        provider.TierMedium,
		buildPrompt: "test prompt",
	}

	ctx := context.Background()
	_, _, _, _ = r.executeClaudeInvocation(ctx, bc)

	// Expected failure: buildProvider is not set by executeClaudeInvocation
	if bc.buildProvider != "test-claude" {
		t.Errorf("bc.buildProvider = %q after executeClaudeInvocation, want %q",
			bc.buildProvider, "test-claude")
	}
}

// TestCrossReviewRoutesToOppositeProvider verifies the end-to-end behavior:
// when routing.phase_preferences.review is "cross" and the build was performed
// by provider "claude", the review is routed to "openai" (and vice versa).
//
// This tests through runLightReview which currently calls router.Select("review", tier).
// After implementation, it should detect the "cross" preference and call
// router.SelectCross(buildProvider, tier) instead.
//
// Expected failure: Router.SelectCross does not exist yet, runLightReview does
// not accept a buildProvider parameter and does not handle "cross" preference
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
// Expected failure: Router.SelectCross does not exist yet
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

func (m *crossReviewMockStateFile) IncrementProviderCount(prov string)                       {}
func (m *crossReviewMockStateFile) GetProviderCounts() map[string]int                        { return make(map[string]int) }
func (m *crossReviewMockStateFile) IsProviderAvailable(prov string) bool                     { return true }
func (m *crossReviewMockStateFile) SetProviderUnavailable(prov string, until time.Time)      {}
