package runner

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
)

// TestSelectTierDelegatesToConfig verifies that escalation.SelectTier calls cfg.SelectTier
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

	b := &bead.Bead{
		ID:       "test-001",
		Priority: 1,
		Labels:   []string{},
	}

	result := escalation.SelectTier(cfg, b)

	if result != provider.TierMedium {
		t.Errorf("SelectTier() = %q, want %q", result, provider.TierMedium)
	}
}

// TestRunValidatesRouterNotNil verifies that Run() checks for nil router
func TestRunValidatesRouterNotNil(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create runner with nil router
	r := &Runner{
		cfg:      cfg,
		beads:    &mockBeadClient{},
		renderer: &mockPromptRenderer{},
		router:   nil, // Explicitly nil
	}

	ctx := context.Background()
	err := r.Run(ctx, 0, time.Time{}, nil, true)

	if err == nil {
		t.Error("Run() with nil router should return error, got nil")
	}

	expectedMsg := "runner router is nil"
	if err.Error() != expectedMsg {
		t.Errorf("Run() error = %q, want %q", err.Error(), expectedMsg)
	}
}
