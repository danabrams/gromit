package provider

import (
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/config"
)

var DefaultTierToModelMap = map[string]string{
	TierHigh:   "opus",
	TierMedium: "sonnet",
	TierLow:    "haiku",
}

var defaultCodexTierToModelMap = map[string]string{
	TierHigh:   "gpt-5.3-codex",
	TierMedium: "gpt-5.3-codex",
	TierLow:    "gpt-5-mini",
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

func BuildProvidersFromConfig(cfg *config.Config) (map[string]Provider, error) {
	providers := make(map[string]Provider)
	for name, def := range cfg.Providers {
		switch {
		case name == "codex" || name == "openai" || def.Binary == "codex":
			tierMap := def.Models
			if len(tierMap) == 0 {
				tierMap = defaultCodexTierToModelMap
			}
			codexProvider := NewCodexProvider(def.Binary, def.Flags, tierMap)
			codexProvider.SetReasoningEffort(def.ReasoningEffort)
			providers[name] = codexProvider
		default:
			return nil, fmt.Errorf("unrecognized provider %q: supported providers are \"claude\" and \"codex\"", name)
		}
	}
	return providers, nil
}
