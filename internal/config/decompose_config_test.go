package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

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

func TestSetDefaultsDecomposeTargetDefaultsNarrowScope(t *testing.T) {
	var cfg Config
	cfg.SetDefaults()
	if cfg.Decompose.Target != DecompositionTargetNarrowScope {
		t.Fatalf("Decompose.Target = %q, want %q", cfg.Decompose.Target, DecompositionTargetNarrowScope)
	}
}

func TestConfigParsesDecomposeTarget(t *testing.T) {
	const raw = `
decompose:
  target: single_concern
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if cfg.Decompose.Target != DecompositionTargetSingleConcern {
		t.Fatalf("Decompose.Target = %q, want %q", cfg.Decompose.Target, DecompositionTargetSingleConcern)
	}
}

func TestValidateRejectsUnknownDecomposeTarget(t *testing.T) {
	var cfg Config
	cfg.SetDefaults()
	cfg.Decompose.Target = DecompositionTarget("shell_shock")
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error for invalid decompose target, got nil")
	}
}
