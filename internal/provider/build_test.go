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
