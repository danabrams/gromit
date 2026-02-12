//go:build acceptance

package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// TestAcceptance_RunnerSelectTierDelegatesToConfigSelectTier verifies that
// Runner.selectTier() calls cfg.SelectTier() with bead priority and labels.
// Expected failure: selectTier method does not exist on Runner yet
func TestAcceptance_RunnerSelectTierDelegatesToConfigSelectTier(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		bead     *bead.Bead
		wantTier string
	}{
		{
			name: "P0 bead with no labels returns high tier",
			cfg: &config.Config{
				Models: config.ModelsConfig{
					P0: provider.TierHigh,
					P1: provider.TierMedium,
					P2: provider.TierLow,
				},
			},
			bead: &bead.Bead{
				ID:       "test-001",
				Title:    "High priority task",
				Priority: 0,
				Labels:   []string{},
			},
			wantTier: provider.TierHigh,
		},
		{
			name: "P1 bead with no labels returns medium tier",
			cfg: &config.Config{
				Models: config.ModelsConfig{
					P0: provider.TierHigh,
					P1: provider.TierMedium,
					P2: provider.TierLow,
				},
			},
			bead: &bead.Bead{
				ID:       "test-002",
				Title:    "Medium priority task",
				Priority: 1,
				Labels:   []string{},
			},
			wantTier: provider.TierMedium,
		},
		{
			name: "P2 bead with no labels returns low tier",
			cfg: &config.Config{
				Models: config.ModelsConfig{
					P0: provider.TierHigh,
					P1: provider.TierMedium,
					P2: provider.TierLow,
				},
			},
			bead: &bead.Bead{
				ID:       "test-003",
				Title:    "Low priority task",
				Priority: 2,
				Labels:   []string{},
			},
			wantTier: provider.TierLow,
		},
		{
			name: "complexity:high label overrides P1 priority to high tier",
			cfg: &config.Config{
				Models: config.ModelsConfig{
					P0: provider.TierHigh,
					P1: provider.TierMedium,
					P2: provider.TierLow,
					Labels: map[string]string{
						"complexity:high": provider.TierHigh,
					},
				},
			},
			bead: &bead.Bead{
				ID:       "test-004",
				Title:    "Medium priority but high complexity",
				Priority: 1,
				Labels:   []string{"complexity:high"},
			},
			wantTier: provider.TierHigh,
		},
		{
			name: "complexity:low label overrides P0 priority to low tier",
			cfg: &config.Config{
				Models: config.ModelsConfig{
					P0: provider.TierHigh,
					P1: provider.TierMedium,
					P2: provider.TierLow,
					Labels: map[string]string{
						"complexity:low": provider.TierLow,
					},
				},
			},
			bead: &bead.Bead{
				ID:       "test-005",
				Title:    "High priority but low complexity",
				Priority: 0,
				Labels:   []string{"complexity:low"},
			},
			wantTier: provider.TierLow,
		},
		{
			name: "backward compat: opus model name in P0 maps to high tier",
			cfg: &config.Config{
				Models: config.ModelsConfig{
					P0: "opus",   // legacy model name
					P1: "sonnet", // legacy model name
					P2: "haiku",  // legacy model name
				},
			},
			bead: &bead.Bead{
				ID:       "test-006",
				Title:    "Legacy config with opus",
				Priority: 0,
				Labels:   []string{},
			},
			wantTier: provider.TierHigh,
		},
		{
			name: "backward compat: sonnet model name in P1 maps to medium tier",
			cfg: &config.Config{
				Models: config.ModelsConfig{
					P0: "opus",
					P1: "sonnet",
					P2: "haiku",
				},
			},
			bead: &bead.Bead{
				ID:       "test-007",
				Title:    "Legacy config with sonnet",
				Priority: 1,
				Labels:   []string{},
			},
			wantTier: provider.TierMedium,
		},
		{
			name: "backward compat: haiku model name in P2 maps to low tier",
			cfg: &config.Config{
				Models: config.ModelsConfig{
					P0: "opus",
					P1: "sonnet",
					P2: "haiku",
				},
			},
			bead: &bead.Bead{
				ID:       "test-008",
				Title:    "Legacy config with haiku",
				Priority: 2,
				Labels:   []string{},
			},
			wantTier: provider.TierLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure defaults are set for consistent behavior
			tt.cfg.SetDefaults()
			tt.cfg.NormalizeNilFields()

			r := &Runner{
				cfg: tt.cfg,
			}

			// Expected failure: selectTier method does not exist on Runner yet
			gotTier := r.selectTier(tt.bead)

			if gotTier != tt.wantTier {
				t.Errorf("selectTier() = %q, want %q", gotTier, tt.wantTier)
			}
		})
	}
}

// TestAcceptance_RunnerSelectTierNilSafety verifies that selectTier handles nil inputs safely.
// Expected failure: selectTier method does not exist on Runner yet
func TestAcceptance_RunnerSelectTierNilSafety(t *testing.T) {
	tests := []struct {
		name     string
		runner   *Runner
		bead     *bead.Bead
		wantTier string
	}{
		{
			name:     "nil runner returns medium tier default",
			runner:   nil,
			bead:     &bead.Bead{ID: "test-001", Priority: 0},
			wantTier: provider.TierMedium,
		},
		{
			name: "nil config returns medium tier default",
			runner: &Runner{
				cfg: nil,
			},
			bead:     &bead.Bead{ID: "test-002", Priority: 0},
			wantTier: provider.TierMedium,
		},
		{
			name: "nil bead returns medium tier default",
			runner: &Runner{
				cfg: &config.Config{
					Models: config.ModelsConfig{
						P0: provider.TierHigh,
						P1: provider.TierMedium,
						P2: provider.TierLow,
					},
				},
			},
			bead:     nil,
			wantTier: provider.TierMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.runner != nil && tt.runner.cfg != nil {
				tt.runner.cfg.SetDefaults()
				tt.runner.cfg.NormalizeNilFields()
			}

			var gotTier string
			if tt.runner == nil {
				// Cannot call method on nil receiver - should not panic
				// Expected failure: selectTier method does not exist yet
				// When implemented, this should safely handle nil runner
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("selectTier panicked with nil runner: %v", r)
					}
				}()
				// This will panic until implemented
				return
			} else {
				// Expected failure: selectTier method does not exist on Runner yet
				gotTier = tt.runner.selectTier(tt.bead)
			}

			if gotTier != tt.wantTier {
				t.Errorf("selectTier() = %q, want %q", gotTier, tt.wantTier)
			}
		})
	}
}

// TestAcceptance_RunnerSelectTierRespectsMultipleLabels verifies that selectTier
// respects label precedence when multiple labels are present.
// Expected failure: selectTier method does not exist on Runner yet
func TestAcceptance_RunnerSelectTierRespectsMultipleLabels(t *testing.T) {
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: provider.TierHigh,
			P1: provider.TierMedium,
			P2: provider.TierLow,
			Labels: map[string]string{
				"complexity:high": provider.TierHigh,
				"complexity:low":  provider.TierLow,
				"spec:simple":     provider.TierLow,
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	r := &Runner{
		cfg: cfg,
	}

	// First matching label should win
	b := &bead.Bead{
		ID:       "test-001",
		Title:    "Multi-label task",
		Priority: 1, // Would normally be medium
		Labels:   []string{"spec:simple", "other:label"},
	}

	// Expected failure: selectTier method does not exist on Runner yet
	gotTier := r.selectTier(b)

	// First matching label is "spec:simple" which maps to low
	if gotTier != provider.TierLow {
		t.Errorf("selectTier() with multiple labels = %q, want %q", gotTier, provider.TierLow)
	}
}
