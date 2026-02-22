package config

import "testing"

func TestConfigHasDecomposeField(t *testing.T) {
	var cfg Config
	cfg.Decompose = DecomposeConfig{Tier: "high"}
	if cfg.Decompose.Tier != "high" {
		t.Errorf("Decompose.Tier = %q, want %q", cfg.Decompose.Tier, "high")
	}
}

func TestSetDefaultsDecomposeTierDefaultsMedium(t *testing.T) {
	var cfg Config
	cfg.SetDefaults()
	if cfg.Decompose.Tier != "medium" {
		t.Errorf("Decompose.Tier = %q, want %q", cfg.Decompose.Tier, "medium")
	}
}
