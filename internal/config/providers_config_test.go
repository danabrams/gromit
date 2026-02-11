package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestProvidersConfigUnmarshal verifies that the providers section can be loaded
// from YAML with all expected fields (binary, flags, prompt_delivery, prompt_flag, models).
// Expected failure: ProvidersConfig and ProviderDef types do not exist on Config yet
func TestProvidersConfigUnmarshal(t *testing.T) {
	yamlContent := `
providers:
  claude:
    binary: claude
    flags: ["--no-input"]
    prompt_delivery: stdin
    models:
      high: opus
      medium: sonnet
      low: haiku
  openai:
    binary: codex
    flags: []
    prompt_delivery: prompt_file_arg
    prompt_flag: "--prompt"
    models:
      high: o3
      medium: gpt-4o
      low: gpt-4o-mini
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

	if cfg.Providers == nil {
		t.Fatal("Providers is nil, want non-nil map")
	}
	if len(cfg.Providers) != 2 {
		t.Errorf("len(Providers) = %d, want 2", len(cfg.Providers))
	}

	claudeProvider, ok := cfg.Providers["claude"]
	if !ok {
		t.Fatal("claude provider not found")
	}
	if claudeProvider.Binary != "claude" {
		t.Errorf("claude.Binary = %q, want %q", claudeProvider.Binary, "claude")
	}
	if len(claudeProvider.Flags) != 1 || claudeProvider.Flags[0] != "--no-input" {
		t.Errorf("claude.Flags = %v, want [--no-input]", claudeProvider.Flags)
	}
	if claudeProvider.PromptDelivery != "stdin" {
		t.Errorf("claude.PromptDelivery = %q, want %q", claudeProvider.PromptDelivery, "stdin")
	}
	if claudeProvider.Models["high"] != "opus" {
		t.Errorf("claude.Models[high] = %q, want %q", claudeProvider.Models["high"], "opus")
	}
	if claudeProvider.Models["medium"] != "sonnet" {
		t.Errorf("claude.Models[medium] = %q, want %q", claudeProvider.Models["medium"], "sonnet")
	}
	if claudeProvider.Models["low"] != "haiku" {
		t.Errorf("claude.Models[low] = %q, want %q", claudeProvider.Models["low"], "haiku")
	}

	openaiProvider, ok := cfg.Providers["openai"]
	if !ok {
		t.Fatal("openai provider not found")
	}
	if openaiProvider.Binary != "codex" {
		t.Errorf("openai.Binary = %q, want %q", openaiProvider.Binary, "codex")
	}
	if openaiProvider.PromptDelivery != "prompt_file_arg" {
		t.Errorf("openai.PromptDelivery = %q, want %q", openaiProvider.PromptDelivery, "prompt_file_arg")
	}
	if openaiProvider.PromptFlag != "--prompt" {
		t.Errorf("openai.PromptFlag = %q, want %q", openaiProvider.PromptFlag, "--prompt")
	}
	if openaiProvider.Models["high"] != "o3" {
		t.Errorf("openai.Models[high] = %q, want %q", openaiProvider.Models["high"], "o3")
	}
	if openaiProvider.Models["medium"] != "gpt-4o" {
		t.Errorf("openai.Models[medium] = %q, want %q", openaiProvider.Models["medium"], "gpt-4o")
	}
	if openaiProvider.Models["low"] != "gpt-4o-mini" {
		t.Errorf("openai.Models[low] = %q, want %q", openaiProvider.Models["low"], "gpt-4o-mini")
	}
}

// TestRoutingConfigUnmarshal verifies that the routing section can be loaded
// from YAML with phase preferences, ratio, and fallback configuration.
// Expected failure: RoutingConfig type does not exist on Config yet
func TestRoutingConfigUnmarshal(t *testing.T) {
	yamlContent := `
routing:
  phase_preferences:
    build: claude
    validate: any
    analyze: any
    scope_check: any
    precheck: any
    decompose: claude
    review: claude
  ratio:
    claude: 60
    openai: 40
  fallback:
    enabled: true
    cooldown: 30m
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

	if cfg.Routing.PhasePreferences == nil {
		t.Fatal("Routing.PhasePreferences is nil, want non-nil map")
	}
	if len(cfg.Routing.PhasePreferences) != 7 {
		t.Errorf("len(Routing.PhasePreferences) = %d, want 7", len(cfg.Routing.PhasePreferences))
	}
	if cfg.Routing.PhasePreferences["build"] != "claude" {
		t.Errorf("Routing.PhasePreferences[build] = %q, want %q", cfg.Routing.PhasePreferences["build"], "claude")
	}
	if cfg.Routing.PhasePreferences["validate"] != "any" {
		t.Errorf("Routing.PhasePreferences[validate] = %q, want %q", cfg.Routing.PhasePreferences["validate"], "any")
	}
	if cfg.Routing.PhasePreferences["decompose"] != "claude" {
		t.Errorf("Routing.PhasePreferences[decompose] = %q, want %q", cfg.Routing.PhasePreferences["decompose"], "claude")
	}

	if cfg.Routing.Ratio == nil {
		t.Fatal("Routing.Ratio is nil, want non-nil map")
	}
	if len(cfg.Routing.Ratio) != 2 {
		t.Errorf("len(Routing.Ratio) = %d, want 2", len(cfg.Routing.Ratio))
	}
	if cfg.Routing.Ratio["claude"] != 60 {
		t.Errorf("Routing.Ratio[claude] = %d, want 60", cfg.Routing.Ratio["claude"])
	}
	if cfg.Routing.Ratio["openai"] != 40 {
		t.Errorf("Routing.Ratio[openai] = %d, want 40", cfg.Routing.Ratio["openai"])
	}

	if !cfg.Routing.Fallback.Enabled {
		t.Error("Routing.Fallback.Enabled = false, want true")
	}
	if cfg.Routing.Fallback.Cooldown != "30m" {
		t.Errorf("Routing.Fallback.Cooldown = %q, want %q", cfg.Routing.Fallback.Cooldown, "30m")
	}
}

// TestNormalizeNilFieldsInitializesProvidersAndRouting verifies that NormalizeNilFields
// converts nil slices and maps in Providers and Routing to empty instances.
// Expected failure: ProvidersConfig and RoutingConfig fields do not exist on Config yet
func TestNormalizeNilFieldsInitializesProvidersAndRouting(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderDef{
			"test": {Binary: "test"},
		},
	}
	cfg.NormalizeNilFields()

	if cfg.Providers == nil {
		t.Error("Providers is nil after NormalizeNilFields, want non-nil map")
	}

	testProvider := cfg.Providers["test"]
	if testProvider.Flags == nil {
		t.Error("test.Flags is nil after NormalizeNilFields, want empty slice")
	}
	if testProvider.Models == nil {
		t.Error("test.Models is nil after NormalizeNilFields, want empty map")
	}

	if cfg.Routing.PhasePreferences == nil {
		t.Error("Routing.PhasePreferences is nil after NormalizeNilFields, want empty map")
	}
	if cfg.Routing.Ratio == nil {
		t.Error("Routing.Ratio is nil after NormalizeNilFields, want empty map")
	}
}

// TestSetDefaultsLeavesProvidersUntouched verifies that SetDefaults does not
// modify the Providers configuration when it exists.
// Expected failure: ProvidersConfig field does not exist on Config yet
func TestSetDefaultsLeavesProvidersUntouched(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderDef{
			"custom": {
				Binary:         "my-provider",
				PromptDelivery: "file_ref",
				Models: map[string]string{
					"high": "custom-high",
				},
			},
		},
	}
	cfg.SetDefaults()

	if len(cfg.Providers) != 1 {
		t.Errorf("len(Providers) = %d after SetDefaults, want 1", len(cfg.Providers))
	}
	custom := cfg.Providers["custom"]
	if custom.Binary != "my-provider" {
		t.Errorf("custom.Binary = %q after SetDefaults, want %q", custom.Binary, "my-provider")
	}
	if custom.Models["high"] != "custom-high" {
		t.Errorf("custom.Models[high] = %q after SetDefaults, want %q", custom.Models["high"], "custom-high")
	}
}

// TestHasProvidersReturnsTrueWhenProvidersExist verifies that HasProviders
// returns true when the providers map is non-empty.
// Expected failure: HasProviders() method does not exist on Config yet
func TestHasProvidersReturnsTrueWhenProvidersExist(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderDef{
			"claude": {Binary: "claude"},
		},
	}

	if !cfg.HasProviders() {
		t.Error("HasProviders() = false, want true when providers map is non-empty")
	}
}

// TestHasProvidersReturnsFalseWhenProvidersEmpty verifies that HasProviders
// returns false when the providers map is empty or nil.
// Expected failure: HasProviders() method does not exist on Config yet
func TestHasProvidersReturnsFalseWhenProvidersEmpty(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "NilProviders",
			cfg:  &Config{Providers: nil},
		},
		{
			name: "EmptyProviders",
			cfg:  &Config{Providers: map[string]ProviderDef{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.HasProviders() {
				t.Error("HasProviders() = true, want false when providers map is empty or nil")
			}
		})
	}
}

// TestRoutingFallbackCooldownParsing verifies that the cooldown string can be parsed
// as a valid duration when accessed programmatically.
// Expected failure: RoutingConfig.Fallback.Cooldown field does not exist yet
func TestRoutingFallbackCooldownParsing(t *testing.T) {
	yamlContent := `
routing:
  fallback:
    enabled: true
    cooldown: 45m
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

	// Verify the string value is correctly loaded
	if cfg.Routing.Fallback.Cooldown != "45m" {
		t.Errorf("Routing.Fallback.Cooldown = %q, want %q", cfg.Routing.Fallback.Cooldown, "45m")
	}

	// Verify it can be parsed as a duration
	duration, err := time.ParseDuration(cfg.Routing.Fallback.Cooldown)
	if err != nil {
		t.Errorf("time.ParseDuration(%q) error = %v, want nil", cfg.Routing.Fallback.Cooldown, err)
	}
	if duration != 45*time.Minute {
		t.Errorf("parsed duration = %v, want %v", duration, 45*time.Minute)
	}
}

// TestBackwardCompatibilityWithoutProvidersSection verifies that configs
// without a providers section still load successfully with the legacy claude config.
// Expected failure: HasProviders() method does not exist on Config yet
func TestBackwardCompatibilityWithoutProvidersSection(t *testing.T) {
	yamlContent := `
models:
  p0: opus
  p1: sonnet
  p2: haiku
claude:
  binary: claude
  timeout: 600
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

	// Should not have providers section
	if cfg.HasProviders() {
		t.Error("HasProviders() = true, want false when providers section is omitted")
	}

	// Legacy claude config should still work
	if cfg.Claude.Binary != "claude" {
		t.Errorf("Claude.Binary = %q, want %q", cfg.Claude.Binary, "claude")
	}
	if cfg.Models.P0 != "opus" {
		t.Errorf("Models.P0 = %q, want %q", cfg.Models.P0, "opus")
	}
}

// TestProvidersConfigEmptyFlags verifies that when flags is omitted or empty,
// NormalizeNilFields converts nil to empty slice.
// Expected failure: ProviderDef type does not exist yet
func TestProvidersConfigEmptyFlags(t *testing.T) {
	yamlContent := `
providers:
  claude:
    binary: claude
    prompt_delivery: stdin
    models:
      high: opus
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

	claudeProvider := cfg.Providers["claude"]
	if claudeProvider.Flags == nil {
		t.Error("claude.Flags is nil after Load, want empty slice")
	}
	if len(claudeProvider.Flags) != 0 {
		t.Errorf("len(claude.Flags) = %d, want 0", len(claudeProvider.Flags))
	}
}

// TestRoutingPhasePreferencesAnyValue verifies that "any" is a valid
// phase preference value indicating no provider preference.
// Expected failure: RoutingConfig.PhasePreferences field does not exist yet
func TestRoutingPhasePreferencesAnyValue(t *testing.T) {
	yamlContent := `
routing:
  phase_preferences:
    build: any
    validate: claude
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

	if cfg.Routing.PhasePreferences["build"] != "any" {
		t.Errorf("Routing.PhasePreferences[build] = %q, want %q", cfg.Routing.PhasePreferences["build"], "any")
	}
	if cfg.Routing.PhasePreferences["validate"] != "claude" {
		t.Errorf("Routing.PhasePreferences[validate] = %q, want %q", cfg.Routing.PhasePreferences["validate"], "claude")
	}
}
