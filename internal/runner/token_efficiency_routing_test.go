package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

func TestResolveUtilityTaskTier_UsesUtilityTierForSummarization(t *testing.T) {
	cfg := &config.Config{
		TokenEfficiency: config.TokenEfficiencyConfig{
			Routing: config.TokenEfficiencyRoutingConfig{
				Enabled:     true,
				UtilityTier: provider.TierLow,
			},
		},
	}

	got := resolveUtilityTaskTier(cfg, "summarization", provider.TierHigh)
	if got != provider.TierLow {
		t.Fatalf("resolveUtilityTaskTier() = %q, want %q", got, provider.TierLow)
	}
}

func TestResolveUtilityTaskTier_UsesTaskOverrideWhenEnabled(t *testing.T) {
	cfg := &config.Config{
		TokenEfficiency: config.TokenEfficiencyConfig{
			Routing: config.TokenEfficiencyRoutingConfig{
				Enabled:     true,
				UtilityTier: provider.TierLow,
				TaskOverrides: map[string]string{
					"summarization": provider.TierMedium,
				},
			},
		},
	}

	got := resolveUtilityTaskTier(cfg, "summarization", provider.TierHigh)
	if got != provider.TierMedium {
		t.Fatalf("resolveUtilityTaskTier() = %q, want %q", got, provider.TierMedium)
	}
}

func TestResolveUtilityTaskTier_UsesUtilityTierForMaskingTransform(t *testing.T) {
	cfg := &config.Config{
		TokenEfficiency: config.TokenEfficiencyConfig{
			Routing: config.TokenEfficiencyRoutingConfig{
				Enabled:     true,
				UtilityTier: provider.TierLow,
			},
		},
	}

	got := resolveUtilityTaskTier(cfg, "masking_transform", provider.TierHigh)
	if got != provider.TierLow {
		t.Fatalf("resolveUtilityTaskTier() = %q, want %q", got, provider.TierLow)
	}
}
