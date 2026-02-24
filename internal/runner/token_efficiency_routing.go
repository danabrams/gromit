package runner

import (
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

func resolveUtilityTaskTier(cfg *config.Config, taskCategory, fallbackTier string) string {
	if cfg == nil || !cfg.TokenEfficiency.Routing.IsEnabled() {
		return fallbackTier
	}
	if taskCategory != "summarization" {
		return fallbackTier
	}
	if cfg.TokenEfficiency.Routing.UtilityTier == "" {
		return fallbackTier
	}
	return provider.TierFromLegacyModel(cfg.TokenEfficiency.Routing.UtilityTier)
}
