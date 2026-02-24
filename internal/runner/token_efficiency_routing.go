package runner

import (
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

func resolveUtilityTaskTier(cfg *config.Config, taskCategory, fallbackTier string) string {
	normalizedCategory := strings.ToLower(strings.TrimSpace(taskCategory))

	if cfg == nil || !cfg.TokenEfficiency.Routing.IsEnabled() {
		return fallbackTier
	}
	if !isUtilityTaskCategory(normalizedCategory) {
		return fallbackTier
	}
	if !cfg.TokenEfficiency.Routing.KillSwitches.DisableTaskOverrides {
		if overrideTier, ok := cfg.TokenEfficiency.Routing.TaskOverrides[normalizedCategory]; ok && overrideTier != "" {
			return provider.TierFromLegacyModel(overrideTier)
		}
	}
	if cfg.TokenEfficiency.Routing.UtilityTier == "" {
		return fallbackTier
	}
	return provider.TierFromLegacyModel(cfg.TokenEfficiency.Routing.UtilityTier)
}

func isUtilityTaskCategory(taskCategory string) bool {
	switch taskCategory {
	case "summarization", "masking_transform", "discovery_indexing":
		return true
	default:
		return false
	}
}
