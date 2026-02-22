package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

func TestBuildRetroProviderRunner_ProvidersPathUsesRouterAdapter(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"openai": {
				Binary: "codex",
				Models: map[string]string{
					provider.TierHigh:   "gpt-5.3-codex",
					provider.TierMedium: "gpt-5.3-codex",
					provider.TierLow:    "gpt-5-mini",
				},
			},
		},
	}

	runner, err := buildRetroProviderRunner(cfg)
	if err != nil {
		t.Fatalf("buildRetroProviderRunner() error = %v", err)
	}

	adapter, ok := runner.(*retroRouterAdapter)
	if !ok {
		t.Fatalf("runner type = %T, want *retroRouterAdapter", runner)
	}
	if adapter.Router == nil {
		t.Fatal("adapter.Router = nil, want non-nil")
	}
	if adapter.Phase != retroSessionCommand {
		t.Fatalf("adapter.Phase = %q, want %q", adapter.Phase, retroSessionCommand)
	}
}
