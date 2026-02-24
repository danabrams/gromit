package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTokenEfficiencyConfigYAMLDeserialization(t *testing.T) {
	yamlContent := `
token_efficiency:
  cache:
    enabled: true
    ttl: 45m
    capacity: 256
  routing:
    enabled: true
    utility_tier: low
    kill_switches:
      disable_utility_routing: true
      disable_task_overrides: true
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

	if !cfg.TokenEfficiency.Cache.Enabled {
		t.Errorf("TokenEfficiency.Cache.Enabled = false, want true")
	}
	if cfg.TokenEfficiency.Cache.TTL != "45m" {
		t.Errorf("TokenEfficiency.Cache.TTL = %q, want %q", cfg.TokenEfficiency.Cache.TTL, "45m")
	}
	if cfg.TokenEfficiency.Cache.Capacity != 256 {
		t.Errorf("TokenEfficiency.Cache.Capacity = %d, want 256", cfg.TokenEfficiency.Cache.Capacity)
	}

	if !cfg.TokenEfficiency.Routing.Enabled {
		t.Errorf("TokenEfficiency.Routing.Enabled = false, want true")
	}
	if cfg.TokenEfficiency.Routing.UtilityTier != "low" {
		t.Errorf("TokenEfficiency.Routing.UtilityTier = %q, want %q", cfg.TokenEfficiency.Routing.UtilityTier, "low")
	}
	if !cfg.TokenEfficiency.Routing.KillSwitches.DisableUtilityRouting {
		t.Errorf("TokenEfficiency.Routing.KillSwitches.DisableUtilityRouting = false, want true")
	}
	if !cfg.TokenEfficiency.Routing.KillSwitches.DisableTaskOverrides {
		t.Errorf("TokenEfficiency.Routing.KillSwitches.DisableTaskOverrides = false, want true")
	}
}

func TestSetDefaultsTokenEfficiencyDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.TokenEfficiency.Cache.Enabled {
		t.Errorf("TokenEfficiency.Cache.Enabled = true, want false by default")
	}
	if cfg.TokenEfficiency.Cache.TTL != "30m" {
		t.Errorf("TokenEfficiency.Cache.TTL = %q, want %q", cfg.TokenEfficiency.Cache.TTL, "30m")
	}
	if cfg.TokenEfficiency.Cache.Capacity != 256 {
		t.Errorf("TokenEfficiency.Cache.Capacity = %d, want 256", cfg.TokenEfficiency.Cache.Capacity)
	}

	if cfg.TokenEfficiency.Routing.Enabled {
		t.Errorf("TokenEfficiency.Routing.Enabled = true, want false by default")
	}
	if cfg.TokenEfficiency.Routing.UtilityTier != "low" {
		t.Errorf("TokenEfficiency.Routing.UtilityTier = %q, want %q", cfg.TokenEfficiency.Routing.UtilityTier, "low")
	}
	if cfg.TokenEfficiency.Routing.KillSwitches.DisableUtilityRouting {
		t.Errorf("TokenEfficiency.Routing.KillSwitches.DisableUtilityRouting = true, want false by default")
	}
	if cfg.TokenEfficiency.Routing.KillSwitches.DisableTaskOverrides {
		t.Errorf("TokenEfficiency.Routing.KillSwitches.DisableTaskOverrides = true, want false by default")
	}
}

func TestTokenEfficiencyAccessorsRespectIndependentKillSwitches(t *testing.T) {
	cfg := Config{
		TokenEfficiency: TokenEfficiencyConfig{
			Cache: TokenEfficiencyCacheConfig{
				Enabled: true,
			},
			Routing: TokenEfficiencyRoutingConfig{
				Enabled: true,
				KillSwitches: TokenEfficiencyRoutingKillSwitchesConfig{
					DisableUtilityRouting: true,
				},
			},
		},
	}

	if !cfg.TokenEfficiency.Cache.IsEnabled() {
		t.Errorf("TokenEfficiency.Cache.IsEnabled() = false, want true")
	}
	if cfg.TokenEfficiency.Routing.IsEnabled() {
		t.Errorf("TokenEfficiency.Routing.IsEnabled() = true, want false when disable_utility_routing is set")
	}
}

func TestTokenEfficiencyRoutingTaskOverridesYAMLDeserialization(t *testing.T) {
	yamlContent := `
token_efficiency:
  routing:
    task_overrides:
      summarization: low
      discovery_indexing: medium
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

	if cfg.TokenEfficiency.Routing.TaskOverrides["summarization"] != "low" {
		t.Errorf("TokenEfficiency.Routing.TaskOverrides[summarization] = %q, want %q", cfg.TokenEfficiency.Routing.TaskOverrides["summarization"], "low")
	}
	if cfg.TokenEfficiency.Routing.TaskOverrides["discovery_indexing"] != "medium" {
		t.Errorf("TokenEfficiency.Routing.TaskOverrides[discovery_indexing] = %q, want %q", cfg.TokenEfficiency.Routing.TaskOverrides["discovery_indexing"], "medium")
	}
}
