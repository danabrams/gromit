package runner

import (
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestEstimatedCostUSD_UsesConfiguredProviderPricing(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "gromit.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%s) error = %v", cfgPath, err)
	}
	if len(cfg.Providers) == 0 {
		t.Fatal("expected gromit.yaml to configure providers")
	}

	costDefs := make(map[string]config.ProviderDef)
	for name, def := range cfg.Providers {
		costDefs[name] = def
	}

	r := &Runner{providerCostDefs: costDefs}

	for name, def := range cfg.Providers {
		model := def.Models["medium"]
		if model == "" {
			t.Fatalf("provider %q missing medium model in gromit.yaml", name)
		}

		providerName := name
		if name == "openai" && def.Binary == "codex" {
			providerName = "codex"
		}

		cost := r.estimatedCostUSD(providerName, model, 0, 2000, 1000)
		if cost <= 0 {
			t.Errorf("estimatedCostUSD(%q, %q) = %v, want > 0", providerName, model, cost)
		}
	}
}
