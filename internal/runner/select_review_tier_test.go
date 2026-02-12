package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// TestSelectReviewTier_OpusBuild verifies selectReviewTier returns "high" when buildModel is "opus"
func TestSelectReviewTier_OpusBuild(t *testing.T) {
	cfg := &config.Config{}
	cfg.Models.P1 = provider.TierMedium
	cfg.NormalizeNilFields()

	r := &Runner{cfg: cfg}
	b := &bead.Bead{ID: "test-001", Priority: 1}

	tier := r.selectReviewTier(b, "opus")

	if tier != provider.TierHigh {
		t.Errorf("selectReviewTier() with opus build = %q, want %q", tier, provider.TierHigh)
	}
}
