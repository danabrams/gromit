package provider

import (
	"time"

	"github.com/danabrams/gromit/internal/config"
)

var DefaultTierToModelMap = map[string]string{
	TierHigh:   "opus",
	TierMedium: "sonnet",
	TierLow:    "haiku",
}

func ParseFallbackCooldown(cfg *config.Config) time.Duration {
	if cfg == nil || !cfg.Routing.Fallback.EnabledOrDefault(len(cfg.Providers) > 1) || cfg.Routing.Fallback.Cooldown == "" {
		return 0
	}
	cooldown, err := time.ParseDuration(cfg.Routing.Fallback.Cooldown)
	if err != nil {
		return 30 * time.Minute
	}
	return cooldown
}
