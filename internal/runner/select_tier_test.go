package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// TestSelectTierDelegatesToConfig verifies that selectTier calls cfg.SelectTier
func TestSelectTierDelegatesToConfig(t *testing.T) {
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: provider.TierHigh,
			P1: provider.TierMedium,
			P2: provider.TierLow,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	r := &Runner{
		cfg: cfg,
	}

	b := &bead.Bead{
		ID:       "test-001",
		Priority: 1,
		Labels:   []string{},
	}

	result := r.selectTier(b)

	if result != provider.TierMedium {
		t.Errorf("selectTier() = %q, want %q", result, provider.TierMedium)
	}
}
