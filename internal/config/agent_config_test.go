package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAgentsConfigDefaultsInitializePhasesToClaude verifies that SetDefaults()
// initializes all phase fields in PhasesConfig to "claude" when not specified.
func TestAgentsConfigDefaultsInitializePhasesToClaude(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	// All phase fields should default to "claude"
	phases := []struct {
		name  string
		value string
	}{
		{"Build", cfg.Agents.Definitions["claude"].Phases.Build},
		{"Validate", cfg.Agents.Definitions["claude"].Phases.Validate},
		{"Analyze", cfg.Agents.Definitions["claude"].Phases.Analyze},
		{"Retro", cfg.Agents.Definitions["claude"].Phases.Retro},
		{"Scope", cfg.Agents.Definitions["claude"].Phases.Scope},
		{"Decompose", cfg.Agents.Definitions["claude"].Phases.Decompose},
		{"Review", cfg.Agents.Definitions["claude"].Phases.Review},
	}

	for _, p := range phases {
		if p.value != "claude" {
			t.Errorf("Expected Agents.Definitions[\"claude\"].Phases.%s to default to \"claude\", got %q", p.name, p.value)
		}
	}
}

// TestAgentsConfigDefaultsPreservesExistingPhases verifies that SetDefaults()
// does not override phase fields that are already set in the config.
func TestAgentsConfigDefaultsPreservesExistingPhases(t *testing.T) {
	cfg := &Config{
		Agents: AgentsConfig{
			Definitions: map[string]AgentDefinition{
				"claude": {
					Phases: PhasesConfig{
						Build:    "custom-agent",
						Validate: "claude",
					},
				},
			},
		},
	}
	cfg.SetDefaults()

	// Custom values should be preserved
	if cfg.Agents.Definitions["claude"].Phases.Build != "custom-agent" {
		t.Errorf("Expected Build to remain \"custom-agent\", got %q", cfg.Agents.Definitions["claude"].Phases.Build)
	}
	if cfg.Agents.Definitions["claude"].Phases.Validate != "claude" {
		t.Errorf("Expected Validate to remain \"claude\", got %q", cfg.Agents.Definitions["claude"].Phases.Validate)
	}

	// Unset fields should default to "claude"
	if cfg.Agents.Definitions["claude"].Phases.Analyze != "claude" {
		t.Errorf("Expected Analyze to default to \"claude\", got %q", cfg.Agents.Definitions["claude"].Phases.Analyze)
	}
}

// TestNormalizeNilFieldsInitializesAgentsDefinitions verifies that
// NormalizeNilFields() initializes a nil Definitions map to an empty map.
func TestNormalizeNilFieldsInitializesAgentsDefinitions(t *testing.T) {
	cfg := &Config{}
	cfg.NormalizeNilFields()

	if cfg.Agents.Definitions == nil {
		t.Error("Expected Agents.Definitions to be non-nil after normalization")
	}

	// Should be an empty map, not nil
	if len(cfg.Agents.Definitions) != 0 {
		t.Errorf("Expected empty Definitions map, got %d entries", len(cfg.Agents.Definitions))
	}
}

// TestNormalizeNilFieldsPreservesExistingDefinitions verifies that
// NormalizeNilFields() does not replace an existing Definitions map.
func TestNormalizeNilFieldsPreservesExistingDefinitions(t *testing.T) {
	cfg := &Config{
		Agents: AgentsConfig{
			Definitions: map[string]AgentDefinition{
				"claude": {
					Phases: PhasesConfig{
						Build: "claude",
					},
				},
				"custom": {
					Phases: PhasesConfig{
						Build: "custom-agent",
					},
				},
			},
		},
	}
	cfg.NormalizeNilFields()

	if len(cfg.Agents.Definitions) != 2 {
		t.Errorf("Expected 2 definitions, got %d", len(cfg.Agents.Definitions))
	}
	if cfg.Agents.Definitions["claude"].Phases.Build != "claude" {
		t.Errorf("Expected claude definition preserved, got %q", cfg.Agents.Definitions["claude"].Phases.Build)
	}
	if cfg.Agents.Definitions["custom"].Phases.Build != "custom-agent" {
		t.Errorf("Expected custom definition preserved, got %q", cfg.Agents.Definitions["custom"].Phases.Build)
	}
}

// TestLoadAgentsConfigFromYAML verifies that the agents config can be loaded
// from a YAML file with the correct structure and YAML tags.
func TestLoadAgentsConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `agents:
  definitions:
    claude:
      phases:
        build: claude
        validate: claude
        analyze: claude
    codex:
      phases:
        build: codex
        review: codex
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Check that agents were loaded correctly
	if len(cfg.Agents.Definitions) != 2 {
		t.Errorf("Expected 2 agent definitions, got %d", len(cfg.Agents.Definitions))
	}

	// Check claude agent
	claudeDef, ok := cfg.Agents.Definitions["claude"]
	if !ok {
		t.Fatal("Expected claude agent definition to be loaded")
	}
	if claudeDef.Phases.Build != "claude" {
		t.Errorf("Expected claude Build phase to be \"claude\", got %q", claudeDef.Phases.Build)
	}
	if claudeDef.Phases.Validate != "claude" {
		t.Errorf("Expected claude Validate phase to be \"claude\", got %q", claudeDef.Phases.Validate)
	}
	if claudeDef.Phases.Analyze != "claude" {
		t.Errorf("Expected claude Analyze phase to be \"claude\", got %q", claudeDef.Phases.Analyze)
	}

	// Check codex agent
	codexDef, ok := cfg.Agents.Definitions["codex"]
	if !ok {
		t.Fatal("Expected codex agent definition to be loaded")
	}
	if codexDef.Phases.Build != "codex" {
		t.Errorf("Expected codex Build phase to be \"codex\", got %q", codexDef.Phases.Build)
	}
	if codexDef.Phases.Review != "codex" {
		t.Errorf("Expected codex Review phase to be \"codex\", got %q", codexDef.Phases.Review)
	}
}

// TestLoadEmptyAgentsConfigAppliesDefaults verifies that when agents config
// is not specified in YAML, Load() applies defaults through SetDefaults().
func TestLoadEmptyAgentsConfigAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `models:
  p0: opus
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Definitions should be initialized to empty map by NormalizeNilFields
	if cfg.Agents.Definitions == nil {
		t.Error("Expected Agents.Definitions to be non-nil after Load")
	}

	// SetDefaults should have initialized the claude agent with default phases
	claudeDef, ok := cfg.Agents.Definitions["claude"]
	if !ok {
		t.Fatal("Expected claude agent definition to be created by defaults")
	}

	// All phases should default to "claude"
	if claudeDef.Phases.Build != "claude" {
		t.Errorf("Expected Build to default to \"claude\", got %q", claudeDef.Phases.Build)
	}
	if claudeDef.Phases.Validate != "claude" {
		t.Errorf("Expected Validate to default to \"claude\", got %q", claudeDef.Phases.Validate)
	}
	if claudeDef.Phases.Analyze != "claude" {
		t.Errorf("Expected Analyze to default to \"claude\", got %q", claudeDef.Phases.Analyze)
	}
	if claudeDef.Phases.Retro != "claude" {
		t.Errorf("Expected Retro to default to \"claude\", got %q", claudeDef.Phases.Retro)
	}
	if claudeDef.Phases.Scope != "claude" {
		t.Errorf("Expected Scope to default to \"claude\", got %q", claudeDef.Phases.Scope)
	}
	if claudeDef.Phases.Decompose != "claude" {
		t.Errorf("Expected Decompose to default to \"claude\", got %q", claudeDef.Phases.Decompose)
	}
	if claudeDef.Phases.Review != "claude" {
		t.Errorf("Expected Review to default to \"claude\", got %q", claudeDef.Phases.Review)
	}
}

// TestPartialAgentsConfigMergesDefaults verifies that when some agent definitions
// exist but don't have all phases set, SetDefaults() fills in missing phases.
func TestPartialAgentsConfigMergesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `agents:
  definitions:
    claude:
      phases:
        build: opus-model
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	claudeDef := cfg.Agents.Definitions["claude"]

	// Explicitly set phase should be preserved
	if claudeDef.Phases.Build != "opus-model" {
		t.Errorf("Expected Build to be \"opus-model\", got %q", claudeDef.Phases.Build)
	}

	// Unset phases should default to "claude"
	if claudeDef.Phases.Validate != "claude" {
		t.Errorf("Expected Validate to default to \"claude\", got %q", claudeDef.Phases.Validate)
	}
	if claudeDef.Phases.Review != "claude" {
		t.Errorf("Expected Review to default to \"claude\", got %q", claudeDef.Phases.Review)
	}
}

// TestAgentsConfigStructFieldsHaveYAMLTags verifies that the struct fields
// have correct YAML tags by attempting to unmarshal YAML with snake_case keys.
func TestAgentsConfigStructFieldsHaveYAMLTags(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")

	// Use YAML keys that would only work if yaml tags are correct
	yaml := `agents:
  definitions:
    test-agent:
      phases:
        build: test-build
        validate: test-validate
        analyze: test-analyze
        retro: test-retro
        scope: test-scope
        decompose: test-decompose
        review: test-review
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	testDef, ok := cfg.Agents.Definitions["test-agent"]
	if !ok {
		t.Fatal("Expected test-agent definition to be loaded")
	}

	// Verify all fields were unmarshaled correctly (proves YAML tags are correct)
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"Build", testDef.Phases.Build, "test-build"},
		{"Validate", testDef.Phases.Validate, "test-validate"},
		{"Analyze", testDef.Phases.Analyze, "test-analyze"},
		{"Retro", testDef.Phases.Retro, "test-retro"},
		{"Scope", testDef.Phases.Scope, "test-scope"},
		{"Decompose", testDef.Phases.Decompose, "test-decompose"},
		{"Review", testDef.Phases.Review, "test-review"},
	}

	for _, tt := range tests {
		if tt.value != tt.want {
			t.Errorf("Phase.%s: expected %q, got %q", tt.name, tt.want, tt.value)
		}
	}
}
