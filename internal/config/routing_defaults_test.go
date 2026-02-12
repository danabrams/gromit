package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSetDefaultsRoutingDefaults verifies that SetDefaults populates routing
// defaults when providers are configured: equal-split ratio from provider names,
// fallback cooldown of "30m", and fallback enabled for multi-provider configs.
func TestSetDefaultsRoutingDefaults(t *testing.T) {
	tests := []struct {
		name           string
		providers      map[string]ProviderDef
		wantRatio      map[string]int
		wantCooldown   string
		wantFallbackOn bool
	}{
		{
			name: "TwoProviders_EqualSplitRatio",
			providers: map[string]ProviderDef{
				"claude": {Binary: "claude"},
				"codex":  {Binary: "codex"},
			},
			wantRatio:      map[string]int{"claude": 50, "codex": 50},
			wantCooldown:   "30m",
			wantFallbackOn: true,
		},
		{
			name: "SingleProvider_FullRatio",
			providers: map[string]ProviderDef{
				"claude": {Binary: "claude"},
			},
			wantRatio:      map[string]int{"claude": 100},
			wantCooldown:   "30m",
			wantFallbackOn: false, // single provider: fallback meaningless
		},
		{
			name: "ThreeProviders_EqualSplit",
			providers: map[string]ProviderDef{
				"claude": {Binary: "claude"},
				"codex":  {Binary: "codex"},
				"other":  {Binary: "other"},
			},
			// Equal split across three providers — implementation may use integer division
			wantRatio:      map[string]int{"claude": 33, "codex": 33, "other": 33},
			wantCooldown:   "30m",
			wantFallbackOn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Providers: tt.providers}
			cfg.SetDefaults()

			// Verify ratio was populated from provider names
			if len(cfg.Routing.Ratio) != len(tt.wantRatio) {
				t.Fatalf("len(Routing.Ratio) = %d, want %d", len(cfg.Routing.Ratio), len(tt.wantRatio))
			}
			for name, wantVal := range tt.wantRatio {
				if cfg.Routing.Ratio[name] != wantVal {
					t.Errorf("Routing.Ratio[%s] = %d, want %d", name, cfg.Routing.Ratio[name], wantVal)
				}
			}

			// Verify fallback cooldown default
			if cfg.Routing.Fallback.Cooldown != tt.wantCooldown {
				t.Errorf("Routing.Fallback.Cooldown = %q, want %q", cfg.Routing.Fallback.Cooldown, tt.wantCooldown)
			}

			// Verify fallback enabled state
			if cfg.Routing.Fallback.Enabled != tt.wantFallbackOn {
				t.Errorf("Routing.Fallback.Enabled = %v, want %v", cfg.Routing.Fallback.Enabled, tt.wantFallbackOn)
			}
		})
	}
}

// TestSetDefaultsPreservesUserRoutingValues verifies that SetDefaults does not
// overwrite user-specified routing values, while still applying defaults for
// fields the user left empty.
func TestSetDefaultsPreservesUserRoutingValues(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderDef{
			"claude": {Binary: "claude"},
			"codex":  {Binary: "codex"},
		},
		Routing: RoutingConfig{
			Ratio: map[string]int{
				"claude": 70,
				"codex":  30,
			},
			// Leave Fallback.Cooldown empty — SetDefaults should fill it in
		},
	}
	cfg.SetDefaults()

	// User-specified ratio must be preserved
	if cfg.Routing.Ratio["claude"] != 70 {
		t.Errorf("Routing.Ratio[claude] = %d, want 70", cfg.Routing.Ratio["claude"])
	}
	if cfg.Routing.Ratio["codex"] != 30 {
		t.Errorf("Routing.Ratio[codex] = %d, want 30", cfg.Routing.Ratio["codex"])
	}

	// Fallback cooldown should still get its default even though ratio was set
	if cfg.Routing.Fallback.Cooldown != "30m" {
		t.Errorf("Routing.Fallback.Cooldown = %q, want %q (default applied for empty field)", cfg.Routing.Fallback.Cooldown, "30m")
	}
}

// TestSetDefaultsDoesNotAddRoutingWhenNoProviders verifies that SetDefaults
// does not populate routing defaults when no providers are configured, but does
// populate them when providers are present.
func TestSetDefaultsDoesNotAddRoutingWhenNoProviders(t *testing.T) {
	// Without providers — routing should remain empty
	cfgNoProviders := &Config{}
	cfgNoProviders.SetDefaults()

	if len(cfgNoProviders.Routing.Ratio) != 0 {
		t.Errorf("no-providers: len(Routing.Ratio) = %d, want 0", len(cfgNoProviders.Routing.Ratio))
	}
	if cfgNoProviders.Routing.Fallback.Cooldown != "" {
		t.Errorf("no-providers: Routing.Fallback.Cooldown = %q, want empty", cfgNoProviders.Routing.Fallback.Cooldown)
	}
	if cfgNoProviders.Routing.Fallback.Enabled {
		t.Error("no-providers: Routing.Fallback.Enabled = true, want false")
	}

	// With providers — routing should get defaults
	cfgWithProviders := &Config{
		Providers: map[string]ProviderDef{
			"claude": {Binary: "claude"},
			"codex":  {Binary: "codex"},
		},
	}
	cfgWithProviders.SetDefaults()

	if len(cfgWithProviders.Routing.Ratio) != 2 {
		t.Errorf("with-providers: len(Routing.Ratio) = %d, want 2", len(cfgWithProviders.Routing.Ratio))
	}
}

// TestSetDefaultsRoutingViaLoad verifies the full Load path applies routing defaults
// when a YAML config has providers but no routing section.
func TestSetDefaultsRoutingViaLoad(t *testing.T) {
	yamlContent := `
providers:
  claude:
    binary: claude
    models:
      high: opus
      medium: sonnet
      low: haiku
  codex:
    binary: codex
    models:
      high: gpt-5.3-codex
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Ratio should be auto-populated from provider names with equal split
	if len(cfg.Routing.Ratio) != 2 {
		t.Fatalf("len(Routing.Ratio) = %d, want 2 (one per provider)", len(cfg.Routing.Ratio))
	}
	if cfg.Routing.Ratio["claude"] != cfg.Routing.Ratio["codex"] {
		t.Errorf("Routing.Ratio not equal: claude=%d, codex=%d",
			cfg.Routing.Ratio["claude"], cfg.Routing.Ratio["codex"])
	}

	// Fallback should default to enabled with 30m cooldown
	if !cfg.Routing.Fallback.Enabled {
		t.Error("Routing.Fallback.Enabled = false, want true for multi-provider config")
	}
	if cfg.Routing.Fallback.Cooldown != "30m" {
		t.Errorf("Routing.Fallback.Cooldown = %q, want %q", cfg.Routing.Fallback.Cooldown, "30m")
	}
}

// TestSetDefaultsRoutingViaLoadPreservesExplicit verifies the full Load path
// does not overwrite routing values that the user explicitly set in YAML, while
// still applying defaults for unset fields.
func TestSetDefaultsRoutingViaLoadPreservesExplicit(t *testing.T) {
	yamlContent := `
providers:
  claude:
    binary: claude
  codex:
    binary: codex
routing:
  ratio:
    claude: 80
    codex: 20
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Explicit ratio values must be preserved
	if cfg.Routing.Ratio["claude"] != 80 {
		t.Errorf("Routing.Ratio[claude] = %d, want 80", cfg.Routing.Ratio["claude"])
	}
	if cfg.Routing.Ratio["codex"] != 20 {
		t.Errorf("Routing.Ratio[codex] = %d, want 20", cfg.Routing.Ratio["codex"])
	}

	// Fallback cooldown should get default even though ratio was explicitly set
	if cfg.Routing.Fallback.Cooldown != "30m" {
		t.Errorf("Routing.Fallback.Cooldown = %q, want %q (default for unset field)", cfg.Routing.Fallback.Cooldown, "30m")
	}

	// Fallback should be enabled by default for multi-provider
	if !cfg.Routing.Fallback.Enabled {
		t.Error("Routing.Fallback.Enabled = false, want true (default for multi-provider)")
	}
}
