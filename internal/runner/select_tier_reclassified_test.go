
package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
)

// makeTestConfig creates a standard config for tier selection tests
func makeTestConfig(labels map[string]string) *config.Config {
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0:     provider.TierHigh,
			P1:     provider.TierMedium,
			P2:     provider.TierLow,
			Labels: labels,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

// TestAcceptance_EscalationSelectTierDelegatesToConfigSelectTier verifies that
// escalation.SelectTier() calls cfg.SelectTier() with bead priority and labels.
func TestAcceptance_EscalationSelectTierDelegatesToConfigSelectTier(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		labels   []string
		wantTier string
		cfgMod   func(*config.Config)
	}{
		{name: "P0 bead returns high tier", priority: 0, labels: []string{}, wantTier: provider.TierHigh},
		{name: "P1 bead returns medium tier", priority: 1, labels: []string{}, wantTier: provider.TierMedium},
		{name: "P2 bead returns low tier", priority: 2, labels: []string{}, wantTier: provider.TierLow},
		{
			name:     "complexity:high label overrides P1 to high tier",
			priority: 1,
			labels:   []string{"complexity:high"},
			wantTier: provider.TierHigh,
			cfgMod:   func(cfg *config.Config) { cfg.Models.Labels["complexity:high"] = provider.TierHigh },
		},
		{
			name:     "complexity:low label overrides P0 to low tier",
			priority: 0,
			labels:   []string{"complexity:low"},
			wantTier: provider.TierLow,
			cfgMod:   func(cfg *config.Config) { cfg.Models.Labels["complexity:low"] = provider.TierLow },
		},
		{
			name:     "backward compat: opus maps to high tier",
			priority: 0,
			labels:   []string{},
			wantTier: provider.TierHigh,
			cfgMod:   func(cfg *config.Config) { cfg.Models.P0 = "opus" },
		},
		{
			name:     "backward compat: sonnet maps to medium tier",
			priority: 1,
			labels:   []string{},
			wantTier: provider.TierMedium,
			cfgMod:   func(cfg *config.Config) { cfg.Models.P1 = "sonnet" },
		},
		{
			name:     "backward compat: haiku maps to low tier",
			priority: 2,
			labels:   []string{},
			wantTier: provider.TierLow,
			cfgMod:   func(cfg *config.Config) { cfg.Models.P2 = "haiku" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := makeTestConfig(nil)
			if tt.cfgMod != nil {
				tt.cfgMod(cfg)
			}

			b := &bead.Bead{ID: "test-001", Priority: tt.priority, Labels: tt.labels}

			if got := escalation.SelectTier(cfg, b); got != tt.wantTier {
				t.Errorf("SelectTier() = %q, want %q", got, tt.wantTier)
			}
		})
	}
}

// TestAcceptance_EscalationSelectTierNilSafety verifies nil handling.
func TestAcceptance_EscalationSelectTierNilSafety(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		bead *bead.Bead
	}{
		{name: "nil config", cfg: nil, bead: &bead.Bead{ID: "test", Priority: 0}},
		{name: "nil bead", cfg: makeTestConfig(nil), bead: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escalation.SelectTier(tt.cfg, tt.bead); got != provider.TierMedium {
				t.Errorf("SelectTier() = %q, want %q", got, provider.TierMedium)
			}
		})
	}
}

// TestAcceptance_EscalationSelectTierRespectsMultipleLabels verifies label precedence.
func TestAcceptance_EscalationSelectTierRespectsMultipleLabels(t *testing.T) {
	cfg := makeTestConfig(map[string]string{
		"complexity:high": provider.TierHigh,
		"complexity:low":  provider.TierLow,
		"spec:simple":     provider.TierLow,
	})
	b := &bead.Bead{ID: "test-001", Priority: 1, Labels: []string{"spec:simple", "other:label"}}

	// First matching label wins ("spec:simple" → low tier)
	if got := escalation.SelectTier(cfg, b); got != provider.TierLow {
		t.Errorf("SelectTier() with multiple labels = %q, want %q", got, provider.TierLow)
	}
}
