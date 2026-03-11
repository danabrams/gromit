package provider

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
)

var DefaultTierToModelMap = map[string]string{
	TierHigh:   "opus",
	TierMedium: "sonnet",
	TierLow:    "haiku",
}

var DefaultCodexTierToModelMap = map[string]string{
	TierHigh:   "gpt-5.3-codex",
	TierMedium: "gpt-5.3-codex",
	TierLow:    "gpt-5.1-codex-mini",
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
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	providers := make(map[string]Provider)
	for name, def := range cfg.Providers {
		binaryName := filepath.Base(def.Binary)
		switch {
		case name == "claude" || def.Binary == "claude" || binaryName == "claude":
			tierMap := def.Models
			if len(tierMap) == 0 {
				tierMap = DefaultTierToModelMap
			}
			client, err := claude.NewClient(def.Binary, def.Flags, cfg.Claude.Timeout)
			if err != nil {
				return nil, err
			}
			providers[name] = NewClaudeProvider(client, tierMap)
		case name == "codex" || name == "openai" || def.Binary == "codex" || binaryName == "codex":
			tierMap := def.Models
			if len(tierMap) == 0 {
				tierMap = DefaultCodexTierToModelMap
			}
			codexProvider := NewCodexProvider(def.Binary, def.Flags, tierMap)
			codexProvider.SetReasoningEffort(def.ReasoningEffort)
			providers[name] = codexProvider
		case name == providerNameGemini || def.Binary == providerNameGemini || binaryName == providerNameGemini:
			tierMap := config.ResolveGeminiModelMap(def)
			providers[name] = NewGeminiProvider(def.Binary, def.Flags, tierMap, def)
		default:
			return nil, fmt.Errorf("unrecognized provider %q: supported providers are \"claude\", \"codex\", and \"gemini\"", name)
		}
	}
	return providers, nil
}

func BuildRouterFromConfig(cfg *config.Config) (*Router, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	if len(cfg.Providers) > 0 {
		cfg.SetDefaults()
		cfg.NormalizeNilFields()

		providers, err := BuildProvidersFromConfig(cfg)
		if err != nil {
			return nil, err
		}
		return NewRouter(
			providers,
			cfg.Routing.PhasePreferences,
			cfg.Routing.Ratio,
			ParseFallbackCooldown(cfg),
			nil,
			nil,
		), nil
	}

	client, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return nil, err
	}

	claudeProvider := NewClaudeProvider(client, DefaultTierToModelMap)
	return NewSingleProviderRouter(claudeProvider), nil
}
