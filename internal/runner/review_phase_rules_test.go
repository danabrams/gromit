package runner

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/reviewpkg"
	"github.com/danabrams/gromit/internal/state"
)

// TestLightReviewUsesPhaseFilteredRules verifies that runLightReview passes
// phase-filtered rules (from LoadRulesForPhase("review")) to the ReviewContext,
// not the full unfiltered rules from LoadRules().
//
// Expected failure: runLightReview currently calls LoadRules() instead of
// LoadRulesForPhase("review"), so the ReviewContext.Rules will contain the
// full rules content rather than the review-phase-filtered content.
func TestLightReviewUsesPhaseFilteredRules(t *testing.T) {
	fullRules := "## Code Style <!-- phases: build, review -->\nUse go fmt\n\n## Process <!-- phases: build -->\nAlways run tests\n"
	reviewPhaseRules := "## Code Style\nUse go fmt\n"

	var capturedRules string

	mockProv := &mockProviderWithRouterTracking{
		name: "test-provider",
		runFn: func(ctx context.Context, p, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "sonnet",
				Output:  `{"summary": "ok", "issues": [], "suggestions": []}`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(mockProv)

	mockRend := &mockPromptRenderer{
		LoadRulesFn: func() (string, error) {
			return fullRules, nil
		},
		LoadRulesForPhaseFn: func(phase string) (string, error) {
			if phase == "review" {
				return reviewPhaseRules, nil
			}
			return fullRules, nil
		},
		RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
			capturedRules = ctx.Rules
			return "review prompt", nil
		},
	}

	cfg := &config.Config{
		Review: config.ReviewConfig{
			Enabled: true,
			Timeout: 60,
		},
	}
	cfg.SetDefaults()

	gitDiffFn := func(commit string) (string, error) {
		return "diff --git a/code.go b/code.go\n+println(\"hello\")", nil
	}

	reviewer := reviewpkg.NewReviewer(cfg, router, nil, mockRend, gitDiffFn, nil)

	b := &bead.Bead{ID: "test-1", Priority: 1, Title: "Test bead"}

	_, _ = reviewer.RunLight(context.Background(), b, nil, "abc123", "sonnet", 1, time.Time{}, "")

	// The rules passed to the review context should be the phase-filtered
	// version (excluding Process section), not the full rules.
	if capturedRules != reviewPhaseRules {
		t.Errorf("runLightReview passed wrong rules to ReviewContext.\ngot:  %q\nwant: %q", capturedRules, reviewPhaseRules)
	}
	if capturedRules == fullRules {
		t.Error("runLightReview passed full unfiltered rules instead of review-phase-filtered rules")
	}
}

// TestThoroughReviewUsesPhaseFilteredRules verifies that runThoroughReview
// passes phase-filtered rules (from LoadRulesForPhase("review")) to the
// ThoroughReviewContext, not the full unfiltered rules from LoadRules().
//
// Expected failure: runThoroughReview currently calls LoadRules() instead of
// LoadRulesForPhase("review"), so the ThoroughReviewContext.Rules will contain
// the full rules content rather than the review-phase-filtered content.
func TestThoroughReviewUsesPhaseFilteredRules(t *testing.T) {
	fullRules := "## Code Style <!-- phases: build, review -->\nUse go fmt\n\n## Process <!-- phases: build -->\nAlways run tests\n"
	reviewPhaseRules := "## Code Style\nUse go fmt\n"

	var capturedRules string

	mockProv := &mockProviderWithRouterTracking{
		name: "test-provider",
		runFn: func(ctx context.Context, p, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "opus",
				Output:  `{"summary": "ok", "issues": [], "suggestions": []}`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(mockProv)

	mockRend := &mockPromptRenderer{
		LoadRulesFn: func() (string, error) {
			return fullRules, nil
		},
		LoadRulesForPhaseFn: func(phase string) (string, error) {
			if phase == "review" {
				return reviewPhaseRules, nil
			}
			return fullRules, nil
		},
		RenderThoroughReviewFn: func(ctx *prompt.ThoroughReviewContext) (string, error) {
			capturedRules = ctx.Rules
			return "thorough review prompt", nil
		},
	}

	cfg := &config.Config{
		Review: config.ReviewConfig{
			Enabled: true,
			Thorough: config.ThoroughReviewConfig{
				Enabled:          true,
				Timeout:          60,
				Model:            "opus",
				EveryNIterations: 5,
			},
		},
	}
	cfg.SetDefaults()

	tmpDir := t.TempDir()
	sf, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state file: %v", err)
	}
	// Record a review commit so the function doesn't bail out early
	if err := sf.RecordReview("abc123", 1); err != nil {
		t.Fatalf("Failed to record review: %v", err)
	}

	gitDiffFn := func(commit string) (string, error) {
		return "diff --git a/code.go b/code.go\n+changes", nil
	}

	reviewer := reviewpkg.NewReviewer(cfg, router, nil, mockRend, gitDiffFn, nil)

	reviewer.RunThorough(context.Background(), sf, 5, time.Time{}, func() (string, error) {
		return "abc123", nil
	})

	// The rules passed to the thorough review context should be phase-filtered.
	if capturedRules != reviewPhaseRules {
		t.Errorf("runThoroughReview passed wrong rules to ThoroughReviewContext.\ngot:  %q\nwant: %q", capturedRules, reviewPhaseRules)
	}
	if capturedRules == fullRules {
		t.Error("runThoroughReview passed full unfiltered rules instead of review-phase-filtered rules")
	}
}

// TestReviewInvocationsCallLoadRulesForPhaseNotLoadRules verifies that review
// functions call LoadRulesForPhase("review") and do NOT call LoadRules().
// This ensures the phase-filtering path is used rather than the unfiltered path.
//
// Expected failure: Both runLightReview and runThoroughReview currently call
// LoadRules() rather than LoadRulesForPhase("review").
func TestReviewInvocationsCallLoadRulesForPhaseNotLoadRules(t *testing.T) {
	mockProv := &mockProviderWithRouterTracking{
		name: "test-provider",
		runFn: func(ctx context.Context, p, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "sonnet",
				Output:  `{"summary": "ok", "issues": [], "suggestions": []}`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(mockProv)

	t.Run("light review calls LoadRulesForPhase", func(t *testing.T) {
		var loadRulesCalled bool
		var loadRulesForPhaseCalled bool
		var loadRulesForPhaseArg string

		mockRend := &mockPromptRenderer{
			LoadRulesFn: func() (string, error) {
				loadRulesCalled = true
				return "full rules", nil
			},
			LoadRulesForPhaseFn: func(phase string) (string, error) {
				loadRulesForPhaseCalled = true
				loadRulesForPhaseArg = phase
				return "review rules", nil
			},
			RenderReviewFn: func(ctx *prompt.ReviewContext) (string, error) {
				return "review prompt", nil
			},
		}

		cfg := &config.Config{
			Review: config.ReviewConfig{Enabled: true, Timeout: 60},
		}
		cfg.SetDefaults()

		gitDiffFn := func(commit string) (string, error) {
			return "diff --git a/code.go b/code.go\n+code", nil
		}

		reviewer := reviewpkg.NewReviewer(cfg, router, nil, mockRend, gitDiffFn, nil)

		b := &bead.Bead{ID: "test-lr", Priority: 1, Title: "Test"}
		_, _ = reviewer.RunLight(context.Background(), b, nil, "abc123", "sonnet", 1, time.Time{}, "")

		if !loadRulesForPhaseCalled {
			t.Error("runLightReview did not call LoadRulesForPhase")
		}
		if loadRulesForPhaseArg != "review" {
			t.Errorf("LoadRulesForPhase called with phase %q, want %q", loadRulesForPhaseArg, "review")
		}
		if loadRulesCalled {
			t.Error("runLightReview called LoadRules() directly — should use LoadRulesForPhase(\"review\") instead")
		}
	})

	t.Run("thorough review calls LoadRulesForPhase", func(t *testing.T) {
		var loadRulesCalled bool
		var loadRulesForPhaseCalled bool
		var loadRulesForPhaseArg string

		mockRend := &mockPromptRenderer{
			LoadRulesFn: func() (string, error) {
				loadRulesCalled = true
				return "full rules", nil
			},
			LoadRulesForPhaseFn: func(phase string) (string, error) {
				loadRulesForPhaseCalled = true
				loadRulesForPhaseArg = phase
				return "review rules", nil
			},
			RenderThoroughReviewFn: func(ctx *prompt.ThoroughReviewContext) (string, error) {
				return "thorough review prompt", nil
			},
		}

		cfg := &config.Config{
			Review: config.ReviewConfig{
				Enabled: true,
				Thorough: config.ThoroughReviewConfig{
					Enabled:          true,
					Timeout:          60,
					Model:            "opus",
					EveryNIterations: 5,
				},
			},
		}
		cfg.SetDefaults()

		tmpDir := t.TempDir()
		sf, err := state.NewFile(tmpDir)
		if err != nil {
			t.Fatalf("Failed to create state file: %v", err)
		}
		if err := sf.RecordReview("abc123", 1); err != nil {
			t.Fatalf("Failed to record review: %v", err)
		}

		gitDiffFn := func(commit string) (string, error) {
			return "diff --git a/code.go b/code.go\n+code", nil
		}

		reviewer := reviewpkg.NewReviewer(cfg, router, nil, mockRend, gitDiffFn, nil)

		reviewer.RunThorough(context.Background(), sf, 5, time.Time{}, func() (string, error) {
			return "abc123", nil
		})

		if !loadRulesForPhaseCalled {
			t.Error("runThoroughReview did not call LoadRulesForPhase")
		}
		if loadRulesForPhaseArg != "review" {
			t.Errorf("LoadRulesForPhase called with phase %q, want %q", loadRulesForPhaseArg, "review")
		}
		if loadRulesCalled {
			t.Error("runThoroughReview called LoadRules() directly — should use LoadRulesForPhase(\"review\") instead")
		}
	})
}
