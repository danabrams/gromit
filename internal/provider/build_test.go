package provider_test

import (
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

func TestDefaultTierToModelMap(t *testing.T) {
	if got := provider.DefaultTierToModelMap[provider.TierHigh]; got != "opus" {
		t.Fatalf("DefaultTierToModelMap[%q] = %q, want %q", provider.TierHigh, got, "opus")
	}
	if got := provider.DefaultTierToModelMap[provider.TierMedium]; got != "sonnet" {
		t.Fatalf("DefaultTierToModelMap[%q] = %q, want %q", provider.TierMedium, got, "sonnet")
	}
	if got := provider.DefaultTierToModelMap[provider.TierLow]; got != "haiku" {
		t.Fatalf("DefaultTierToModelMap[%q] = %q, want %q", provider.TierLow, got, "haiku")
	}
}

func TestParseFallbackCooldown_UsesConfiguredDuration(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"claude": {},
			"codex":  {},
		},
		Routing: config.RoutingConfig{
			Fallback: config.FallbackConfig{
				Enabled:  func() *bool { b := true; return &b }(),
				Cooldown: "45m",
			},
		},
	}

	if got := provider.ParseFallbackCooldown(cfg); got != 45*time.Minute {
		t.Fatalf("ParseFallbackCooldown() = %v, want %v", got, 45*time.Minute)
	}
}

func TestBuildProvidersFromConfig_CodexDefaults(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"codex": {
				Binary: "codex",
			},
		},
	}

	providers, err := provider.BuildProvidersFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildProvidersFromConfig() error = %v", err)
	}

	codexProvider, ok := providers["codex"]
	if !ok {
		t.Fatalf("providers missing %q entry", "codex")
	}

	if got := codexProvider.ModelForTier(provider.TierHigh); got != "gpt-5.3-codex" {
		t.Fatalf("ModelForTier(%q) = %q, want %q", provider.TierHigh, got, "gpt-5.3-codex")
	}
	if got := codexProvider.ModelForTier(provider.TierMedium); got != "gpt-5.3-codex" {
		t.Fatalf("ModelForTier(%q) = %q, want %q", provider.TierMedium, got, "gpt-5.3-codex")
	}
	if got := codexProvider.ModelForTier(provider.TierLow); got != "gpt-5-mini" {
		t.Fatalf("ModelForTier(%q) = %q, want %q", provider.TierLow, got, "gpt-5-mini")
	}
}

func TestBuildProvidersFromConfig_ClaudeDefaults(t *testing.T) {
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Timeout: 10,
		},
		Providers: map[string]config.ProviderDef{
			"claude": {
				Binary: "claude",
			},
		},
	}

	providers, err := provider.BuildProvidersFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildProvidersFromConfig() error = %v", err)
	}

	claudeProvider, ok := providers["claude"]
	if !ok {
		t.Fatalf("providers missing %q entry", "claude")
	}

	if got := claudeProvider.ModelForTier(provider.TierHigh); got != "opus" {
		t.Fatalf("ModelForTier(%q) = %q, want %q", provider.TierHigh, got, "opus")
	}
	if got := claudeProvider.ModelForTier(provider.TierMedium); got != "sonnet" {
		t.Fatalf("ModelForTier(%q) = %q, want %q", provider.TierMedium, got, "sonnet")
	}
	if got := claudeProvider.ModelForTier(provider.TierLow); got != "haiku" {
		t.Fatalf("ModelForTier(%q) = %q, want %q", provider.TierLow, got, "haiku")
	}
}
