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

func TestBuildProvidersFromConfig_NilConfig(t *testing.T) {
	providers, err := provider.BuildProvidersFromConfig(nil)
	if err == nil {
		t.Fatal("BuildProvidersFromConfig() error = nil, want non-nil")
	}
	if providers != nil {
		t.Fatalf("BuildProvidersFromConfig() providers = %#v, want nil", providers)
	}
}

func TestBuildRouterFromConfig_NoProvidersReturnsSingleClaudeRouter(t *testing.T) {
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 10,
		},
	}

	router, err := provider.BuildRouterFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildRouterFromConfig() error = %v", err)
	}
	if router == nil {
		t.Fatal("BuildRouterFromConfig() router = nil, want non-nil")
	}

	selected, model := router.Select("build", provider.TierMedium)
	if selected == nil {
		t.Fatal("router.Select() provider = nil, want non-nil")
	}
	if got := selected.Name(); got != "claude" {
		t.Fatalf("router.Select() provider name = %q, want %q", got, "claude")
	}
	if model != "sonnet" {
		t.Fatalf("router.Select() model = %q, want %q", model, "sonnet")
	}
}

func TestBuildRouterFromConfig_ClaudeOnlyConfigReturnsClaudeRouter(t *testing.T) {
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Timeout: 10,
		},
		Providers: map[string]config.ProviderDef{
			"claude": {
				Binary: "claude",
				Models: map[string]string{
					provider.TierMedium: "custom-sonnet",
				},
			},
		},
		Routing: config.RoutingConfig{
			PhasePreferences: map[string]string{
				"build": "claude",
			},
			Ratio: map[string]int{
				"claude": 100,
			},
		},
	}

	router, err := provider.BuildRouterFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildRouterFromConfig() error = %v", err)
	}

	selected, model := router.Select("build", provider.TierMedium)
	if selected == nil {
		t.Fatal("router.Select() provider = nil, want non-nil")
	}
	if got := selected.Name(); got != "claude" {
		t.Fatalf("router.Select() provider name = %q, want %q", got, "claude")
	}
	if model != "custom-sonnet" {
		t.Fatalf("router.Select() model = %q, want %q", model, "custom-sonnet")
	}
}

func TestBuildRouterFromConfig_MultiProviderAppliesFallbackDefaults(t *testing.T) {
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Timeout: 10,
		},
		Providers: map[string]config.ProviderDef{
			"claude": {
				Binary: "claude",
			},
			"codex": {
				Binary: "codex",
			},
		},
		Routing: config.RoutingConfig{
			PhasePreferences: map[string]string{
				"build": "claude",
			},
		},
	}

	router, err := provider.BuildRouterFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildRouterFromConfig() error = %v", err)
	}

	first, _ := router.Select("build", provider.TierMedium)
	if first == nil {
		t.Fatal("first router.Select() provider = nil, want non-nil")
	}
	if got := first.Name(); got != "claude" {
		t.Fatalf("first router.Select() provider name = %q, want %q", got, "claude")
	}

	router.MarkUnavailable("claude")

	second, _ := router.Select("build", provider.TierMedium)
	if second == nil {
		t.Fatal("second router.Select() provider = nil, want non-nil")
	}
	if got := second.Name(); got != "codex" {
		t.Fatalf("second router.Select() provider name = %q, want %q", got, "codex")
	}
}

func TestBuildRouterFromConfig_CodexOnlyConfigReturnsCodexRouter(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"codex": {
				Binary: "codex",
			},
		},
	}

	router, err := provider.BuildRouterFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildRouterFromConfig() error = %v", err)
	}

	selected, model := router.Select("build", provider.TierMedium)
	if selected == nil {
		t.Fatal("router.Select() provider = nil, want non-nil")
	}
	if got := selected.Name(); got != "codex" {
		t.Fatalf("router.Select() provider name = %q, want %q", got, "codex")
	}
	if model != "gpt-5.3-codex" {
		t.Fatalf("router.Select() model = %q, want %q", model, "gpt-5.3-codex")
	}
}
