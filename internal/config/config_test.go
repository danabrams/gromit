package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestAgentsConfigUnmarshal(t *testing.T) {
	yamlContent := `
agents:
  definitions:
    claude: {}
    custom:
      binary: "my-agent"
      flags: ["--flag1"]
  phases:
    refine: claude
    plan: custom
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

	if cfg.Agents.Definitions == nil {
		t.Fatal("Agents.Definitions is nil, want non-nil map")
	}
	if len(cfg.Agents.Definitions) != 2 {
		t.Errorf("len(Agents.Definitions) = %d, want 2", len(cfg.Agents.Definitions))
	}

	claudeDef, ok := cfg.Agents.Definitions["claude"]
	if !ok {
		t.Fatal("claude definition not found")
	}
	if claudeDef.Binary != "" {
		t.Errorf("claude.Binary = %q, want empty (should use defaults)", claudeDef.Binary)
	}

	customDef, ok := cfg.Agents.Definitions["custom"]
	if !ok {
		t.Fatal("custom definition not found")
	}
	if customDef.Binary != "my-agent" {
		t.Errorf("custom.Binary = %q, want %q", customDef.Binary, "my-agent")
	}
	if len(customDef.Flags) != 1 || customDef.Flags[0] != "--flag1" {
		t.Errorf("custom.Flags = %v, want [--flag1]", customDef.Flags)
	}

	if cfg.Agents.Phases.Refine != "claude" {
		t.Errorf("Agents.Phases.Refine = %q, want %q", cfg.Agents.Phases.Refine, "claude")
	}
	if cfg.Agents.Phases.Plan != "custom" {
		t.Errorf("Agents.Phases.Plan = %q, want %q", cfg.Agents.Phases.Plan, "custom")
	}
}

func TestNormalizeNilFieldsInitializesAgentFlags(t *testing.T) {
	cfg := &Config{
		Agents: AgentsConfig{
			Definitions: map[string]AgentDefinition{
				"agent1": {Binary: "test"},
			},
		},
	}
	cfg.NormalizeNilFields()

	if cfg.Agents.Definitions["agent1"].Flags == nil {
		t.Error("agent1.Flags is nil, want empty slice")
	}
	if len(cfg.Agents.Definitions["agent1"].Flags) != 0 {
		t.Errorf("len(agent1.Flags) = %d, want 0", len(cfg.Agents.Definitions["agent1"].Flags))
	}
}

func TestNormalizeNilFieldsInitializesDefinitionsMap(t *testing.T) {
	cfg := &Config{}
	cfg.NormalizeNilFields()

	if cfg.Agents.Definitions == nil {
		t.Error("Agents.Definitions is nil, want empty map")
	}
	if len(cfg.Agents.Definitions) != 0 {
		t.Errorf("len(Agents.Definitions) = %d, want 0", len(cfg.Agents.Definitions))
	}
}

func TestNormalizeNilFields_MandatoryCommandsInitialized(t *testing.T) {
	cfg := &Config{}
	cfg.NormalizeNilFields()

	if cfg.Validation.MandatoryCommands == nil {
		t.Error("Validation.MandatoryCommands is nil, want empty slice")
	}
	if len(cfg.Validation.MandatoryCommands) != 0 {
		t.Errorf("len(Validation.MandatoryCommands) = %d, want 0", len(cfg.Validation.MandatoryCommands))
	}
}

func TestCompileCommandYAMLDeserialization(t *testing.T) {
	yamlContent := `
preflight:
  compile_command: "go build ./..."
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

	if cfg.Preflight.CompileCommand != "go build ./..." {
		t.Errorf("Preflight.CompileCommand = %q, want %q", cfg.Preflight.CompileCommand, "go build ./...")
	}
}

func TestMandatoryCommandsYAMLDeserialization(t *testing.T) {
	yamlContent := `
validation:
  mandatory_commands:
    - "go vet ./..."
    - "go build ./..."
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

	if len(cfg.Validation.MandatoryCommands) != 2 {
		t.Fatalf("len(MandatoryCommands) = %d, want 2", len(cfg.Validation.MandatoryCommands))
	}
	if cfg.Validation.MandatoryCommands[0] != "go vet ./..." {
		t.Errorf("MandatoryCommands[0] = %q, want %q", cfg.Validation.MandatoryCommands[0], "go vet ./...")
	}
	if cfg.Validation.MandatoryCommands[1] != "go build ./..." {
		t.Errorf("MandatoryCommands[1] = %q, want %q", cfg.Validation.MandatoryCommands[1], "go build ./...")
	}
}

func TestSetDefaultsRunbookTTLDays(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Runbook.TTLDays != DefaultRunbookTTLDays {
		t.Errorf("Runbook.TTLDays = %d, want %d", cfg.Runbook.TTLDays, DefaultRunbookTTLDays)
	}
}

func TestRunbookConfigYAMLDeserialization(t *testing.T) {
	yamlContent := `
runbook:
  ttl_days: 30
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

	if cfg.Runbook.TTLDays != 30 {
		t.Errorf("Runbook.TTLDays = %d, want 30", cfg.Runbook.TTLDays)
	}
}

func TestRunbookTTLDaysDefaultWhenOmittedFromYAML(t *testing.T) {
	yamlContent := `
models:
  p0: opus
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

	if cfg.Runbook.TTLDays != DefaultRunbookTTLDays {
		t.Errorf("Runbook.TTLDays = %d, want %d when omitted from YAML", cfg.Runbook.TTLDays, DefaultRunbookTTLDays)
	}
}

func TestMethodologyConfigSupportsBuildStrategyField(t *testing.T) {
	cfg := Config{
		Methodology: MethodologyConfig{
			BuildStrategy: "single_pass",
		},
	}

	if cfg.Methodology.BuildStrategy != "single_pass" {
		t.Fatalf("Methodology.BuildStrategy = %q, want %q", cfg.Methodology.BuildStrategy, "single_pass")
	}
}

func TestSetDefaultsMethodologyBuildStrategy_DefaultsToSinglePass(t *testing.T) {
	cfg := &Config{}

	cfg.SetDefaults()

	if cfg.Methodology.BuildStrategy != "single_pass" {
		t.Fatalf("Methodology.BuildStrategy = %q, want %q", cfg.Methodology.BuildStrategy, "single_pass")
	}
}

func TestMethodologyConfigSupportsPhaseModelsField(t *testing.T) {
	cfg := Config{
		Methodology: MethodologyConfig{
			PhaseModels: PhaseModelsConfig{
				Decompose: "medium",
				Build:     "medium",
				Red:       "low",
				Green:     "medium",
				Refactor:  "low",
			},
		},
	}

	if cfg.Methodology.PhaseModels.Decompose != "medium" {
		t.Fatalf("Methodology.PhaseModels.Decompose = %q, want %q", cfg.Methodology.PhaseModels.Decompose, "medium")
	}
	if cfg.Methodology.PhaseModels.Build != "medium" {
		t.Fatalf("Methodology.PhaseModels.Build = %q, want %q", cfg.Methodology.PhaseModels.Build, "medium")
	}
	if cfg.Methodology.PhaseModels.Red != "low" {
		t.Fatalf("Methodology.PhaseModels.Red = %q, want %q", cfg.Methodology.PhaseModels.Red, "low")
	}
	if cfg.Methodology.PhaseModels.Green != "medium" {
		t.Fatalf("Methodology.PhaseModels.Green = %q, want %q", cfg.Methodology.PhaseModels.Green, "medium")
	}
	if cfg.Methodology.PhaseModels.Refactor != "low" {
		t.Fatalf("Methodology.PhaseModels.Refactor = %q, want %q", cfg.Methodology.PhaseModels.Refactor, "low")
	}
}

func TestSetDefaultsMethodologyPhaseModels_SetsDefaultTiers(t *testing.T) {
	cfg := &Config{}

	cfg.SetDefaults()

	if cfg.Methodology.PhaseModels.Decompose != "medium" {
		t.Fatalf("Methodology.PhaseModels.Decompose = %q, want %q", cfg.Methodology.PhaseModels.Decompose, "medium")
	}
	if cfg.Methodology.PhaseModels.Build != "medium" {
		t.Fatalf("Methodology.PhaseModels.Build = %q, want %q", cfg.Methodology.PhaseModels.Build, "medium")
	}
	if cfg.Methodology.PhaseModels.Red != "low" {
		t.Fatalf("Methodology.PhaseModels.Red = %q, want %q", cfg.Methodology.PhaseModels.Red, "low")
	}
	if cfg.Methodology.PhaseModels.Green != "medium" {
		t.Fatalf("Methodology.PhaseModels.Green = %q, want %q", cfg.Methodology.PhaseModels.Green, "medium")
	}
	if cfg.Methodology.PhaseModels.Refactor != "low" {
		t.Fatalf("Methodology.PhaseModels.Refactor = %q, want %q", cfg.Methodology.PhaseModels.Refactor, "low")
	}
}

func TestNormalizeNilFieldsMethodologyAndRefactor_SetsSafeDefaults(t *testing.T) {
	cfg := &Config{}

	cfg.NormalizeNilFields()

	if cfg.Methodology.BuildStrategy != "single_pass" {
		t.Fatalf("Methodology.BuildStrategy = %q, want %q", cfg.Methodology.BuildStrategy, "single_pass")
	}
	if cfg.Methodology.PhaseModels.Decompose != "medium" {
		t.Fatalf("Methodology.PhaseModels.Decompose = %q, want %q", cfg.Methodology.PhaseModels.Decompose, "medium")
	}
	if cfg.Methodology.PhaseModels.Build != "medium" {
		t.Fatalf("Methodology.PhaseModels.Build = %q, want %q", cfg.Methodology.PhaseModels.Build, "medium")
	}
	if cfg.Methodology.PhaseModels.Red != "low" {
		t.Fatalf("Methodology.PhaseModels.Red = %q, want %q", cfg.Methodology.PhaseModels.Red, "low")
	}
	if cfg.Methodology.PhaseModels.Green != "medium" {
		t.Fatalf("Methodology.PhaseModels.Green = %q, want %q", cfg.Methodology.PhaseModels.Green, "medium")
	}
	if cfg.Methodology.PhaseModels.Refactor != "low" {
		t.Fatalf("Methodology.PhaseModels.Refactor = %q, want %q", cfg.Methodology.PhaseModels.Refactor, "low")
	}
	if cfg.Refactor.MinFilesChanged != 3 {
		t.Fatalf("Refactor.MinFilesChanged = %d, want %d", cfg.Refactor.MinFilesChanged, 3)
	}
}

func TestMethodologyBuildStrategyAndPhaseModels_ParseFromFullYAML(t *testing.T) {
	yamlContent := `methodology:
  build_strategy: "  single_pass  "
  phase_models:
    decompose: " HIGH "
    build: " MEDIUM "
    red: " LOW "
    green: " MEDIUM "
    refactor: " LOW "
`
	cfg := loadConfigFromYAML(t, yamlContent)

	if cfg.Methodology.BuildStrategy != "single_pass" {
		t.Fatalf("Methodology.BuildStrategy = %q, want %q", cfg.Methodology.BuildStrategy, "single_pass")
	}
	if cfg.Methodology.PhaseModels.Decompose != "high" {
		t.Fatalf("Methodology.PhaseModels.Decompose = %q, want %q", cfg.Methodology.PhaseModels.Decompose, "high")
	}
	if cfg.Methodology.PhaseModels.Build != "medium" {
		t.Fatalf("Methodology.PhaseModels.Build = %q, want %q", cfg.Methodology.PhaseModels.Build, "medium")
	}
	if cfg.Methodology.PhaseModels.Red != "low" {
		t.Fatalf("Methodology.PhaseModels.Red = %q, want %q", cfg.Methodology.PhaseModels.Red, "low")
	}
	if cfg.Methodology.PhaseModels.Green != "medium" {
		t.Fatalf("Methodology.PhaseModels.Green = %q, want %q", cfg.Methodology.PhaseModels.Green, "medium")
	}
	if cfg.Methodology.PhaseModels.Refactor != "low" {
		t.Fatalf("Methodology.PhaseModels.Refactor = %q, want %q", cfg.Methodology.PhaseModels.Refactor, "low")
	}
}

func TestMethodologyBuildStrategyAndPhaseModels_DefaultFromPartialYAML(t *testing.T) {
	yamlContent := `methodology:
  build_strategy: " SINGLE_PASS "
  phase_models:
    red: " HIGH "
`
	cfg := loadConfigFromYAML(t, yamlContent)

	if cfg.Methodology.BuildStrategy != "single_pass" {
		t.Fatalf("Methodology.BuildStrategy = %q, want %q", cfg.Methodology.BuildStrategy, "single_pass")
	}
	if cfg.Methodology.PhaseModels.Decompose != "medium" {
		t.Fatalf("Methodology.PhaseModels.Decompose = %q, want %q", cfg.Methodology.PhaseModels.Decompose, "medium")
	}
	if cfg.Methodology.PhaseModels.Build != "medium" {
		t.Fatalf("Methodology.PhaseModels.Build = %q, want %q", cfg.Methodology.PhaseModels.Build, "medium")
	}
	if cfg.Methodology.PhaseModels.Red != "high" {
		t.Fatalf("Methodology.PhaseModels.Red = %q, want %q", cfg.Methodology.PhaseModels.Red, "high")
	}
	if cfg.Methodology.PhaseModels.Green != "medium" {
		t.Fatalf("Methodology.PhaseModels.Green = %q, want %q", cfg.Methodology.PhaseModels.Green, "medium")
	}
	if cfg.Methodology.PhaseModels.Refactor != "low" {
		t.Fatalf("Methodology.PhaseModels.Refactor = %q, want %q", cfg.Methodology.PhaseModels.Refactor, "low")
	}
}

func TestMethodologyBuildStrategyAndPhaseModels_DefaultFromEmptyYAML(t *testing.T) {
	cfg := loadConfigFromYAML(t, "")

	if cfg.Methodology.BuildStrategy != defaultMethodologyBuildStrategy {
		t.Fatalf("Methodology.BuildStrategy = %q, want %q", cfg.Methodology.BuildStrategy, defaultMethodologyBuildStrategy)
	}
	if cfg.Methodology.PhaseModels.Decompose != "medium" {
		t.Fatalf("Methodology.PhaseModels.Decompose = %q, want %q", cfg.Methodology.PhaseModels.Decompose, "medium")
	}
	if cfg.Methodology.PhaseModels.Build != "medium" {
		t.Fatalf("Methodology.PhaseModels.Build = %q, want %q", cfg.Methodology.PhaseModels.Build, "medium")
	}
	if cfg.Methodology.PhaseModels.Red != "low" {
		t.Fatalf("Methodology.PhaseModels.Red = %q, want %q", cfg.Methodology.PhaseModels.Red, "low")
	}
	if cfg.Methodology.PhaseModels.Green != "medium" {
		t.Fatalf("Methodology.PhaseModels.Green = %q, want %q", cfg.Methodology.PhaseModels.Green, "medium")
	}
	if cfg.Methodology.PhaseModels.Refactor != "low" {
		t.Fatalf("Methodology.PhaseModels.Refactor = %q, want %q", cfg.Methodology.PhaseModels.Refactor, "low")
	}
}

func TestSetDefaultsInitializesPhasesToClaude(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Agents.Phases.Refine != "claude" {
		t.Errorf("Phases.Refine = %q, want %q", cfg.Agents.Phases.Refine, "claude")
	}
	if cfg.Agents.Phases.Plan != "claude" {
		t.Errorf("Phases.Plan = %q, want %q", cfg.Agents.Phases.Plan, "claude")
	}
	if cfg.Agents.Phases.Review != "claude" {
		t.Errorf("Phases.Review = %q, want %q", cfg.Agents.Phases.Review, "claude")
	}
	if cfg.Agents.Phases.Explore != "claude" {
		t.Errorf("Phases.Explore = %q, want %q", cfg.Agents.Phases.Explore, "claude")
	}
	if cfg.Agents.Phases.Debug != "claude" {
		t.Errorf("Phases.Debug = %q, want %q", cfg.Agents.Phases.Debug, "claude")
	}
}

func TestSelectModelBasics(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		priority int
		labels   []string
		want     string
	}{
		{
			name:     "NilReceiver",
			cfg:      nil,
			priority: 1,
			labels:   nil,
			want:     "sonnet",
		},
		{
			name:     "NilLabels",
			cfg:      &Config{Models: ModelsConfig{P0: "opus", P1: "sonnet", P2: "haiku"}},
			priority: 0,
			labels:   nil,
			want:     "opus",
		},
		{
			name:     "P0",
			cfg:      &Config{Models: ModelsConfig{P0: "opus", P1: "sonnet", P2: "haiku"}},
			priority: 0,
			labels:   nil,
			want:     "opus",
		},
		{
			name:     "P1",
			cfg:      &Config{Models: ModelsConfig{P0: "opus", P1: "sonnet", P2: "haiku"}},
			priority: 1,
			labels:   nil,
			want:     "sonnet",
		},
		{
			name:     "P2",
			cfg:      &Config{Models: ModelsConfig{P0: "opus", P1: "sonnet", P2: "haiku"}},
			priority: 2,
			labels:   nil,
			want:     "haiku",
		},
		{
			name:     "UnknownPriority",
			cfg:      &Config{Models: ModelsConfig{P0: "opus", P1: "sonnet", P2: "haiku"}},
			priority: 99,
			labels:   nil,
			want:     "sonnet",
		},
		{
			name: "LabelOverride",
			cfg: &Config{Models: ModelsConfig{
				P0: "opus", P1: "sonnet", P2: "haiku",
				Labels: map[string]string{"complexity:high": "opus"},
			}},
			priority: 1,
			labels:   []string{"complexity:high"},
			want:     "opus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg *Config
			if tt.cfg != nil {
				cfg = tt.cfg
			}
			result := cfg.SelectModel(tt.priority, tt.labels)
			if result != tt.want {
				t.Errorf("SelectModel(%d, %v) = %q, want %q", tt.priority, tt.labels, result, tt.want)
			}
		})
	}
}

func TestNextEscalationModelBasics(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		current string
		want    string
	}{
		{
			name:    "NilReceiver",
			cfg:     nil,
			current: "haiku",
			want:    "",
		},
		{
			name: "Disabled",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: false,
				Chain:   []string{"haiku", "sonnet", "opus"},
			}},
			current: "haiku",
			want:    "",
		},
		{
			name: "StartOfChain",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"haiku", "sonnet", "opus"},
			}},
			current: "haiku",
			want:    "sonnet",
		},
		{
			name: "MiddleOfChain",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"haiku", "sonnet", "opus"},
			}},
			current: "sonnet",
			want:    "opus",
		},
		{
			name: "EndOfChain",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"haiku", "sonnet", "opus"},
			}},
			current: "opus",
			want:    "",
		},
		{
			name: "NotInChain",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"haiku", "sonnet", "opus"},
			}},
			current: "unknown",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg *Config
			if tt.cfg != nil {
				cfg = tt.cfg
			}
			result := cfg.NextEscalationModel(tt.current)
			if result != tt.want {
				t.Errorf("NextEscalationModel(%q) = %q, want %q", tt.current, result, tt.want)
			}
		})
	}
}

func TestLoadAndDefaults(t *testing.T) {
	// Create a minimal config file
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("models:\n  p0: opus\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Check defaults were applied
	if cfg.Models.P1 != "sonnet" {
		t.Errorf("expected default P1='sonnet', got %q", cfg.Models.P1)
	}
	if cfg.Claude.Binary != "claude" {
		t.Errorf("expected default binary='claude', got %q", cfg.Claude.Binary)
	}
	if cfg.Paths.GromitDir != ".gromit" {
		t.Errorf("expected default GromitDir='.gromit', got %q", cfg.Paths.GromitDir)
	}
}

func TestExperimentDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Experiment.MinSampleSize != 20 {
		t.Fatalf("expected Experiment.MinSampleSize=20, got %d", cfg.Experiment.MinSampleSize)
	}
	if cfg.Experiment.ConfidenceThreshold != 0.95 {
		t.Fatalf("expected Experiment.ConfidenceThreshold=0.95, got %f", cfg.Experiment.ConfidenceThreshold)
	}
	expectedDir := filepath.Join(cfg.Paths.GromitDir, "experiments")
	if cfg.Experiment.ExperimentsDir != expectedDir {
		t.Fatalf("expected Experiment.ExperimentsDir=%q, got %q", expectedDir, cfg.Experiment.ExperimentsDir)
	}
}

func TestValidateRejectsInvalidExperimentConfig(t *testing.T) {
	tests := []struct {
		name             string
		minSampleSize    int
		confidenceThresh float64
		wantErr          string
	}{
		{
			name:          "MinSampleSizeTooLow",
			minSampleSize: 0,
			wantErr:       "experiment.min_sample_size",
		},
		{
			name:             "ThresholdTooHigh",
			minSampleSize:    20,
			confidenceThresh: 1.1,
			wantErr:          "experiment.confidence_threshold",
		},
		{
			name:             "ThresholdTooLow",
			minSampleSize:    20,
			confidenceThresh: 0,
			wantErr:          "experiment.confidence_threshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.SetDefaults()
			cfg.Experiment.MinSampleSize = tt.minSampleSize
			cfg.Experiment.ConfidenceThreshold = tt.confidenceThresh
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestStallTimeoutDefault(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if cfg.Claude.StallTimeout != DefaultStallTimeoutSeconds {
		t.Errorf("expected default StallTimeout=%d, got %d", DefaultStallTimeoutSeconds, cfg.Claude.StallTimeout)
	}
}

func TestPipelineTimeoutDefault(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if cfg.Claude.PipelineTimeout != 1800 {
		t.Errorf("expected default PipelineTimeout=1800, got %d", cfg.Claude.PipelineTimeout)
	}
}

func TestStallTimeoutActiveDefault(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if cfg.Claude.StallTimeoutActive != DefaultStallTimeoutActiveSeconds {
		t.Errorf("expected default StallTimeoutActive=%d, got %d", DefaultStallTimeoutActiveSeconds, cfg.Claude.StallTimeoutActive)
	}
}

func TestStallTimeoutActiveFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("claude:\n  stall_timeout_active: 180\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Claude.StallTimeoutActive != 180 {
		t.Errorf("expected StallTimeoutActive=180, got %d", cfg.Claude.StallTimeoutActive)
	}
}

func TestStallTimeoutFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("claude:\n  stall_timeout: 60\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Claude.StallTimeout != 60 {
		t.Errorf("expected StallTimeout=60, got %d", cfg.Claude.StallTimeout)
	}
}

func TestPipelineTimeoutFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("claude:\n  pipeline_timeout: 2400\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Claude.PipelineTimeout != 2400 {
		t.Errorf("expected PipelineTimeout=2400, got %d", cfg.Claude.PipelineTimeout)
	}
}

func TestPipelineTimeoutDefaultWhenOmittedFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("models:\n  p0: opus\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Claude.PipelineTimeout != DefaultPipelineTimeoutSeconds {
		t.Errorf("expected PipelineTimeout=%d (default), got %d", DefaultPipelineTimeoutSeconds, cfg.Claude.PipelineTimeout)
	}
}

func TestClaudeFailureContextAndTokenBudgetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Claude.MaxFailureContextChars != 2000 {
		t.Errorf("expected MaxFailureContextChars=2000, got %d", cfg.Claude.MaxFailureContextChars)
	}
	if cfg.Claude.MaxInputTokensPerBead != 400000 {
		t.Errorf("expected MaxInputTokensPerBead=400000, got %d", cfg.Claude.MaxInputTokensPerBead)
	}
}

func TestClaudeFailureContextAndTokenBudgetFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yamlContent := `
claude:
  max_failure_context_chars: 4096
  max_input_tokens_per_bead: 250000
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Claude.MaxFailureContextChars != 4096 {
		t.Errorf("expected MaxFailureContextChars=4096, got %d", cfg.Claude.MaxFailureContextChars)
	}
	if cfg.Claude.MaxInputTokensPerBead != 250000 {
		t.Errorf("expected MaxInputTokensPerBead=250000, got %d", cfg.Claude.MaxInputTokensPerBead)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/gromit.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("invalid: [yaml: {broken"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestSetDefaultsInitializesLabelsMap(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if cfg.Models.Labels == nil {
		t.Error("expected Models.Labels to be initialized, got nil")
	}
	// Ensure we can write to the map without panic
	cfg.Models.Labels["test"] = "value"
	if cfg.Models.Labels["test"] != "value" {
		t.Errorf("expected 'value', got %q", cfg.Models.Labels["test"])
	}
}

func TestSetDefaultsPreservesExistingLabels(t *testing.T) {
	cfg := &Config{
		Models: ModelsConfig{
			Labels: map[string]string{"complexity:high": "opus"},
		},
	}
	cfg.SetDefaults()
	if cfg.Models.Labels["complexity:high"] != "opus" {
		t.Errorf("expected existing label preserved, got %q", cfg.Models.Labels["complexity:high"])
	}
}

func TestSelectModelNilLabelsMap(t *testing.T) {
	// Config with nil Labels map should not panic
	cfg := &Config{
		Models: ModelsConfig{
			P0: "opus",
			P1: "sonnet",
		},
	}
	// Don't call setDefaults - test with nil Labels
	result := cfg.SelectModel(1, []string{"complexity:high"})
	if result != "sonnet" {
		t.Errorf("expected 'sonnet' (fallback to priority), got %q", result)
	}
}

func TestMaxRetriesPerBeadDefault(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if cfg.Escalation.MaxRetriesPerBead != 10 {
		t.Errorf("expected default MaxRetriesPerBead=10, got %d", cfg.Escalation.MaxRetriesPerBead)
	}
}

func TestMaxRetriesPerBeadFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `escalation:
  max_retries_per_bead: 5
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Escalation.MaxRetriesPerBead != 5 {
		t.Errorf("expected MaxRetriesPerBead=5, got %d", cfg.Escalation.MaxRetriesPerBead)
	}
}

func TestSelectModelPriorityWithEmptyLabels(t *testing.T) {
	// Test priority-based selection with empty labels array
	cfg := &Config{}
	cfg.SetDefaults()

	tests := []struct {
		priority int
		want     string
	}{
		{0, "opus"},
		{1, "sonnet"},
		{2, "haiku"},
	}

	for _, tt := range tests {
		result := cfg.SelectModel(tt.priority, []string{})
		if result != tt.want {
			t.Errorf("SelectModel(%d, []) = %q, want %q", tt.priority, result, tt.want)
		}
	}
}

func TestSelectModelPriorityWithCustomModels(t *testing.T) {
	// Test priority-based selection with custom model values
	cfg := &Config{
		Models: ModelsConfig{
			P0: "custom-p0",
			P1: "custom-p1",
			P2: "custom-p2",
		},
	}

	tests := []struct {
		priority int
		want     string
	}{
		{0, "custom-p0"},
		{1, "custom-p1"},
		{2, "custom-p2"},
	}

	for _, tt := range tests {
		result := cfg.SelectModel(tt.priority, nil)
		if result != tt.want {
			t.Errorf("SelectModel(%d) with custom models = %q, want %q", tt.priority, result, tt.want)
		}
	}
}

func TestSelectModelPriorityIgnoresNonMatchingLabels(t *testing.T) {
	// Test that when no labels match, priority-based selection is used
	cfg := &Config{
		Models: ModelsConfig{
			P0:     "opus",
			P1:     "sonnet",
			P2:     "haiku",
			Labels: map[string]string{"complexity:high": "opus"},
		},
	}

	// These labels don't exist in the config, so should fall back to priority
	result := cfg.SelectModel(1, []string{"nonexistent", "another-nonexistent"})
	if result != "sonnet" {
		t.Errorf("SelectModel(1) with non-matching labels = %q, want 'sonnet'", result)
	}
}

func TestSelectModelPriorityDefaultWhenUnknown(t *testing.T) {
	// Test that unknown priority defaults to P1
	cfg := &Config{}
	cfg.SetDefaults()

	tests := []struct {
		priority int
		want     string
	}{
		{99, "sonnet"},
		{-1, "sonnet"},
		{100, "sonnet"},
	}

	for _, tt := range tests {
		result := cfg.SelectModel(tt.priority, nil)
		if result != tt.want {
			t.Errorf("SelectModel(%d) = %q, want %q", tt.priority, result, tt.want)
		}
	}
}

// Comprehensive tests for label overrides
func TestSelectModelLabelOverrideHighPriority(t *testing.T) {
	// Label override should take precedence over P0 (opus)
	cfg := &Config{
		Models: ModelsConfig{
			P0:     "opus",
			P1:     "sonnet",
			P2:     "haiku",
			Labels: map[string]string{"complexity:low": "haiku"},
		},
	}
	result := cfg.SelectModel(0, []string{"complexity:low"})
	if result != "haiku" {
		t.Errorf("label override should beat P0: expected 'haiku', got %q", result)
	}
}

func TestSelectModelLabelOverrideLowPriority(t *testing.T) {
	// Label override should take precedence over P2 (haiku)
	cfg := &Config{
		Models: ModelsConfig{
			P0:     "opus",
			P1:     "sonnet",
			P2:     "haiku",
			Labels: map[string]string{"complexity:high": "opus"},
		},
	}
	result := cfg.SelectModel(2, []string{"complexity:high"})
	if result != "opus" {
		t.Errorf("label override should beat P2: expected 'opus', got %q", result)
	}
}

func TestSelectModelMultipleLabelsFirstMatches(t *testing.T) {
	// When multiple labels are provided, should use first matching label
	cfg := &Config{
		Models: ModelsConfig{
			P0: "opus",
			P1: "sonnet",
			P2: "haiku",
			Labels: map[string]string{
				"complexity:high": "opus",
				"complexity:low":  "haiku",
				"spec:auth":       "sonnet",
			},
		},
	}
	// First label matches
	result := cfg.SelectModel(1, []string{"complexity:high", "complexity:low"})
	if result != "opus" {
		t.Errorf("expected first matching label 'opus', got %q", result)
	}
}

func TestSelectModelMultipleLabelsSecondMatches(t *testing.T) {
	// When multiple labels are provided, should use first matching label
	cfg := &Config{
		Models: ModelsConfig{
			P0: "opus",
			P1: "sonnet",
			P2: "haiku",
			Labels: map[string]string{
				"complexity:high": "opus",
				"complexity:low":  "haiku",
			},
		},
	}
	// Second label matches (first doesn't)
	result := cfg.SelectModel(1, []string{"nonexistent", "complexity:low"})
	if result != "haiku" {
		t.Errorf("expected second matching label 'haiku', got %q", result)
	}
}

func TestSelectModelLabelOverrideWithCustomModels(t *testing.T) {
	// Label override should work with custom model names
	cfg := &Config{
		Models: ModelsConfig{
			P0:     "custom-opus",
			P1:     "custom-sonnet",
			P2:     "custom-haiku",
			Labels: map[string]string{"priority:critical": "custom-opus"},
		},
	}
	result := cfg.SelectModel(2, []string{"priority:critical"})
	if result != "custom-opus" {
		t.Errorf("label override with custom models: expected 'custom-opus', got %q", result)
	}
}

func TestSelectModelLabelOverrideIgnoresPriority(t *testing.T) {
	// Demonstrates that label overrides completely ignore priority parameter
	cfg := &Config{
		Models: ModelsConfig{
			P0:     "opus",
			P1:     "sonnet",
			P2:     "haiku",
			Labels: map[string]string{"complexity:low": "haiku"},
		},
	}

	// Same label, different priorities should all return the label's model
	tests := []struct {
		priority int
		want     string
	}{
		{0, "haiku"},
		{1, "haiku"},
		{2, "haiku"},
		{99, "haiku"},
	}

	for _, tt := range tests {
		result := cfg.SelectModel(tt.priority, []string{"complexity:low"})
		if result != tt.want {
			t.Errorf("SelectModel(%d) with complexity:low label = %q, want %q", tt.priority, result, tt.want)
		}
	}
}

func TestSelectModelMultipleLabelsDifferentModels(t *testing.T) {
	// When multiple labels are provided and multiple match, first wins
	cfg := &Config{
		Models: ModelsConfig{
			P0: "opus",
			P1: "sonnet",
			P2: "haiku",
			Labels: map[string]string{
				"spec:auth": "opus",
				"spec:ui":   "haiku",
				"spec:data": "sonnet",
			},
		},
	}
	// All three labels match; should use first
	result := cfg.SelectModel(1, []string{"spec:auth", "spec:ui", "spec:data"})
	if result != "opus" {
		t.Errorf("expected first matching label 'opus', got %q", result)
	}
}

func TestSelectModelComplexityHighLabel(t *testing.T) {
	// Test the standard complexity:high label override from documentation
	cfg := &Config{
		Models: ModelsConfig{
			P0:     "opus",
			P1:     "sonnet",
			P2:     "haiku",
			Labels: map[string]string{"complexity:high": "opus"},
		},
	}

	tests := []struct {
		priority int
		labels   []string
		want     string
	}{
		{0, []string{"complexity:high"}, "opus"}, // P0 + label: should be opus
		{1, []string{"complexity:high"}, "opus"}, // P1 + label: should be opus
		{2, []string{"complexity:high"}, "opus"}, // P2 + label: should be opus
	}

	for _, tt := range tests {
		result := cfg.SelectModel(tt.priority, tt.labels)
		if result != tt.want {
			t.Errorf("SelectModel(%d, %v) = %q, want %q", tt.priority, tt.labels, result, tt.want)
		}
	}
}

func TestSelectModelComplexityLowLabel(t *testing.T) {
	// Test the standard complexity:low label override from documentation
	cfg := &Config{
		Models: ModelsConfig{
			P0:     "opus",
			P1:     "sonnet",
			P2:     "haiku",
			Labels: map[string]string{"complexity:low": "haiku"},
		},
	}

	tests := []struct {
		priority int
		labels   []string
		want     string
	}{
		{0, []string{"complexity:low"}, "haiku"}, // P0 + low complexity: should be haiku
		{1, []string{"complexity:low"}, "haiku"}, // P1 + low complexity: should be haiku
		{2, []string{"complexity:low"}, "haiku"}, // P2 + low complexity: should be haiku
	}

	for _, tt := range tests {
		result := cfg.SelectModel(tt.priority, tt.labels)
		if result != tt.want {
			t.Errorf("SelectModel(%d, %v) = %q, want %q", tt.priority, tt.labels, result, tt.want)
		}
	}
}

func TestSelectModelLabelOverrideEmptyConfig(t *testing.T) {
	// Label override with empty config should not panic
	cfg := &Config{
		Models: ModelsConfig{
			Labels: map[string]string{"custom": "haiku"},
		},
	}
	result := cfg.SelectModel(1, []string{"custom"})
	if result != "haiku" {
		t.Errorf("expected 'haiku' from label override, got %q", result)
	}
}

func TestSelectModelLabelNotFoundFallsBackToPriority(t *testing.T) {
	// If label doesn't exist in config, should fall back to priority
	cfg := &Config{
		Models: ModelsConfig{
			P0:     "opus",
			P1:     "sonnet",
			P2:     "haiku",
			Labels: map[string]string{"existing:label": "opus"},
		},
	}
	result := cfg.SelectModel(1, []string{"nonexistent:label"})
	if result != "sonnet" {
		t.Errorf("expected fallback to P1 'sonnet', got %q", result)
	}
}

func TestSelectModelSpecLabel(t *testing.T) {
	// Test spec labels work with overrides
	cfg := &Config{
		Models: ModelsConfig{
			P0:     "opus",
			P1:     "sonnet",
			P2:     "haiku",
			Labels: map[string]string{"spec:database": "opus"},
		},
	}
	result := cfg.SelectModel(2, []string{"spec:database"})
	if result != "opus" {
		t.Errorf("expected spec label override to 'opus', got %q", result)
	}
}

// Comprehensive tests for NextEscalationModel chain traversal

func TestNextEscalationModelStartOfChain(t *testing.T) {
	// Test starting at the beginning of escalation chain
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"haiku", "sonnet", "opus"},
		},
	}
	result := cfg.NextEscalationModel("haiku")
	if result != "sonnet" {
		t.Errorf("expected escalation from haiku to sonnet, got %q", result)
	}
}

func TestNextEscalationModelMiddleOfChain(t *testing.T) {
	// Test in the middle of escalation chain
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"haiku", "sonnet", "opus"},
		},
	}
	result := cfg.NextEscalationModel("sonnet")
	if result != "opus" {
		t.Errorf("expected escalation from sonnet to opus, got %q", result)
	}
}

func TestNextEscalationModelEndOfChain(t *testing.T) {
	// Test at the end of escalation chain
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"haiku", "sonnet", "opus"},
		},
	}
	result := cfg.NextEscalationModel("opus")
	if result != "" {
		t.Errorf("expected empty string at end of chain, got %q", result)
	}
}

func TestNextEscalationModelNotInChain(t *testing.T) {
	// Test model not in escalation chain
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"haiku", "sonnet", "opus"},
		},
	}
	result := cfg.NextEscalationModel("unknown-model")
	if result != "" {
		t.Errorf("expected empty string for model not in chain, got %q", result)
	}
}

func TestNextEscalationModelCustomChain(t *testing.T) {
	// Test with custom escalation chain
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"model-a", "model-b", "model-c", "model-d"},
		},
	}

	tests := []struct {
		current string
		want    string
	}{
		{"model-a", "model-b"},
		{"model-b", "model-c"},
		{"model-c", "model-d"},
		{"model-d", ""},
	}

	for _, tt := range tests {
		result := cfg.NextEscalationModel(tt.current)
		if result != tt.want {
			t.Errorf("NextEscalationModel(%q) = %q, want %q", tt.current, result, tt.want)
		}
	}
}

func TestNextEscalationModelSingleModelChain(t *testing.T) {
	// Test with chain containing only one model
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"only-model"},
		},
	}
	result := cfg.NextEscalationModel("only-model")
	if result != "" {
		t.Errorf("expected empty string for single-model chain, got %q", result)
	}
}

func TestNextEscalationModelTwoModelChain(t *testing.T) {
	// Test with chain containing two models
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"model-a", "model-b"},
		},
	}

	tests := []struct {
		current string
		want    string
	}{
		{"model-a", "model-b"},
		{"model-b", ""},
	}

	for _, tt := range tests {
		result := cfg.NextEscalationModel(tt.current)
		if result != tt.want {
			t.Errorf("NextEscalationModel(%q) = %q, want %q", tt.current, result, tt.want)
		}
	}
}

func TestNextEscalationModelEmptyChain(t *testing.T) {
	// Test with empty escalation chain
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{},
		},
	}
	result := cfg.NextEscalationModel("any-model")
	if result != "" {
		t.Errorf("expected empty string for empty chain, got %q", result)
	}
}

func TestNextEscalationModelCaseInsensitive(t *testing.T) {
	// Test that model matching is case-sensitive
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"haiku", "sonnet", "opus"},
		},
	}
	// Different case should not match
	result := cfg.NextEscalationModel("Haiku")
	if result != "" {
		t.Errorf("expected empty string for case-mismatched model, got %q", result)
	}
}

func TestNextEscalationModelWithWhitespace(t *testing.T) {
	// Test that whitespace in model names is not trimmed
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"haiku", "sonnet", "opus"},
		},
	}
	// Leading/trailing whitespace should not match
	result := cfg.NextEscalationModel(" haiku")
	if result != "" {
		t.Errorf("expected empty string for model with leading space, got %q", result)
	}

	result = cfg.NextEscalationModel("haiku ")
	if result != "" {
		t.Errorf("expected empty string for model with trailing space, got %q", result)
	}
}

func TestNextEscalationModelDefaultChain(t *testing.T) {
	// Test with default chain after setDefaults()
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Escalation.Enabled = true

	result := cfg.NextEscalationModel("haiku")
	if result != "sonnet" {
		t.Errorf("expected 'sonnet' with default chain, got %q", result)
	}
}

func TestNextEscalationModelLongChain(t *testing.T) {
	// Test with longer escalation chain
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"model-1", "model-2", "model-3", "model-4", "model-5"},
		},
	}

	tests := []struct {
		current string
		want    string
	}{
		{"model-1", "model-2"},
		{"model-2", "model-3"},
		{"model-3", "model-4"},
		{"model-4", "model-5"},
		{"model-5", ""},
	}

	for _, tt := range tests {
		result := cfg.NextEscalationModel(tt.current)
		if result != tt.want {
			t.Errorf("NextEscalationModel(%q) = %q, want %q", tt.current, result, tt.want)
		}
	}
}

func TestNextEscalationModelDisabledWithChain(t *testing.T) {
	// Verify escalation disabled prevents any escalation even with chain
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: false,
			Chain:   []string{"haiku", "sonnet", "opus"},
		},
	}

	tests := []struct {
		current string
	}{
		{"haiku"},
		{"sonnet"},
		{"opus"},
	}

	for _, tt := range tests {
		result := cfg.NextEscalationModel(tt.current)
		if result != "" {
			t.Errorf("NextEscalationModel(%q) with disabled escalation = %q, want empty", tt.current, result)
		}
	}
}

func TestNextEscalationModelDuplicateModelsInChain(t *testing.T) {
	// Test behavior with duplicate models in chain (edge case)
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"haiku", "sonnet", "haiku", "opus"},
		},
	}

	// Should match the first occurrence
	result := cfg.NextEscalationModel("haiku")
	if result != "sonnet" {
		t.Errorf("expected escalation to sonnet (first haiku match), got %q", result)
	}

	// Should match the second occurrence of haiku
	result = cfg.NextEscalationModel("sonnet")
	if result != "haiku" {
		t.Errorf("expected escalation to haiku (second in chain), got %q", result)
	}
}

func TestNextEscalationModelEmptyStringModel(t *testing.T) {
	// Test with empty string as current model
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"haiku", "sonnet", "opus"},
		},
	}
	result := cfg.NextEscalationModel("")
	if result != "" {
		t.Errorf("expected empty string for empty current model, got %q", result)
	}
}

// Comprehensive tests for config.Load() default values and YAML edge cases

func TestLoadEmptyFile(t *testing.T) {
	// Test loading an empty YAML file - should apply all defaults
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading empty config: %v", err)
	}

	// Check all defaults
	if cfg.Models.P0 != "opus" {
		t.Errorf("expected P0='opus', got %q", cfg.Models.P0)
	}
	if cfg.Models.P1 != "sonnet" {
		t.Errorf("expected P1='sonnet', got %q", cfg.Models.P1)
	}
	if cfg.Models.P2 != "haiku" {
		t.Errorf("expected P2='haiku', got %q", cfg.Models.P2)
	}
	if cfg.Models.Validation != "haiku" {
		t.Errorf("expected Validation='haiku', got %q", cfg.Models.Validation)
	}
	if cfg.Claude.Binary != "claude" {
		t.Errorf("expected Binary='claude', got %q", cfg.Claude.Binary)
	}
	if cfg.Claude.Timeout != 900 {
		t.Errorf("expected Timeout=900, got %d", cfg.Claude.Timeout)
	}
	if cfg.Claude.StallTimeout != 120 {
		t.Errorf("expected StallTimeout=120, got %d", cfg.Claude.StallTimeout)
	}
	if cfg.Claude.StallTimeoutActive != 300 {
		t.Errorf("expected StallTimeoutActive=300, got %d", cfg.Claude.StallTimeoutActive)
	}
	if cfg.Claude.BeadTimeout != 1200 {
		t.Errorf("expected BeadTimeout=1200, got %d", cfg.Claude.BeadTimeout)
	}
	if cfg.Claude.AnalysisTimeout != 120 {
		t.Errorf("expected AnalysisTimeout=120, got %d", cfg.Claude.AnalysisTimeout)
	}
	if cfg.Paths.GromitDir != ".gromit" {
		t.Errorf("expected GromitDir='.gromit', got %q", cfg.Paths.GromitDir)
	}
	if cfg.Paths.Templates != ".gromit/templates" {
		t.Errorf("expected Templates='.gromit/templates', got %q", cfg.Paths.Templates)
	}
	if cfg.Paths.Specs != ".gromit/specs" {
		t.Errorf("expected Specs='.gromit/specs', got %q", cfg.Paths.Specs)
	}
	if cfg.Paths.Plans != ".gromit/plans" {
		t.Errorf("expected Plans='.gromit/plans', got %q", cfg.Paths.Plans)
	}
	if cfg.Paths.Logs != ".gromit/logs" {
		t.Errorf("expected Logs='.gromit/logs', got %q", cfg.Paths.Logs)
	}
	if cfg.Paths.ProjectClaudeMD != "CLAUDE.md" {
		t.Errorf("expected ProjectClaudeMD='CLAUDE.md', got %q", cfg.Paths.ProjectClaudeMD)
	}
	if len(cfg.Escalation.Chain) != 3 || cfg.Escalation.Chain[0] != "haiku" {
		t.Errorf("expected default Chain, got %v", cfg.Escalation.Chain)
	}
	if cfg.Escalation.MaxRetriesPerModel != 1 {
		t.Errorf("expected MaxRetriesPerModel=1, got %d", cfg.Escalation.MaxRetriesPerModel)
	}
	if cfg.Escalation.MaxRetriesPerBead != 10 {
		t.Errorf("expected MaxRetriesPerBead=10, got %d", cfg.Escalation.MaxRetriesPerBead)
	}
	if cfg.Preflight.AutoInstall != "ask" {
		t.Errorf("expected AutoInstall='ask', got %q", cfg.Preflight.AutoInstall)
	}
	if cfg.Models.Labels == nil {
		t.Error("expected Models.Labels to be initialized")
	}
}

func TestLoadPartialYAML(t *testing.T) {
	// Test that defaults apply only to missing fields
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `models:
  p0: custom-opus
  p1: custom-sonnet
claude:
  timeout: 300
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Custom values should be preserved
	if cfg.Models.P0 != "custom-opus" {
		t.Errorf("expected P0='custom-opus', got %q", cfg.Models.P0)
	}
	if cfg.Models.P1 != "custom-sonnet" {
		t.Errorf("expected P1='custom-sonnet', got %q", cfg.Models.P1)
	}
	if cfg.Claude.Timeout != 300 {
		t.Errorf("expected Timeout=300, got %d", cfg.Claude.Timeout)
	}

	// Missing fields should get defaults
	if cfg.Models.P2 != "haiku" {
		t.Errorf("expected P2='haiku', got %q", cfg.Models.P2)
	}
	if cfg.Claude.Binary != "claude" {
		t.Errorf("expected Binary='claude', got %q", cfg.Claude.Binary)
	}
	if cfg.Claude.StallTimeout != 120 {
		t.Errorf("expected StallTimeout=120, got %d", cfg.Claude.StallTimeout)
	}
}

func TestLoadWithAllFields(t *testing.T) {
	// Test that all custom values are preserved when provided
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `models:
  p0: ultra
  p1: standard
  p2: light
  validation: light
  labels:
    "spec:auth": ultra
    "complexity:high": ultra
escalation:
  enabled: true
  chain: ["light", "standard", "ultra"]
  max_retries_per_model: 2
  max_retries_per_bead: 5
loop:
  max_iterations: 20
  stop_on_failure: true
validation:
  enabled: true
  commands:
    - "test"
    - "lint"
preflight:
  auto_install: always
  tools: ["git", "go"]
claude:
  binary: /usr/local/bin/claude
  timeout: 900
  stall_timeout: 60
  stall_timeout_active: 180
  bead_timeout: 2400
  analysis_timeout: 300
  flags:
    - "--dangerously-skip-permissions"
paths:
  gromit_dir: .custom-gromit
  templates: .custom-gromit/templates
  specs: .custom-gromit/specs
  plans: .custom-gromit/plans
  logs: .custom-gromit/logs
  project_claude_md: CUSTOM.md
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Verify all custom values are preserved
	if cfg.Models.P0 != "ultra" {
		t.Errorf("expected P0='ultra', got %q", cfg.Models.P0)
	}
	if cfg.Models.P1 != "standard" {
		t.Errorf("expected P1='standard', got %q", cfg.Models.P1)
	}
	if cfg.Models.P2 != "light" {
		t.Errorf("expected P2='light', got %q", cfg.Models.P2)
	}
	if cfg.Models.Validation != "light" {
		t.Errorf("expected Validation='light', got %q", cfg.Models.Validation)
	}
	if len(cfg.Models.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(cfg.Models.Labels))
	}
	if cfg.Models.Labels["spec:auth"] != "ultra" {
		t.Errorf("expected Labels[spec:auth]='ultra', got %q", cfg.Models.Labels["spec:auth"])
	}

	if cfg.Escalation.MaxRetriesPerModel != 2 {
		t.Errorf("expected MaxRetriesPerModel=2, got %d", cfg.Escalation.MaxRetriesPerModel)
	}
	if cfg.Escalation.MaxRetriesPerBead != 5 {
		t.Errorf("expected MaxRetriesPerBead=5, got %d", cfg.Escalation.MaxRetriesPerBead)
	}

	if cfg.Loop.MaxIterations != 20 {
		t.Errorf("expected MaxIterations=20, got %d", cfg.Loop.MaxIterations)
	}
	if !cfg.Loop.StopOnFailure {
		t.Errorf("expected StopOnFailure=true, got false")
	}

	if !cfg.Validation.Enabled {
		t.Errorf("expected Validation.Enabled=true, got false")
	}
	if len(cfg.Validation.Commands) != 2 {
		t.Errorf("expected 2 validation commands, got %d", len(cfg.Validation.Commands))
	}

	if cfg.Preflight.AutoInstall != "always" {
		t.Errorf("expected AutoInstall='always', got %q", cfg.Preflight.AutoInstall)
	}
	if len(cfg.Preflight.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(cfg.Preflight.Tools))
	}

	if cfg.Claude.Binary != "/usr/local/bin/claude" {
		t.Errorf("expected Binary='/usr/local/bin/claude', got %q", cfg.Claude.Binary)
	}
	if cfg.Claude.Timeout != 900 {
		t.Errorf("expected Timeout=900, got %d", cfg.Claude.Timeout)
	}
	if cfg.Claude.StallTimeout != 60 {
		t.Errorf("expected StallTimeout=60, got %d", cfg.Claude.StallTimeout)
	}
	if cfg.Claude.StallTimeoutActive != 180 {
		t.Errorf("expected StallTimeoutActive=180, got %d", cfg.Claude.StallTimeoutActive)
	}
	if cfg.Claude.BeadTimeout != 2400 {
		t.Errorf("expected BeadTimeout=2400, got %d", cfg.Claude.BeadTimeout)
	}
	if cfg.Claude.AnalysisTimeout != 300 {
		t.Errorf("expected AnalysisTimeout=300, got %d", cfg.Claude.AnalysisTimeout)
	}
	if len(cfg.Claude.Flags) != 1 {
		t.Errorf("expected 1 flag, got %d", len(cfg.Claude.Flags))
	}

	if cfg.Paths.GromitDir != ".custom-gromit" {
		t.Errorf("expected GromitDir='.custom-gromit', got %q", cfg.Paths.GromitDir)
	}
	if cfg.Paths.Templates != ".custom-gromit/templates" {
		t.Errorf("expected Templates='.custom-gromit/templates', got %q", cfg.Paths.Templates)
	}
	if cfg.Paths.Specs != ".custom-gromit/specs" {
		t.Errorf("expected Specs='.custom-gromit/specs', got %q", cfg.Paths.Specs)
	}
	if cfg.Paths.Plans != ".custom-gromit/plans" {
		t.Errorf("expected Plans='.custom-gromit/plans', got %q", cfg.Paths.Plans)
	}
	if cfg.Paths.Logs != ".custom-gromit/logs" {
		t.Errorf("expected Logs='.custom-gromit/logs', got %q", cfg.Paths.Logs)
	}
	if cfg.Paths.ProjectClaudeMD != "CUSTOM.md" {
		t.Errorf("expected ProjectClaudeMD='CUSTOM.md', got %q", cfg.Paths.ProjectClaudeMD)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	// Test error handling for missing file
	_, err := Load("/nonexistent/path/to/config.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "reading config file") {
		t.Errorf("expected 'reading config file' in error, got: %v", err)
	}
}

func TestLoadInvalidYAMLSyntax(t *testing.T) {
	// Test error handling for invalid YAML syntax
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("invalid: [yaml: {broken"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "parsing config file") {
		t.Errorf("expected 'parsing config file' in error, got: %v", err)
	}
}

func TestLoadZeroTimeouts(t *testing.T) {
	// Test that zero timeouts are replaced with defaults
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `claude:
  timeout: 0
  stall_timeout: 0
  stall_timeout_active: 0
  bead_timeout: 0
  analysis_timeout: 0
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Zero values should be replaced with defaults
	if cfg.Claude.Timeout != 900 {
		t.Errorf("expected Timeout=900, got %d", cfg.Claude.Timeout)
	}
	if cfg.Claude.StallTimeout != 120 {
		t.Errorf("expected StallTimeout=120, got %d", cfg.Claude.StallTimeout)
	}
	if cfg.Claude.StallTimeoutActive != 300 {
		t.Errorf("expected StallTimeoutActive=300, got %d", cfg.Claude.StallTimeoutActive)
	}
	if cfg.Claude.BeadTimeout != 1200 {
		t.Errorf("expected BeadTimeout=1200, got %d", cfg.Claude.BeadTimeout)
	}
	if cfg.Claude.AnalysisTimeout != 120 {
		t.Errorf("expected AnalysisTimeout=120, got %d", cfg.Claude.AnalysisTimeout)
	}
}

func TestLoadEmptyStrings(t *testing.T) {
	// Test that empty string fields are replaced with defaults
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `models:
  p0: ""
  p1: ""
  p2: ""
  validation: ""
claude:
  binary: ""
paths:
  gromit_dir: ""
  templates: ""
  specs: ""
  plans: ""
  logs: ""
  project_claude_md: ""
preflight:
  auto_install: ""
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Empty strings should be replaced with defaults
	if cfg.Models.P0 != "opus" {
		t.Errorf("expected P0='opus', got %q", cfg.Models.P0)
	}
	if cfg.Models.P1 != "sonnet" {
		t.Errorf("expected P1='sonnet', got %q", cfg.Models.P1)
	}
	if cfg.Models.P2 != "haiku" {
		t.Errorf("expected P2='haiku', got %q", cfg.Models.P2)
	}
	if cfg.Models.Validation != "haiku" {
		t.Errorf("expected Validation='haiku', got %q", cfg.Models.Validation)
	}
	if cfg.Claude.Binary != "claude" {
		t.Errorf("expected Binary='claude', got %q", cfg.Claude.Binary)
	}
	if cfg.Paths.GromitDir != ".gromit" {
		t.Errorf("expected GromitDir='.gromit', got %q", cfg.Paths.GromitDir)
	}
	if cfg.Paths.Templates != ".gromit/templates" {
		t.Errorf("expected Templates='.gromit/templates', got %q", cfg.Paths.Templates)
	}
	if cfg.Paths.Specs != ".gromit/specs" {
		t.Errorf("expected Specs='.gromit/specs', got %q", cfg.Paths.Specs)
	}
	if cfg.Paths.Plans != ".gromit/plans" {
		t.Errorf("expected Plans='.gromit/plans', got %q", cfg.Paths.Plans)
	}
	if cfg.Paths.Logs != ".gromit/logs" {
		t.Errorf("expected Logs='.gromit/logs', got %q", cfg.Paths.Logs)
	}
	if cfg.Paths.ProjectClaudeMD != "CLAUDE.md" {
		t.Errorf("expected ProjectClaudeMD='CLAUDE.md', got %q", cfg.Paths.ProjectClaudeMD)
	}
	if cfg.Preflight.AutoInstall != "ask" {
		t.Errorf("expected AutoInstall='ask', got %q", cfg.Preflight.AutoInstall)
	}
}

func TestLoadNilEscalationChain(t *testing.T) {
	// Test that nil escalation chain is replaced with default
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Escalation.Chain == nil {
		t.Error("expected Escalation.Chain to be initialized, got nil")
	}
	if len(cfg.Escalation.Chain) != 3 {
		t.Errorf("expected 3 models in default chain, got %d", len(cfg.Escalation.Chain))
	}
	if cfg.Escalation.Chain[0] != "haiku" || cfg.Escalation.Chain[1] != "sonnet" || cfg.Escalation.Chain[2] != "opus" {
		t.Errorf("expected default chain [haiku, sonnet, opus], got %v", cfg.Escalation.Chain)
	}
}

func TestLoadZeroEscalationRetries(t *testing.T) {
	// Test that zero escalation retry values are replaced with defaults
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `escalation:
  max_retries_per_model: 0
  max_retries_per_bead: 0
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Zero values should be replaced with defaults
	if cfg.Escalation.MaxRetriesPerModel != 1 {
		t.Errorf("expected MaxRetriesPerModel=1, got %d", cfg.Escalation.MaxRetriesPerModel)
	}
	if cfg.Escalation.MaxRetriesPerBead != 10 {
		t.Errorf("expected MaxRetriesPerBead=10, got %d", cfg.Escalation.MaxRetriesPerBead)
	}
}

func TestLoadPreservesNonZeroRetries(t *testing.T) {
	// Test that non-zero retry values are preserved
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `escalation:
  max_retries_per_model: 3
  max_retries_per_bead: 15
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Escalation.MaxRetriesPerModel != 3 {
		t.Errorf("expected MaxRetriesPerModel=3, got %d", cfg.Escalation.MaxRetriesPerModel)
	}
	if cfg.Escalation.MaxRetriesPerBead != 15 {
		t.Errorf("expected MaxRetriesPerBead=15, got %d", cfg.Escalation.MaxRetriesPerBead)
	}
}

func TestLoadEmptyEscalationChain(t *testing.T) {
	// Test that empty escalation chain is replaced with default
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `escalation:
  chain: []
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if len(cfg.Escalation.Chain) != 3 {
		t.Errorf("expected 3 models in default chain, got %d", len(cfg.Escalation.Chain))
	}
	if cfg.Escalation.Chain[0] != "haiku" || cfg.Escalation.Chain[1] != "sonnet" || cfg.Escalation.Chain[2] != "opus" {
		t.Errorf("expected default chain [haiku, sonnet, opus], got %v", cfg.Escalation.Chain)
	}
}

func TestLoadCustomEscalationChain(t *testing.T) {
	// Test that custom escalation chain is preserved
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `escalation:
  chain: ["model-a", "model-b", "model-c"]
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if len(cfg.Escalation.Chain) != 3 {
		t.Errorf("expected 3 models in chain, got %d", len(cfg.Escalation.Chain))
	}
	if cfg.Escalation.Chain[0] != "model-a" || cfg.Escalation.Chain[1] != "model-b" || cfg.Escalation.Chain[2] != "model-c" {
		t.Errorf("expected custom chain, got %v", cfg.Escalation.Chain)
	}
}

func TestLoadNilLabelsMap(t *testing.T) {
	// Test that nil labels map is initialized as empty
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Models.Labels == nil {
		t.Error("expected Models.Labels to be initialized, got nil")
	}
	if len(cfg.Models.Labels) != 0 {
		t.Errorf("expected empty Labels map, got %v", cfg.Models.Labels)
	}

	// Ensure we can write to the map without panic
	cfg.Models.Labels["test"] = "value"
	if cfg.Models.Labels["test"] != "value" {
		t.Errorf("expected 'value', got %q", cfg.Models.Labels["test"])
	}
}

func TestLoadPreservesLabels(t *testing.T) {
	// Test that existing labels are preserved and defaults apply to missing fields
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `models:
  labels:
    "complexity:high": "opus"
    "complexity:low": "haiku"
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Models.Labels["complexity:high"] != "opus" {
		t.Errorf("expected 'opus', got %q", cfg.Models.Labels["complexity:high"])
	}
	if cfg.Models.Labels["complexity:low"] != "haiku" {
		t.Errorf("expected 'haiku', got %q", cfg.Models.Labels["complexity:low"])
	}

	// Defaults should still apply to missing fields
	if cfg.Models.P0 != "opus" {
		t.Errorf("expected P0='opus', got %q", cfg.Models.P0)
	}
	if cfg.Claude.Binary != "claude" {
		t.Errorf("expected Binary='claude', got %q", cfg.Claude.Binary)
	}
}

// Tests for stuck-bead threshold configuration

func TestStuckBeadThresholdDefault(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if cfg.Loop.StuckBeadThreshold != 3 {
		t.Errorf("expected default StuckBeadThreshold=3, got %d", cfg.Loop.StuckBeadThreshold)
	}
}

func TestStuckBeadThresholdFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("loop:\n  stuck_bead_threshold: 5\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Loop.StuckBeadThreshold != 5 {
		t.Errorf("expected StuckBeadThreshold=5, got %d", cfg.Loop.StuckBeadThreshold)
	}
}

func TestStuckBeadThresholdZeroGetsDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("loop:\n  stuck_bead_threshold: 0\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Loop.StuckBeadThreshold != 3 {
		t.Errorf("expected StuckBeadThreshold=3 (default), got %d", cfg.Loop.StuckBeadThreshold)
	}
}

func TestStuckBeadThresholdPreserved(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("loop:\n  stuck_bead_threshold: 10\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Loop.StuckBeadThreshold != 10 {
		t.Errorf("expected StuckBeadThreshold=10, got %d", cfg.Loop.StuckBeadThreshold)
	}
}

func TestStuckBeadThresholdInFullConfig(t *testing.T) {
	// Test that stuck_bead_threshold works alongside other loop settings
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `loop:
  max_iterations: 20
  stop_on_failure: true
  stuck_bead_threshold: 7
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Loop.MaxIterations != 20 {
		t.Errorf("expected MaxIterations=20, got %d", cfg.Loop.MaxIterations)
	}
	if !cfg.Loop.StopOnFailure {
		t.Errorf("expected StopOnFailure=true, got false")
	}
	if cfg.Loop.StuckBeadThreshold != 7 {
		t.Errorf("expected StuckBeadThreshold=7, got %d", cfg.Loop.StuckBeadThreshold)
	}
}

// Tests for max-consecutive-skips configuration

func TestMaxConsecutiveSkipsDefault(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if cfg.Loop.MaxConsecutiveSkips != 3 {
		t.Errorf("expected default MaxConsecutiveSkips=3, got %d", cfg.Loop.MaxConsecutiveSkips)
	}
}

func TestMaxConsecutiveSkipsFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("loop:\n  max_consecutive_skips: 5\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Loop.MaxConsecutiveSkips != 5 {
		t.Errorf("expected MaxConsecutiveSkips=5, got %d", cfg.Loop.MaxConsecutiveSkips)
	}
}

func TestMaxConsecutiveSkipsZeroGetsDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("loop:\n  max_consecutive_skips: 0\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Loop.MaxConsecutiveSkips != 3 {
		t.Errorf("expected MaxConsecutiveSkips=3 (default), got %d", cfg.Loop.MaxConsecutiveSkips)
	}
}

func TestMaxConsecutiveSkipsPreserved(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("loop:\n  max_consecutive_skips: 10\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Loop.MaxConsecutiveSkips != 10 {
		t.Errorf("expected MaxConsecutiveSkips=10, got %d", cfg.Loop.MaxConsecutiveSkips)
	}
}

func TestMaxConsecutiveSkipsInFullConfig(t *testing.T) {
	// Test that max_consecutive_skips works alongside other loop settings
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `loop:
  max_iterations: 20
  stop_on_failure: true
  stuck_bead_threshold: 5
  max_consecutive_skips: 7
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Loop.MaxIterations != 20 {
		t.Errorf("expected MaxIterations=20, got %d", cfg.Loop.MaxIterations)
	}
	if !cfg.Loop.StopOnFailure {
		t.Errorf("expected StopOnFailure=true, got false")
	}
	if cfg.Loop.StuckBeadThreshold != 5 {
		t.Errorf("expected StuckBeadThreshold=5, got %d", cfg.Loop.StuckBeadThreshold)
	}
	if cfg.Loop.MaxConsecutiveSkips != 7 {
		t.Errorf("expected MaxConsecutiveSkips=7, got %d", cfg.Loop.MaxConsecutiveSkips)
	}
}

func TestNormalizeNilFields(t *testing.T) {
	cfg := &Config{}
	cfg.NormalizeNilFields()

	if cfg.Escalation.Chain == nil {
		t.Error("Expected Escalation.Chain to be non-nil after normalization")
	}
	if cfg.Validation.Commands == nil {
		t.Error("Expected Validation.Commands to be non-nil after normalization")
	}
	if cfg.Preflight.Tools == nil {
		t.Error("Expected Preflight.Tools to be non-nil after normalization")
	}
	if cfg.Claude.Flags == nil {
		t.Error("Expected Claude.Flags to be non-nil after normalization")
	}
	if cfg.Models.Labels == nil {
		t.Error("Expected Models.Labels to be non-nil after normalization")
	}
}

func TestNormalizeNilFieldsPreservesExisting(t *testing.T) {
	cfg := &Config{
		Escalation: EscalationConfig{
			Chain: []string{"haiku", "sonnet"},
		},
		Validation: ValidationConfig{
			Commands: []string{"go test ./..."},
		},
		Preflight: PreflightConfig{
			Tools: []string{"go"},
		},
		Claude: ClaudeConfig{
			Flags: []string{"--dangerously-skip-permissions"},
		},
		Models: ModelsConfig{
			Labels: map[string]string{"complexity:high": "opus"},
		},
	}
	cfg.NormalizeNilFields()

	if len(cfg.Escalation.Chain) != 2 {
		t.Errorf("Expected 2 chain entries, got %d", len(cfg.Escalation.Chain))
	}
	if len(cfg.Validation.Commands) != 1 {
		t.Errorf("Expected 1 command, got %d", len(cfg.Validation.Commands))
	}
	if len(cfg.Preflight.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(cfg.Preflight.Tools))
	}
	if len(cfg.Claude.Flags) != 1 {
		t.Errorf("Expected 1 flag, got %d", len(cfg.Claude.Flags))
	}
	if cfg.Models.Labels["complexity:high"] != "opus" {
		t.Errorf("Expected 'opus', got %q", cfg.Models.Labels["complexity:high"])
	}
}

func TestLoadNormalizesNilSliceFields(t *testing.T) {
	// Loading a minimal config should produce non-nil slices for all fields
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Validation.Commands == nil {
		t.Error("Expected Validation.Commands to be non-nil after Load")
	}
	if cfg.Preflight.Tools == nil {
		t.Error("Expected Preflight.Tools to be non-nil after Load")
	}
	if cfg.Claude.Flags == nil {
		t.Error("Expected Claude.Flags to be non-nil after Load")
	}
}

// Helper function to check if string contains substring
// Tests for ReviewConfig

func TestReviewConfigDefaults(t *testing.T) {
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

	if cfg.Review.Model != "sonnet" {
		t.Errorf("expected default review model 'sonnet', got %q", cfg.Review.Model)
	}
	if !cfg.Review.ShouldMatchBuildModel() {
		t.Errorf("expected match_build_model default true")
	}
	if cfg.Review.Timeout != 120 {
		t.Errorf("expected default review timeout 120, got %d", cfg.Review.Timeout)
	}
	if cfg.Review.Thorough.Model != "opus" {
		t.Errorf("expected default thorough model 'opus', got %q", cfg.Review.Thorough.Model)
	}
	if cfg.Review.Thorough.EveryNIterations != 5 {
		t.Errorf("expected default every_n_iterations 5, got %d", cfg.Review.Thorough.EveryNIterations)
	}
	if !cfg.Review.Thorough.ShouldRunOnEpicComplete() {
		t.Errorf("expected on_epic_complete default true")
	}
	if cfg.Review.Thorough.Timeout != 900 {
		t.Errorf("expected default thorough timeout 900, got %d", cfg.Review.Thorough.Timeout)
	}
}

func TestReviewConfigExplicit(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  enabled: true
  model: opus
  match_build_model: false
  timeout: 200
  thorough:
    enabled: true
    every_n_iterations: 10
    on_epic_complete: false
    model: sonnet
    timeout: 600
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if !cfg.Review.Enabled {
		t.Errorf("expected review enabled true")
	}
	if cfg.Review.Model != "opus" {
		t.Errorf("expected review model 'opus', got %q", cfg.Review.Model)
	}
	if cfg.Review.ShouldMatchBuildModel() {
		t.Errorf("expected match_build_model false, got true")
	}
	if cfg.Review.Timeout != 200 {
		t.Errorf("expected review timeout 200, got %d", cfg.Review.Timeout)
	}
	if !cfg.Review.Thorough.Enabled {
		t.Errorf("expected thorough enabled true")
	}
	if cfg.Review.Thorough.EveryNIterations != 10 {
		t.Errorf("expected every_n_iterations 10, got %d", cfg.Review.Thorough.EveryNIterations)
	}
	if cfg.Review.Thorough.ShouldRunOnEpicComplete() {
		t.Errorf("expected on_epic_complete false, got true")
	}
	if cfg.Review.Thorough.Model != "sonnet" {
		t.Errorf("expected thorough model 'sonnet', got %q", cfg.Review.Thorough.Model)
	}
	if cfg.Review.Thorough.Timeout != 600 {
		t.Errorf("expected thorough timeout 600, got %d", cfg.Review.Thorough.Timeout)
	}
}

func TestReviewConfigTierParsingNormalizesCaseAndWhitespace(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  tier: " LOW "
  thorough:
    tier: " HiGh "
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Review.Tier != "low" {
		t.Fatalf("Review.Tier = %q, want %q", cfg.Review.Tier, "low")
	}
	if cfg.Review.Thorough.Tier != "high" {
		t.Fatalf("Review.Thorough.Tier = %q, want %q", cfg.Review.Thorough.Tier, "high")
	}
}

func TestReviewConfigLegacyModelAutoMappingTrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  model: " OpUs "
  thorough:
    model: " hAiKu "
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Review.Tier != "high" {
		t.Fatalf("Review.Tier = %q, want %q", cfg.Review.Tier, "high")
	}
	if cfg.Review.Thorough.Tier != "low" {
		t.Fatalf("Review.Thorough.Tier = %q, want %q", cfg.Review.Thorough.Tier, "low")
	}
}

func TestReviewConfigMatchBuildModelDeprecationWarningMentionsIgnored(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  match_build_model: true
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	var warning strings.Builder
	originalWriter := configWarningWriter
	configWarningWriter = &warning
	t.Cleanup(func() {
		configWarningWriter = originalWriter
	})

	if _, err := Load(cfgPath); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	warningText := strings.ToLower(warning.String())
	if !strings.Contains(warningText, "match_build_model") {
		t.Fatalf("warning = %q, want mention of match_build_model", warning.String())
	}
	if !strings.Contains(warningText, "ignored") {
		t.Fatalf("warning = %q, want mention that setting is ignored", warning.String())
	}
}

func TestReviewConfigPartialExplicit(t *testing.T) {
	// Test that explicit false values are preserved while unset values get defaults
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  enabled: false
  match_build_model: false
  thorough:
    enabled: false
    on_epic_complete: false
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Explicit false values should be preserved
	if cfg.Review.Enabled {
		t.Errorf("expected review enabled false")
	}
	if cfg.Review.ShouldMatchBuildModel() {
		t.Errorf("expected match_build_model false, got true")
	}
	if cfg.Review.Thorough.Enabled {
		t.Errorf("expected thorough enabled false")
	}
	if cfg.Review.Thorough.ShouldRunOnEpicComplete() {
		t.Errorf("expected on_epic_complete false, got true")
	}

	// Unset values should still get defaults
	if cfg.Review.Model != "sonnet" {
		t.Errorf("expected default model 'sonnet', got %q", cfg.Review.Model)
	}
	if cfg.Review.Timeout != 120 {
		t.Errorf("expected default timeout 120, got %d", cfg.Review.Timeout)
	}
}

func TestReviewConfigZeroTimeouts(t *testing.T) {
	// Test that zero timeout values are replaced with defaults
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  timeout: 0
  thorough:
    timeout: 0
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Review.Timeout != 120 {
		t.Errorf("expected default timeout 120, got %d", cfg.Review.Timeout)
	}
	if cfg.Review.Thorough.Timeout != 900 {
		t.Errorf("expected default thorough timeout 900, got %d", cfg.Review.Thorough.Timeout)
	}
}

func TestReviewConfigZeroIterations(t *testing.T) {
	// Test that zero every_n_iterations is replaced with default
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  thorough:
    every_n_iterations: 0
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Review.Thorough.EveryNIterations != 5 {
		t.Errorf("expected default every_n_iterations 5, got %d", cfg.Review.Thorough.EveryNIterations)
	}
}

func TestReviewConfigEmptyStrings(t *testing.T) {
	// Test that empty string models are replaced with defaults
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  model: ""
  thorough:
    model: ""
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Review.Model != "sonnet" {
		t.Errorf("expected default model 'sonnet', got %q", cfg.Review.Model)
	}
	if cfg.Review.Thorough.Model != "opus" {
		t.Errorf("expected default thorough model 'opus', got %q", cfg.Review.Thorough.Model)
	}
}

func TestReviewConfigCustomModels(t *testing.T) {
	// Test that custom model values are preserved
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  model: custom-review-model
  thorough:
    model: custom-thorough-model
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Review.Model != "custom-review-model" {
		t.Errorf("expected custom model 'custom-review-model', got %q", cfg.Review.Model)
	}
	if cfg.Review.Thorough.Model != "custom-thorough-model" {
		t.Errorf("expected custom thorough model 'custom-thorough-model', got %q", cfg.Review.Thorough.Model)
	}
}

func TestReviewConfigCustomTimeouts(t *testing.T) {
	// Test that custom timeout values are preserved
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  timeout: 300
  thorough:
    timeout: 1800
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Review.Timeout != 300 {
		t.Errorf("expected custom timeout 300, got %d", cfg.Review.Timeout)
	}
	if cfg.Review.Thorough.Timeout != 1800 {
		t.Errorf("expected custom thorough timeout 1800, got %d", cfg.Review.Thorough.Timeout)
	}
}

func TestReviewConfigCustomIterations(t *testing.T) {
	// Test that custom every_n_iterations value is preserved
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  thorough:
    every_n_iterations: 15
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Review.Thorough.EveryNIterations != 15 {
		t.Errorf("expected custom every_n_iterations 15, got %d", cfg.Review.Thorough.EveryNIterations)
	}
}

func TestShouldMatchBuildModelNilPointer(t *testing.T) {
	// Test that nil pointer returns true (default)
	cfg := ReviewConfig{}
	if !cfg.ShouldMatchBuildModel() {
		t.Errorf("expected ShouldMatchBuildModel() to return true for nil pointer")
	}
}

func TestShouldMatchBuildModelExplicitTrue(t *testing.T) {
	// Test that explicit true value is preserved
	trueVal := true
	cfg := ReviewConfig{MatchBuildModel: &trueVal}
	if !cfg.ShouldMatchBuildModel() {
		t.Errorf("expected ShouldMatchBuildModel() to return true for explicit true")
	}
}

func TestShouldMatchBuildModelExplicitFalse(t *testing.T) {
	// Test that explicit false value is preserved
	falseVal := false
	cfg := ReviewConfig{MatchBuildModel: &falseVal}
	if cfg.ShouldMatchBuildModel() {
		t.Errorf("expected ShouldMatchBuildModel() to return false for explicit false")
	}
}

func TestShouldRunOnEpicCompleteNilPointer(t *testing.T) {
	// Test that nil pointer returns true (default)
	cfg := ThoroughReviewConfig{}
	if !cfg.ShouldRunOnEpicComplete() {
		t.Errorf("expected ShouldRunOnEpicComplete() to return true for nil pointer")
	}
}

func TestShouldRunOnEpicCompleteExplicitTrue(t *testing.T) {
	// Test that explicit true value is preserved
	trueVal := true
	cfg := ThoroughReviewConfig{OnEpicComplete: &trueVal}
	if !cfg.ShouldRunOnEpicComplete() {
		t.Errorf("expected ShouldRunOnEpicComplete() to return true for explicit true")
	}
}

func TestShouldRunOnEpicCompleteExplicitFalse(t *testing.T) {
	// Test that explicit false value is preserved
	falseVal := false
	cfg := ThoroughReviewConfig{OnEpicComplete: &falseVal}
	if cfg.ShouldRunOnEpicComplete() {
		t.Errorf("expected ShouldRunOnEpicComplete() to return false for explicit false")
	}
}

func TestReviewConfigInFullConfig(t *testing.T) {
	// Test that review config works alongside all other config sections
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `models:
  p0: opus
  p1: sonnet
  p2: haiku
escalation:
  enabled: true
  chain: ["haiku", "sonnet", "opus"]
loop:
  max_iterations: 10
validation:
  enabled: true
  commands: ["go test ./..."]
review:
  enabled: true
  model: sonnet
  match_build_model: true
  timeout: 150
  thorough:
    enabled: true
    every_n_iterations: 7
    on_epic_complete: true
    model: opus
    timeout: 1000
claude:
  timeout: 600
paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Check other sections still work
	if cfg.Models.P0 != "opus" {
		t.Errorf("expected P0='opus', got %q", cfg.Models.P0)
	}
	if cfg.Loop.MaxIterations != 10 {
		t.Errorf("expected MaxIterations=10, got %d", cfg.Loop.MaxIterations)
	}

	// Check review section
	if !cfg.Review.Enabled {
		t.Errorf("expected review enabled true")
	}
	if cfg.Review.Model != "sonnet" {
		t.Errorf("expected review model 'sonnet', got %q", cfg.Review.Model)
	}
	if !cfg.Review.ShouldMatchBuildModel() {
		t.Errorf("expected match_build_model true")
	}
	if cfg.Review.Timeout != 150 {
		t.Errorf("expected review timeout 150, got %d", cfg.Review.Timeout)
	}
	if !cfg.Review.Thorough.Enabled {
		t.Errorf("expected thorough enabled true")
	}
	if cfg.Review.Thorough.EveryNIterations != 7 {
		t.Errorf("expected every_n_iterations 7, got %d", cfg.Review.Thorough.EveryNIterations)
	}
	if !cfg.Review.Thorough.ShouldRunOnEpicComplete() {
		t.Errorf("expected on_epic_complete true")
	}
	if cfg.Review.Thorough.Model != "opus" {
		t.Errorf("expected thorough model 'opus', got %q", cfg.Review.Thorough.Model)
	}
	if cfg.Review.Thorough.Timeout != 1000 {
		t.Errorf("expected thorough timeout 1000, got %d", cfg.Review.Thorough.Timeout)
	}
}

// Tests for BetweenIterationsCommand configuration

func loadConfigFromYAML(t *testing.T, yamlContent string) *Config {
	t.Helper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	return cfg
}

func loadConfigErrorFromYAML(t *testing.T, yamlContent string) error {
	t.Helper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	_, err := Load(cfgPath)
	return err
}

func assertLoopCommandFromYAML(t *testing.T, fieldLabel string, yamlKey string, getCommand func(*Config) string) {
	t.Helper()

	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "Simple command",
			command:  "make",
			expected: "make",
		},
		{
			name:     "Command with flags",
			command:  "go build -o gromit",
			expected: "go build -o gromit",
		},
		{
			name:     "Command with shell operators",
			command:  "make && go install",
			expected: "make && go install",
		},
		{
			name:     "Empty string",
			command:  "",
			expected: "",
		},
		{
			name:     "Command with path",
			command:  "./scripts/rebuild.sh",
			expected: "./scripts/rebuild.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := "loop:\n  " + yamlKey + ": \"" + tt.command + "\"\n"
			cfg := loadConfigFromYAML(t, yaml)

			if got := getCommand(cfg); got != tt.expected {
				t.Errorf("expected %s=%q, got %q", fieldLabel, tt.expected, got)
			}
		})
	}
}

func TestBetweenIterationsCommandDefault(t *testing.T) {
	// Test that BetweenIterationsCommand defaults to empty string when absent
	cfg := loadConfigFromYAML(t, "")

	if cfg.Loop.BetweenIterationsCommand != "" {
		t.Errorf("expected default BetweenIterationsCommand='', got %q", cfg.Loop.BetweenIterationsCommand)
	}
}

func TestBetweenIterationsCommandFromYAML(t *testing.T) {
	// Test that BetweenIterationsCommand loads correctly from YAML
	assertLoopCommandFromYAML(
		t,
		"BetweenIterationsCommand",
		"between_iterations_command",
		func(cfg *Config) string { return cfg.Loop.BetweenIterationsCommand },
	)
}

func TestBetweenIterationsCommandWithOtherLoopSettings(t *testing.T) {
	// Test that between_iterations_command works alongside other loop settings
	yaml := `loop:
  max_iterations: 20
  stop_on_failure: true
  stuck_bead_threshold: 5
  between_iterations_command: "make"
`
	cfg := loadConfigFromYAML(t, yaml)

	// Check all loop settings
	if cfg.Loop.MaxIterations != 20 {
		t.Errorf("expected MaxIterations=20, got %d", cfg.Loop.MaxIterations)
	}
	if !cfg.Loop.StopOnFailure {
		t.Errorf("expected StopOnFailure=true, got false")
	}
	if cfg.Loop.StuckBeadThreshold != 5 {
		t.Errorf("expected StuckBeadThreshold=5, got %d", cfg.Loop.StuckBeadThreshold)
	}
	if cfg.Loop.BetweenIterationsCommand != "make" {
		t.Errorf("expected BetweenIterationsCommand='make', got %q", cfg.Loop.BetweenIterationsCommand)
	}
}

func TestBetweenIterationsCommandPreservedAcrossFullConfig(t *testing.T) {
	// Test that between_iterations_command is preserved when loading a full config
	yaml := `models:
  p0: opus
  p1: sonnet
  p2: haiku
escalation:
  enabled: true
  chain: ["haiku", "sonnet", "opus"]
loop:
  max_iterations: 10
  between_iterations_command: "go build ./cmd/gromit"
  end_of_loop_command: "make clean"
validation:
  enabled: true
  commands: ["go test ./..."]
claude:
  timeout: 600
`
	cfg := loadConfigFromYAML(t, yaml)

	// Check other sections still work
	if cfg.Models.P0 != "opus" {
		t.Errorf("expected P0='opus', got %q", cfg.Models.P0)
	}
	if cfg.Loop.MaxIterations != 10 {
		t.Errorf("expected MaxIterations=10, got %d", cfg.Loop.MaxIterations)
	}

	// Check between_iterations_command
	if cfg.Loop.BetweenIterationsCommand != "go build ./cmd/gromit" {
		t.Errorf("expected BetweenIterationsCommand='go build ./cmd/gromit', got %q", cfg.Loop.BetweenIterationsCommand)
	}
	if cfg.Loop.EndOfLoopCommand != "make clean" {
		t.Errorf("expected EndOfLoopCommand='make clean', got %q", cfg.Loop.EndOfLoopCommand)
	}
}

// Tests for EndOfLoopCommand configuration

func TestEndOfLoopCommandDefault(t *testing.T) {
	// Test that EndOfLoopCommand defaults to empty string when absent
	cfg := loadConfigFromYAML(t, "")

	if cfg.Loop.EndOfLoopCommand != "" {
		t.Errorf("expected default EndOfLoopCommand='', got %q", cfg.Loop.EndOfLoopCommand)
	}
}

func TestEndOfLoopCommandFromYAML(t *testing.T) {
	// Test that EndOfLoopCommand loads correctly from YAML
	assertLoopCommandFromYAML(
		t,
		"EndOfLoopCommand",
		"end_of_loop_command",
		func(cfg *Config) string { return cfg.Loop.EndOfLoopCommand },
	)
}

func TestEndOfLoopCommandWithOtherLoopSettings(t *testing.T) {
	// Test that end_of_loop_command works alongside other loop settings
	yaml := `loop:
  max_iterations: 20
  stop_on_failure: true
  stuck_bead_threshold: 5
  end_of_loop_command: "make"
`
	cfg := loadConfigFromYAML(t, yaml)

	// Check all loop settings
	if cfg.Loop.MaxIterations != 20 {
		t.Errorf("expected MaxIterations=20, got %d", cfg.Loop.MaxIterations)
	}
	if !cfg.Loop.StopOnFailure {
		t.Errorf("expected StopOnFailure=true, got false")
	}
	if cfg.Loop.StuckBeadThreshold != 5 {
		t.Errorf("expected StuckBeadThreshold=5, got %d", cfg.Loop.StuckBeadThreshold)
	}
	if cfg.Loop.EndOfLoopCommand != "make" {
		t.Errorf("expected EndOfLoopCommand='make', got %q", cfg.Loop.EndOfLoopCommand)
	}
}

func TestEndOfLoopCommandPreservedAcrossFullConfig(t *testing.T) {
	// Test that end_of_loop_command is preserved when loading a full config
	yaml := `models:
  p0: opus
  p1: sonnet
  p2: haiku
escalation:
  enabled: true
  chain: ["haiku", "sonnet", "opus"]
loop:
  max_iterations: 10
  between_iterations_command: "go build ./cmd/gromit"
  end_of_loop_command: "make clean"
validation:
  enabled: true
  commands: ["go test ./..."]
claude:
  timeout: 600
`
	cfg := loadConfigFromYAML(t, yaml)

	// Check other sections still work
	if cfg.Models.P0 != "opus" {
		t.Errorf("expected P0='opus', got %q", cfg.Models.P0)
	}
	if cfg.Loop.MaxIterations != 10 {
		t.Errorf("expected MaxIterations=10, got %d", cfg.Loop.MaxIterations)
	}

	// Check end_of_loop_command
	if cfg.Loop.EndOfLoopCommand != "make clean" {
		t.Errorf("expected EndOfLoopCommand='make clean', got %q", cfg.Loop.EndOfLoopCommand)
	}
}

// Tests for PrecheckConfig
func TestPrecheckConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Precheck.IsEnabled() {
		t.Errorf("expected default precheck enabled=false")
	}
	if cfg.Precheck.Model != "haiku" {
		t.Errorf("expected default precheck model='haiku', got %q", cfg.Precheck.Model)
	}
	if cfg.Precheck.TimeoutSeconds != 120 {
		t.Errorf("expected default precheck timeout=120, got %d", cfg.Precheck.TimeoutSeconds)
	}
}

func TestPrecheckConfigFromYAML(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		expectEnabled bool
		expectModel   string
		expectTimeout int
	}{
		{
			name: "All fields explicit",
			yaml: `precheck:
  enabled: true
  model: sonnet
  timeout_seconds: 60
`,
			expectEnabled: true,
			expectModel:   "sonnet",
			expectTimeout: 60,
		},
		{
			name: "Disabled explicitly",
			yaml: `precheck:
  enabled: false
`,
			expectEnabled: false,
			expectModel:   "haiku",
			expectTimeout: 120,
		},
		{
			name: "Custom model only",
			yaml: `precheck:
  model: opus
`,
			expectEnabled: false,
			expectModel:   "opus",
			expectTimeout: 120,
		},
		{
			name: "Custom timeout only",
			yaml: `precheck:
  timeout_seconds: 90
`,
			expectEnabled: false,
			expectModel:   "haiku",
			expectTimeout: 90,
		},
		{
			name:          "Empty config uses defaults",
			yaml:          "",
			expectEnabled: false,
			expectModel:   "haiku",
			expectTimeout: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "gromit.yaml")
			if err := os.WriteFile(cfgPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("writing config: %v", err)
			}

			cfg, err := Load(cfgPath)
			if err != nil {
				t.Fatalf("loading config: %v", err)
			}

			if cfg.Precheck.IsEnabled() != tt.expectEnabled {
				t.Errorf("expected enabled=%v, got %v", tt.expectEnabled, cfg.Precheck.IsEnabled())
			}
			if cfg.Precheck.Model != tt.expectModel {
				t.Errorf("expected model=%q, got %q", tt.expectModel, cfg.Precheck.Model)
			}
			if cfg.Precheck.TimeoutSeconds != tt.expectTimeout {
				t.Errorf("expected timeout=%d, got %d", tt.expectTimeout, cfg.Precheck.TimeoutSeconds)
			}
		})
	}
}

func TestPrecheckIsEnabledNilPointer(t *testing.T) {
	cfg := PrecheckConfig{}
	if cfg.IsEnabled() {
		t.Errorf("expected IsEnabled() to return false for nil pointer (default-off)")
	}
}

func TestPrecheckIsEnabledExplicitTrue(t *testing.T) {
	trueVal := true
	cfg := PrecheckConfig{Enabled: &trueVal}
	if !cfg.IsEnabled() {
		t.Errorf("expected IsEnabled() to return true for explicit true")
	}
}

func TestPrecheckIsEnabledExplicitFalse(t *testing.T) {
	falseVal := false
	cfg := PrecheckConfig{Enabled: &falseVal}
	if cfg.IsEnabled() {
		t.Errorf("expected IsEnabled() to return false for explicit false")
	}
}

func TestPrecheckConfigInFullConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `models:
  p0: opus
  p1: sonnet
  p2: haiku
precheck:
  enabled: false
  model: sonnet
  timeout_seconds: 45
validation:
  enabled: true
  commands: ["go test ./..."]
claude:
  timeout: 600
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Check other sections still work
	if cfg.Models.P0 != "opus" {
		t.Errorf("expected P0='opus', got %q", cfg.Models.P0)
	}

	// Check precheck section
	if cfg.Precheck.IsEnabled() {
		t.Errorf("expected precheck enabled=false")
	}
	if cfg.Precheck.Model != "sonnet" {
		t.Errorf("expected precheck model='sonnet', got %q", cfg.Precheck.Model)
	}
	if cfg.Precheck.TimeoutSeconds != 45 {
		t.Errorf("expected precheck timeout=45, got %d", cfg.Precheck.TimeoutSeconds)
	}
}

func TestPrecheckConfigZeroTimeout(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `precheck:
  timeout_seconds: 0
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Zero timeout should be replaced with default
	if cfg.Precheck.TimeoutSeconds != 120 {
		t.Errorf("expected default timeout=120, got %d", cfg.Precheck.TimeoutSeconds)
	}
}

func TestPrecheckConfigEmptyModel(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `precheck:
  model: ""
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Empty model should be replaced with default
	if cfg.Precheck.Model != "haiku" {
		t.Errorf("expected default model='haiku', got %q", cfg.Precheck.Model)
	}
}

// Tests for MethodologyConfig parsing
func TestMethodologyConfigParsing(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		expectATDD bool
		expectTDD  bool
	}{
		{
			name: "ATDD present true",
			yaml: `methodology:
  atdd: true
`,
			expectATDD: true,
			expectTDD:  false,
		},
		{
			name: "ATDD present false",
			yaml: `methodology:
  atdd: false
`,
			expectATDD: false,
			expectTDD:  false,
		},
		{
			name: "ATDD absent defaults false",
			yaml: `methodology:
  tdd: false
`,
			expectATDD: false,
			expectTDD:  false,
		},
		{
			name: "TDD present true",
			yaml: `methodology:
  tdd: true
`,
			expectATDD: false,
			expectTDD:  true,
		},
		{
			name: "TDD present false",
			yaml: `methodology:
  tdd: false
`,
			expectATDD: false,
			expectTDD:  false,
		},
		{
			name: "TDD absent defaults false",
			yaml: `methodology:
  atdd: false
`,
			expectATDD: false,
			expectTDD:  false,
		},
		{
			name: "Methodology section absent",
			yaml: `models:
  p0: opus
`,
			expectATDD: false,
			expectTDD:  false,
		},
		{
			name: "Both ATDD and TDD true",
			yaml: `methodology:
  atdd: true
  tdd: true
`,
			expectATDD: true,
			expectTDD:  true,
		},
		{
			name: "Both ATDD and TDD false",
			yaml: `methodology:
  atdd: false
  tdd: false
`,
			expectATDD: false,
			expectTDD:  false,
		},
		{
			name: "Empty methodology section",
			yaml: `methodology:
`,
			expectATDD: false,
			expectTDD:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "gromit.yaml")
			if err := os.WriteFile(cfgPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("writing config: %v", err)
			}

			cfg, err := Load(cfgPath)
			if err != nil {
				t.Fatalf("loading config: %v", err)
			}

			if cfg.Methodology.ATDD != tt.expectATDD {
				t.Errorf("expected ATDD=%v, got %v", tt.expectATDD, cfg.Methodology.ATDD)
			}
			if cfg.Methodology.TDD != tt.expectTDD {
				t.Errorf("expected TDD=%v, got %v", tt.expectTDD, cfg.Methodology.TDD)
			}
		})
	}
}

func TestMethodologyGranularityDefaultsToBead(t *testing.T) {
	yaml := `methodology:
  tdd: true
`
	cfg := loadConfigFromYAML(t, yaml)

	if cfg.Methodology.Granularity != MethodologyGranularityBead {
		t.Errorf("expected granularity=%s, got %q", MethodologyGranularityBead, cfg.Methodology.Granularity)
	}
}

func TestMethodologyMaxTDDCyclesDefaultsToTen(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Methodology.MaxTDDCycles != DefaultMaxTDDCycles {
		t.Errorf("expected max_tdd_cycles=%d, got %d", DefaultMaxTDDCycles, cfg.Methodology.MaxTDDCycles)
	}
}

func TestMethodologyMaxTDDCyclesParsesFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `methodology:
  max_tdd_cycles: 7
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Methodology.MaxTDDCycles != 7 {
		t.Errorf("expected max_tdd_cycles=7, got %d", cfg.Methodology.MaxTDDCycles)
	}
}

func TestMethodologySpecGateMaxRetriesDefaultsToThree(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Methodology.SpecGateMaxRetries != DefaultSpecGateMaxRetries {
		t.Errorf("expected spec_gate_max_retries=%d, got %d", DefaultSpecGateMaxRetries, cfg.Methodology.SpecGateMaxRetries)
	}
}

func TestMethodologySpecGateMaxRetriesParsesFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `methodology:
  spec_gate_max_retries: 6
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Methodology.SpecGateMaxRetries != 6 {
		t.Errorf("expected spec_gate_max_retries=6, got %d", cfg.Methodology.SpecGateMaxRetries)
	}
}

func TestMethodologyFreshContextPerCycleParsesFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `methodology:
  fresh_context_per_cycle: true
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if !cfg.Methodology.FreshContextPerCycle {
		t.Error("expected fresh_context_per_cycle=true, got false")
	}
}

func TestMethodologyFreshContextPerCycleDefaultsFalseWhenUnset(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `methodology:
  tdd: true
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Methodology.FreshContextPerCycle {
		t.Error("expected fresh_context_per_cycle=false when unset")
	}
}

func TestMethodologyATDDPromptDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if !cfg.Methodology.ATDDPrompt.IncludeRules {
		t.Error("expected default ATDDPrompt.IncludeRules=true")
	}
	if !cfg.Methodology.ATDDPrompt.IncludeSpec {
		t.Error("expected default ATDDPrompt.IncludeSpec=true")
	}
	if !cfg.Methodology.ATDDPrompt.IncludeClaudeMD {
		t.Error("expected default ATDDPrompt.IncludeClaudeMD=true")
	}
	if cfg.Methodology.ATDDPrompt.MaxChars != 20000 {
		t.Errorf("expected default ATDDPrompt.MaxChars=%d, got %d", defaultATDDPromptMaxChars, cfg.Methodology.ATDDPrompt.MaxChars)
	}
	if cfg.Methodology.ATDDPrompt.MaxConfirmedLearningChars != 2000 {
		t.Errorf("expected default ATDDPrompt.MaxConfirmedLearningChars=%d, got %d", defaultATDDPromptLearningCharsCap, cfg.Methodology.ATDDPrompt.MaxConfirmedLearningChars)
	}
}

func TestMethodologyATDDPromptConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `methodology:
  atdd_prompt:
    include_rules: false
    include_spec: false
    include_claude_md: false
    max_chars: 12345
    max_confirmed_learning_chars: 456
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Methodology.ATDDPrompt.IncludeRules != false {
		t.Errorf("expected include_rules=false, got %t", cfg.Methodology.ATDDPrompt.IncludeRules)
	}
	if cfg.Methodology.ATDDPrompt.IncludeSpec != false {
		t.Errorf("expected include_spec=false, got %t", cfg.Methodology.ATDDPrompt.IncludeSpec)
	}
	if cfg.Methodology.ATDDPrompt.IncludeClaudeMD != false {
		t.Errorf("expected include_claude_md=false, got %t", cfg.Methodology.ATDDPrompt.IncludeClaudeMD)
	}
	if cfg.Methodology.ATDDPrompt.MaxChars != 12345 {
		t.Errorf("expected max_chars=12345, got %d", cfg.Methodology.ATDDPrompt.MaxChars)
	}
	if cfg.Methodology.ATDDPrompt.MaxConfirmedLearningChars != 456 {
		t.Errorf("expected max_confirmed_learning_chars=456, got %d", cfg.Methodology.ATDDPrompt.MaxConfirmedLearningChars)
	}
}

func TestMethodologyGranularityParsing(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		granularity string
	}{
		{
			name: "Granularity bead",
			yaml: `methodology:
  granularity: "bead"
`,
			granularity: MethodologyGranularityBead,
		},
		{
			name: "Granularity spec",
			yaml: `methodology:
  granularity: "spec"
`,
			granularity: MethodologyGranularitySpec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := loadConfigFromYAML(t, tt.yaml)

			if cfg.Methodology.Granularity != tt.granularity {
				t.Errorf("expected granularity=%q, got %q", tt.granularity, cfg.Methodology.Granularity)
			}
		})
	}
}

func TestMethodologyGranularityRejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "unknown granularity",
			yaml: `methodology:
  granularity: "task"
`,
		},
		{
			name: "uppercase granularity",
			yaml: `methodology:
  granularity: "SPEC"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loadConfigErrorFromYAML(t, tt.yaml)
			if err == nil {
				t.Fatal("expected error for invalid granularity")
			}
			if !strings.Contains(err.Error(), "methodology.granularity") {
				t.Errorf("expected granularity error, got %v", err)
			}
		})
	}
}

func TestMethodologyPhaseTimeoutsParseFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `methodology:
  phase_timeouts:
    red_seconds: 111
    green_seconds: 222
    refactor_seconds: 333
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Methodology.PhaseTimeouts.RedSeconds != 111 {
		t.Errorf("expected red_seconds=111, got %d", cfg.Methodology.PhaseTimeouts.RedSeconds)
	}
	if cfg.Methodology.PhaseTimeouts.GreenSeconds != 222 {
		t.Errorf("expected green_seconds=222, got %d", cfg.Methodology.PhaseTimeouts.GreenSeconds)
	}
	if cfg.Methodology.PhaseTimeouts.RefactorSeconds != 333 {
		t.Errorf("expected refactor_seconds=333, got %d", cfg.Methodology.PhaseTimeouts.RefactorSeconds)
	}
}

func TestMethodologyConfigResolvePhaseTimeoutSeconds(t *testing.T) {
	m := MethodologyConfig{
		PhaseTimeouts: MethodologyPhaseTimeout{
			RedSeconds:      45,
			GreenSeconds:    90,
			RefactorSeconds: 0,
		},
	}

	if got := m.ResolvePhaseTimeoutSeconds("red", 300); got != 45 {
		t.Errorf("ResolvePhaseTimeoutSeconds(red) = %d, want 45", got)
	}
	if got := m.ResolvePhaseTimeoutSeconds("green", 300); got != 90 {
		t.Errorf("ResolvePhaseTimeoutSeconds(green) = %d, want 90", got)
	}
	if got := m.ResolvePhaseTimeoutSeconds("refactor", 300); got != 300 {
		t.Errorf("ResolvePhaseTimeoutSeconds(refactor) = %d, want fallback 300", got)
	}
}

func TestValidationPhaseTimeoutSecondsParseAndResolve(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `validation:
  phase_timeout_seconds: 77
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Validation.PhaseTimeoutSeconds != 77 {
		t.Fatalf("expected phase_timeout_seconds=77, got %d", cfg.Validation.PhaseTimeoutSeconds)
	}
	if got := cfg.Validation.ResolvePhaseTimeoutSeconds(300); got != 77 {
		t.Errorf("ResolvePhaseTimeoutSeconds() = %d, want 77", got)
	}

	cfg.Validation.PhaseTimeoutSeconds = 0
	if got := cfg.Validation.ResolvePhaseTimeoutSeconds(300); got != 300 {
		t.Errorf("ResolvePhaseTimeoutSeconds() with zero override = %d, want fallback 300", got)
	}
}

func TestPhaseTimeoutResolversBackwardCompatibleWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `claude:
  bead_timeout: 456
methodology:
  atdd: true
validation:
  enabled: true
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Methodology.PhaseTimeouts.RedSeconds != 0 {
		t.Fatalf("expected red_seconds default to remain zero, got %d", cfg.Methodology.PhaseTimeouts.RedSeconds)
	}
	if cfg.Validation.PhaseTimeoutSeconds != 0 {
		t.Fatalf("expected validation.phase_timeout_seconds default to remain zero, got %d", cfg.Validation.PhaseTimeoutSeconds)
	}

	if got := cfg.Methodology.ResolvePhaseTimeoutSeconds("red", cfg.Claude.BeadTimeout); got != 456 {
		t.Errorf("methodology red fallback timeout = %d, want 456", got)
	}
	if got := cfg.Methodology.ResolvePhaseTimeoutSeconds("green", cfg.Claude.BeadTimeout); got != 456 {
		t.Errorf("methodology green fallback timeout = %d, want 456", got)
	}
	if got := cfg.Methodology.ResolvePhaseTimeoutSeconds("refactor", cfg.Claude.BeadTimeout); got != 456 {
		t.Errorf("methodology refactor fallback timeout = %d, want 456", got)
	}
	if got := cfg.Validation.ResolvePhaseTimeoutSeconds(cfg.Claude.BeadTimeout); got != 456 {
		t.Errorf("validation fallback timeout = %d, want 456", got)
	}
}

func TestMethodologyConfigResolvePhaseTimeoutSeconds_UnknownPhaseReturnsFallback(t *testing.T) {
	m := MethodologyConfig{
		PhaseTimeouts: MethodologyPhaseTimeout{
			RedSeconds:      45,
			GreenSeconds:    90,
			RefactorSeconds: 120,
		},
	}

	// An unknown phase name should fall back to beadTimeoutSeconds,
	// not to any configured phase timeout.
	if got := m.ResolvePhaseTimeoutSeconds("unknown_phase", 300); got != 300 {
		t.Errorf("ResolvePhaseTimeoutSeconds(unknown_phase) = %d, want fallback 300", got)
	}
	if got := m.ResolvePhaseTimeoutSeconds("", 300); got != 300 {
		t.Errorf("ResolvePhaseTimeoutSeconds('') = %d, want fallback 300", got)
	}
}

func TestMethodologyConfigResolvePhaseTimeoutSeconds_AllPhasesConfigured(t *testing.T) {
	m := MethodologyConfig{
		PhaseTimeouts: MethodologyPhaseTimeout{
			RedSeconds:      30,
			GreenSeconds:    60,
			RefactorSeconds: 90,
		},
	}

	tests := []struct {
		phase string
		want  int
	}{
		{"red", 30},
		{"green", 60},
		{"refactor", 90},
	}
	for _, tt := range tests {
		if got := m.ResolvePhaseTimeoutSeconds(tt.phase, 300); got != tt.want {
			t.Errorf("ResolvePhaseTimeoutSeconds(%q) = %d, want %d", tt.phase, got, tt.want)
		}
	}
}

// Tests for GitConfig
func TestGitConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if !cfg.Git.IsAutoPushEnabled() {
		t.Errorf("expected default git auto_push=true")
	}
	if cfg.Git.PushFailure != "warn" {
		t.Errorf("expected default git push_failure='warn', got %q", cfg.Git.PushFailure)
	}
}

func TestGitConfigDefaultPushTimeout(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Git.PushTimeout != 60 {
		t.Errorf("expected default git push_timeout=60, got %d", cfg.Git.PushTimeout)
	}
}

func TestGitConfigPushTimeoutDurationFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `git:
  push_timeout: 90
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Git.PushTimeout != 90 {
		t.Errorf("expected git push_timeout=90, got %d", cfg.Git.PushTimeout)
	}
	if cfg.Git.PushTimeoutDuration() != 90*time.Second {
		t.Errorf("expected git PushTimeoutDuration=90s, got %s", cfg.Git.PushTimeoutDuration())
	}
}

func TestGitConfigPushTimeoutZeroFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `git:
  push_timeout: 0
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Git.PushTimeout != 0 {
		t.Errorf("expected git push_timeout=0, got %d", cfg.Git.PushTimeout)
	}
	if cfg.Git.PushTimeoutDuration() != 0 {
		t.Errorf("expected git PushTimeoutDuration=0, got %s", cfg.Git.PushTimeoutDuration())
	}
}

func TestGitConfigFromYAML(t *testing.T) {
	tests := []struct {
		name              string
		yaml              string
		expectAutoPush    bool
		expectPushFailure string
	}{
		{
			name: "All fields explicit",
			yaml: `git:
  auto_push: true
  push_failure: warn
`,
			expectAutoPush:    true,
			expectPushFailure: "warn",
		},
		{
			name: "Disabled explicitly",
			yaml: `git:
  auto_push: false
  push_failure: stop
`,
			expectAutoPush:    false,
			expectPushFailure: "stop",
		},
		{
			name:              "Omitted section gets defaults",
			yaml:              `models: {p0: opus}`,
			expectAutoPush:    true,
			expectPushFailure: "warn",
		},
		{
			name: "Partial config gets defaults for missing fields",
			yaml: `git:
  push_failure: stop
`,
			expectAutoPush:    true,
			expectPushFailure: "stop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "gromit.yaml")
			if err := os.WriteFile(cfgPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("writing config: %v", err)
			}

			cfg, err := Load(cfgPath)
			if err != nil {
				t.Fatalf("loading config: %v", err)
			}

			if cfg.Git.IsAutoPushEnabled() != tt.expectAutoPush {
				t.Errorf("expected auto_push=%v, got %v", tt.expectAutoPush, cfg.Git.IsAutoPushEnabled())
			}
			if cfg.Git.PushFailure != tt.expectPushFailure {
				t.Errorf("expected push_failure=%q, got %q", tt.expectPushFailure, cfg.Git.PushFailure)
			}
		})
	}
}

func TestGitIsAutoPushEnabledNilPointer(t *testing.T) {
	cfg := GitConfig{}
	if !cfg.IsAutoPushEnabled() {
		t.Errorf("expected IsAutoPushEnabled() to return true for nil pointer")
	}
}

func TestGitIsAutoPushEnabledExplicitTrue(t *testing.T) {
	trueVal := true
	cfg := GitConfig{AutoPush: &trueVal}
	if !cfg.IsAutoPushEnabled() {
		t.Errorf("expected IsAutoPushEnabled() to return true for explicit true")
	}
}

func TestGitIsAutoPushEnabledExplicitFalse(t *testing.T) {
	falseVal := false
	cfg := GitConfig{AutoPush: &falseVal}
	if cfg.IsAutoPushEnabled() {
		t.Errorf("expected IsAutoPushEnabled() to return false for explicit false")
	}
}

func TestGitConfigInFullConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `models:
  p0: opus
  p1: sonnet
  p2: haiku
git:
  auto_push: false
  push_failure: stop
validation:
  enabled: true
  commands: ["go test ./..."]
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Check git section
	if cfg.Git.IsAutoPushEnabled() {
		t.Errorf("expected git auto_push=false")
	}
	if cfg.Git.PushFailure != "stop" {
		t.Errorf("expected git push_failure='stop', got %q", cfg.Git.PushFailure)
	}
}

func TestStateStaleThresholdDefault(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if cfg.State.StaleThreshold != 60 {
		t.Errorf("expected default StaleThreshold=60, got %d", cfg.State.StaleThreshold)
	}
}

func TestStateStaleThresholdFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `state:
  stale_threshold: 120
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.State.StaleThreshold != 120 {
		t.Errorf("expected StaleThreshold=120, got %d", cfg.State.StaleThreshold)
	}
}

func TestStateStaleThresholdMissingInYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `loop:
  max_iterations: 10
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.State.StaleThreshold != 60 {
		t.Errorf("expected default StaleThreshold=60 when omitted, got %d", cfg.State.StaleThreshold)
	}
}

func TestShouldBlockOversizedNilPointer(t *testing.T) {
	cfg := ScopeCheckConfig{}
	if !cfg.ShouldBlockOversized() {
		t.Errorf("expected ShouldBlockOversized() to return true for nil pointer")
	}
}

func TestShouldBlockOversizedExplicitTrue(t *testing.T) {
	trueVal := true
	cfg := ScopeCheckConfig{BlockOversized: &trueVal}
	if !cfg.ShouldBlockOversized() {
		t.Errorf("expected ShouldBlockOversized() to return true for explicit true")
	}
}

func TestShouldBlockOversizedExplicitFalse(t *testing.T) {
	falseVal := false
	cfg := ScopeCheckConfig{BlockOversized: &falseVal}
	if cfg.ShouldBlockOversized() {
		t.Errorf("expected ShouldBlockOversized() to return false for explicit false")
	}
}

// --- Per-model timeout tests ---

func TestTimeoutsForModel_Defaults(t *testing.T) {
	cfg := ClaudeConfig{
		StallTimeout:       120,
		StallTimeoutActive: 300,
		BeadTimeout:        1200,
		ModelTimeouts:      map[string]ModelTimeoutOverrides{},
	}

	invocation, stall, stallActive, bead := cfg.TimeoutsForModel("sonnet")
	if invocation != 0 {
		t.Errorf("expected invocation=0 (no base Timeout set), got %d", invocation)
	}
	if stall != 120 {
		t.Errorf("expected stall=120, got %d", stall)
	}
	if stallActive != 300 {
		t.Errorf("expected stallActive=300, got %d", stallActive)
	}
	if bead != 1200 {
		t.Errorf("expected bead=1200, got %d", bead)
	}
}

func TestTimeoutsForModel_Override(t *testing.T) {
	cfg := ClaudeConfig{
		StallTimeout:       120,
		StallTimeoutActive: 300,
		BeadTimeout:        1200,
		ModelTimeouts: map[string]ModelTimeoutOverrides{
			"sonnet": {
				StallTimeout:       60,
				StallTimeoutActive: 150,
				BeadTimeout:        900,
			},
		},
	}

	_, stall, stallActive, bead := cfg.TimeoutsForModel("sonnet")
	if stall != 60 {
		t.Errorf("expected stall=60, got %d", stall)
	}
	if stallActive != 150 {
		t.Errorf("expected stallActive=150, got %d", stallActive)
	}
	if bead != 900 {
		t.Errorf("expected bead=900, got %d", bead)
	}

	// Opus should still get defaults
	_, stall, stallActive, bead = cfg.TimeoutsForModel("opus")
	if stall != 120 {
		t.Errorf("expected opus stall=120, got %d", stall)
	}
	if stallActive != 300 {
		t.Errorf("expected opus stallActive=300, got %d", stallActive)
	}
	if bead != 1200 {
		t.Errorf("expected opus bead=1200, got %d", bead)
	}
}

func TestTimeoutsForModel_PartialOverride(t *testing.T) {
	cfg := ClaudeConfig{
		StallTimeout:       120,
		StallTimeoutActive: 300,
		BeadTimeout:        1200,
		ModelTimeouts: map[string]ModelTimeoutOverrides{
			"sonnet": {
				StallTimeout: 60,
				// StallTimeoutActive and BeadTimeout not set → fall back to defaults
			},
		},
	}

	_, stall, stallActive, bead := cfg.TimeoutsForModel("sonnet")
	if stall != 60 {
		t.Errorf("expected stall=60, got %d", stall)
	}
	if stallActive != 300 {
		t.Errorf("expected stallActive=300 (default), got %d", stallActive)
	}
	if bead != 1200 {
		t.Errorf("expected bead=1200 (default), got %d", bead)
	}
}

func TestTimeoutsForModel_NilMap(t *testing.T) {
	cfg := ClaudeConfig{
		StallTimeout:       120,
		StallTimeoutActive: 300,
		BeadTimeout:        1200,
	}

	_, stall, stallActive, bead := cfg.TimeoutsForModel("sonnet")
	if stall != 120 || stallActive != 300 || bead != 1200 {
		t.Errorf("expected defaults with nil map, got stall=%d, stallActive=%d, bead=%d", stall, stallActive, bead)
	}
}

func TestTimeoutsForModel_LoadFromYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlContent := `
claude:
  stall_timeout: 120
  stall_timeout_active: 300
  bead_timeout: 1200
  model_timeouts:
    sonnet:
      stall_timeout: 60
      bead_timeout: 900
`
	cfgPath := tmpDir + "/gromit.yaml"
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	_, stall, stallActive, bead := cfg.Claude.TimeoutsForModel("sonnet")
	if stall != 60 {
		t.Errorf("expected stall=60 from YAML, got %d", stall)
	}
	if stallActive != 300 {
		t.Errorf("expected stallActive=300 (default), got %d", stallActive)
	}
	if bead != 900 {
		t.Errorf("expected bead=900 from YAML, got %d", bead)
	}
}

// TestModelTimeoutOverrides_InvocationTimeout verifies that per-model invocation
// timeout overrides load from YAML and are returned by TimeoutsForModel.
func TestModelTimeoutOverrides_InvocationTimeout(t *testing.T) {
	yamlContent := `
claude:
  timeout: 900
  stall_timeout: 180
  stall_timeout_active: 600
  bead_timeout: 1800
  model_timeouts:
    sonnet:
      timeout: 1200
      stall_timeout_active: 900
      bead_timeout: 2400
`
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	sonnetOverrides := cfg.Claude.ModelTimeouts["sonnet"]
	if sonnetOverrides.Timeout != 1200 {
		t.Errorf("sonnet invocation timeout: got %d, want 1200", sonnetOverrides.Timeout)
	}
}

// TestTimeoutsForModel_ReturnsInvocationTimeout verifies that TimeoutsForModel
// returns the per-model invocation timeout as the first return value.
func TestTimeoutsForModel_ReturnsInvocationTimeout(t *testing.T) {
	cfg := ClaudeConfig{
		Timeout:            900,
		StallTimeout:       180,
		StallTimeoutActive: 600,
		BeadTimeout:        1800,
		ModelTimeouts: map[string]ModelTimeoutOverrides{
			"sonnet": {
				Timeout: 1200,
			},
		},
	}

	invocationTimeout, _, _, _ := cfg.TimeoutsForModel("sonnet")
	if invocationTimeout != 1200 {
		t.Errorf("sonnet invocation timeout: got %d, want 1200", invocationTimeout)
	}

	// Model without override should get the base timeout
	invocationTimeout, _, _, _ = cfg.TimeoutsForModel("opus")
	if invocationTimeout != 900 {
		t.Errorf("opus invocation timeout: got %d, want 900 (base default)", invocationTimeout)
	}
}

func TestTokenBudgetForModel_DefaultAndOverride(t *testing.T) {
	cfg := ClaudeConfig{
		MaxInputTokensPerBead: 400000,
		ModelTimeouts: map[string]ModelTimeoutOverrides{
			"sonnet": {
				MaxInputTokensPerBead: 200000,
			},
		},
	}

	if got := cfg.TokenBudgetForModel("sonnet"); got != 200000 {
		t.Errorf("TokenBudgetForModel(sonnet) = %d, want 200000", got)
	}
	if got := cfg.TokenBudgetForModel("opus"); got != 400000 {
		t.Errorf("TokenBudgetForModel(opus) = %d, want 400000", got)
	}
}

func TestTokenBudgetForModel_LoadFromYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlContent := `
claude:
  max_input_tokens_per_bead: 400000
  model_timeouts:
    sonnet:
      max_input_tokens_per_bead: 150000
`
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Claude.MaxInputTokensPerBead != 400000 {
		t.Errorf("expected top-level MaxInputTokensPerBead=400000, got %d", cfg.Claude.MaxInputTokensPerBead)
	}
	if got := cfg.Claude.ModelTimeouts["sonnet"].MaxInputTokensPerBead; got != 150000 {
		t.Errorf("expected sonnet override MaxInputTokensPerBead=150000, got %d", got)
	}
	if got := cfg.Claude.TokenBudgetForModel("sonnet"); got != 150000 {
		t.Errorf("TokenBudgetForModel(sonnet) = %d, want 150000", got)
	}
	if got := cfg.Claude.TokenBudgetForModel("opus"); got != 400000 {
		t.Errorf("TokenBudgetForModel(opus) = %d, want 400000", got)
	}
}

// TestProjectGromitYAML_HasModelTimeouts loads the actual project gromit.yaml
// and verifies that sonnet and haiku have per-model timeout overrides configured.
func TestProjectGromitYAML_HasModelTimeouts(t *testing.T) {
	projectRoot := findProjectRoot(t)
	cfgPath := filepath.Join(projectRoot, "gromit.yaml")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%s) error = %v", cfgPath, err)
	}

	t.Run("sonnet_has_overrides", func(t *testing.T) {
		sonnet, ok := cfg.Claude.ModelTimeouts["sonnet"]
		if !ok {
			t.Fatal("gromit.yaml missing model_timeouts entry for sonnet")
		}
		if sonnet.Timeout <= cfg.Claude.Timeout {
			t.Errorf("sonnet invocation timeout (%d) should be greater than base timeout (%d)",
				sonnet.Timeout, cfg.Claude.Timeout)
		}
	})

	t.Run("haiku_has_overrides", func(t *testing.T) {
		haiku, ok := cfg.Claude.ModelTimeouts["haiku"]
		if !ok {
			t.Fatal("gromit.yaml missing model_timeouts entry for haiku")
		}
		if haiku.StallTimeout >= cfg.Claude.StallTimeout {
			t.Errorf("haiku stall_timeout (%d) should be less than base stall_timeout (%d)",
				haiku.StallTimeout, cfg.Claude.StallTimeout)
		}
		if haiku.BeadTimeout >= cfg.Claude.BeadTimeout {
			t.Errorf("haiku bead_timeout (%d) should be less than base bead_timeout (%d)",
				haiku.BeadTimeout, cfg.Claude.BeadTimeout)
		}
	})
}

func TestPrecheckVerificationDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Precheck.Verification.Enabled == nil {
		t.Fatal("Verification.Enabled should not be nil after SetDefaults")
	}
	if !*cfg.Precheck.Verification.Enabled {
		t.Error("Verification.Enabled should default to true")
	}
	if cfg.Precheck.Verification.TimeoutSeconds != 120 {
		t.Errorf("Verification.TimeoutSeconds should default to 120, got %d", cfg.Precheck.Verification.TimeoutSeconds)
	}
}

func TestPrecheckVerificationIsEnabledNilPointer(t *testing.T) {
	v := PrecheckVerificationConfig{}
	if !v.IsVerificationEnabled() {
		t.Errorf("expected IsVerificationEnabled() to return true for nil pointer")
	}
}

func TestPrecheckVerificationIsEnabledExplicitTrue(t *testing.T) {
	trueVal := true
	v := PrecheckVerificationConfig{Enabled: &trueVal}
	if !v.IsVerificationEnabled() {
		t.Errorf("expected IsVerificationEnabled() to return true for explicit true")
	}
}

func TestPrecheckVerificationIsEnabledExplicitFalse(t *testing.T) {
	falseVal := false
	v := PrecheckVerificationConfig{Enabled: &falseVal}
	if v.IsVerificationEnabled() {
		t.Errorf("expected IsVerificationEnabled() to return false for explicit false")
	}
}

func TestPrecheckVerificationFromYAML(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		expectEnabled bool
		expectTimeout int
	}{
		{
			name: "Verification explicit",
			yaml: `precheck:
  verification:
    enabled: false
    timeout_seconds: 60
`,
			expectEnabled: false,
			expectTimeout: 60,
		},
		{
			name:          "Verification uses defaults",
			yaml:          "",
			expectEnabled: true,
			expectTimeout: 120,
		},
		{
			name: "Verification timeout only",
			yaml: `precheck:
  verification:
    timeout_seconds: 90
`,
			expectEnabled: true,
			expectTimeout: 90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "gromit.yaml")
			if err := os.WriteFile(cfgPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("writing config: %v", err)
			}

			cfg, err := Load(cfgPath)
			if err != nil {
				t.Fatalf("loading config: %v", err)
			}

			if cfg.Precheck.Verification.IsVerificationEnabled() != tt.expectEnabled {
				t.Errorf("expected verification enabled=%v, got %v", tt.expectEnabled, cfg.Precheck.Verification.IsVerificationEnabled())
			}
			if cfg.Precheck.Verification.TimeoutSeconds != tt.expectTimeout {
				t.Errorf("expected verification timeout=%d, got %d", tt.expectTimeout, cfg.Precheck.Verification.TimeoutSeconds)
			}
		})
	}
}

func TestValidationConfig_FastCommandsFallback(t *testing.T) {
	cfg := &Config{
		Validation: ValidationConfig{
			Commands: []string{"legacy"},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	got := cfg.Validation.FastCommandsOrDefault()
	if len(got) != 1 || got[0] != "legacy" {
		t.Fatalf("FastCommandsOrDefault = %v, want [legacy]", got)
	}
}

func TestValidationConfig_FullCommandsFallback(t *testing.T) {
	cfg := &Config{
		Validation: ValidationConfig{
			Commands: []string{"legacy"},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	got := cfg.Validation.FullCommandsOrDefault()
	if len(got) != 1 || got[0] != "legacy" {
		t.Fatalf("FullCommandsOrDefault = %v, want [legacy]", got)
	}
}

func TestValidationConfig_NonInteractiveDefaultTrue(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	if !cfg.Validation.IsNonInteractive() {
		t.Fatal("expected validation non_interactive default to true")
	}
}

func TestValidationConfig_MaxParallelCommandsDefaultOne(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	if cfg.Validation.MaxParallelCommands != 1 {
		t.Fatalf("MaxParallelCommands = %d, want 1", cfg.Validation.MaxParallelCommands)
	}
}

func TestValidationConfig_MaxParallelCommandsYAML(t *testing.T) {
	yamlContent := `
validation:
  max_parallel_commands: 3
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Validation.MaxParallelCommands != 3 {
		t.Fatalf("MaxParallelCommands = %d, want 3", cfg.Validation.MaxParallelCommands)
	}
}

func TestValidationConfig_MaxSubBeadsDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	if cfg.Validation.PlanMaxSubBeads == nil || *cfg.Validation.PlanMaxSubBeads != DefaultMaxSubBeads {
		t.Fatalf("PlanMaxSubBeads = %v, want %d", cfg.Validation.PlanMaxSubBeads, DefaultMaxSubBeads)
	}
	if cfg.Validation.RuntimeMaxSubBeads != DefaultMaxSubBeads {
		t.Fatalf("RuntimeMaxSubBeads = %d, want %d", cfg.Validation.RuntimeMaxSubBeads, DefaultMaxSubBeads)
	}
}

func TestValidationConfig_MaxSubBeadsYAML(t *testing.T) {
	yamlContent := `
validation:
  plan_max_sub_beads: 0
  runtime_max_sub_beads: 10
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Validation.PlanMaxSubBeads == nil || *cfg.Validation.PlanMaxSubBeads != 0 {
		t.Fatalf("PlanMaxSubBeads = %v, want 0", cfg.Validation.PlanMaxSubBeads)
	}
	if cfg.Validation.RuntimeMaxSubBeads != 10 {
		t.Fatalf("RuntimeMaxSubBeads = %d, want 10", cfg.Validation.RuntimeMaxSubBeads)
	}
}

func TestValidationConfig_CommandTimeoutYAML(t *testing.T) {
	yamlContent := `
validation:
  command_timeout: 30s
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if got := time.Duration(cfg.Validation.CommandTimeout); got != 30*time.Second {
		t.Fatalf("CommandTimeout = %s, want %s", got, 30*time.Second)
	}
}

func TestValidationConfig_CommandTimeoutYAMLIntegerSeconds(t *testing.T) {
	yamlContent := `
validation:
  command_timeout: 30
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if got := time.Duration(cfg.Validation.CommandTimeout); got != 30*time.Second {
		t.Fatalf("CommandTimeout = %s, want %s", got, 30*time.Second)
	}
}

func TestValidationConfig_RunFinalFullGateDefaultTrue(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	if !cfg.Validation.ShouldRunFinalFullGate() {
		t.Fatal("expected run_final_full_gate default to true")
	}
}

func TestValidationConfig_RunFinalFullGateYAML(t *testing.T) {
	yamlContent := `
validation:
  run_final_full_gate: false
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Validation.ShouldRunFinalFullGate() {
		t.Fatal("expected run_final_full_gate=false from yaml")
	}
}

func TestScopeGoTestCommands(t *testing.T) {
	commands := []string{"go test -count=1 ./...", "go vet ./..."}
	touched := []string{"internal/runner", "internal/provider", "internal/runner"}

	got := ScopeGoTestCommands(commands, touched)
	want0 := "go test -count=1 ./internal/runner/... ./internal/provider/..."
	if len(got) != 2 {
		t.Fatalf("ScopeGoTestCommands returned %d commands, want 2", len(got))
	}
	if got[0] != want0 {
		t.Fatalf("scoped go test command = %q, want %q", got[0], want0)
	}
	if got[1] != "go vet ./..." {
		t.Fatalf("non-go-test command should be unchanged, got %q", got[1])
	}
}

func TestScopeGoTestCommands_CollapsesNestedPackages(t *testing.T) {
	commands := []string{"go test -count=1 ./..."}
	touched := []string{"internal/runner/andon", "internal/runner", "cmd/gromit"}

	got := ScopeGoTestCommands(commands, touched)
	want := "go test -count=1 ./internal/runner/... ./cmd/gromit/..."

	if len(got) != 1 {
		t.Fatalf("ScopeGoTestCommands returned %d commands, want 1", len(got))
	}
	if got[0] != want {
		t.Fatalf("scoped go test command = %q, want %q", got[0], want)
	}
}

func TestScopeGoTestCommands_CollapsesInterleavedChildren(t *testing.T) {
	commands := []string{"go test -count=1 ./..."}
	// Parent "x" appears after children "x/a" and "x/b", interleaved with
	// unrelated packages "y" and "z". This exercises the filter loop where
	// multiple elements are removed while others are kept.
	touched := []string{"x/a", "y", "x/b", "z", "x"}

	got := ScopeGoTestCommands(commands, touched)
	want := "go test -count=1 ./y/... ./z/... ./x/..."

	if len(got) != 1 {
		t.Fatalf("ScopeGoTestCommands returned %d commands, want 1", len(got))
	}
	if got[0] != want {
		t.Fatalf("scoped go test command = %q, want %q", got[0], want)
	}
}

func TestScopeGoTestCommands_IncludesRootPackage(t *testing.T) {
	commands := []string{"go test -count=1 ./..."}
	touched := []string{".", "internal/runner"}

	got := ScopeGoTestCommands(commands, touched)
	want := "go test -count=1 . ./internal/runner/..."

	if len(got) != 1 {
		t.Fatalf("ScopeGoTestCommands returned %d commands, want 1", len(got))
	}
	if got[0] != want {
		t.Fatalf("scoped go test command = %q, want %q", got[0], want)
	}
}

// findProjectRoot walks up from the current working directory to find the
// project root (directory containing gromit.yaml).
func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "gromit.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no gromit.yaml found)")
		}
		dir = parent
	}
}

func TestProviderDefCostFieldsUnmarshal(t *testing.T) {
	yamlContent := `
providers:
  claude:
    binary: claude
    cost_per_1k_input: 0.015
    cost_per_1k_output: 0.075
  openai:
    binary: codex
    cost_per_1k_input: 0.005
    cost_per_1k_output: 0.015
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

	claude := cfg.Providers["claude"]
	if claude.CostPer1kInput != 0.015 {
		t.Errorf("claude.CostPer1kInput = %v, want 0.015", claude.CostPer1kInput)
	}
	if claude.CostPer1kOutput != 0.075 {
		t.Errorf("claude.CostPer1kOutput = %v, want 0.075", claude.CostPer1kOutput)
	}

	openai := cfg.Providers["openai"]
	if openai.CostPer1kInput != 0.005 {
		t.Errorf("openai.CostPer1kInput = %v, want 0.005", openai.CostPer1kInput)
	}
	if openai.CostPer1kOutput != 0.015 {
		t.Errorf("openai.CostPer1kOutput = %v, want 0.015", openai.CostPer1kOutput)
	}
}

func TestProviderDefModelCostsUnmarshal(t *testing.T) {
	yamlContent := `
providers:
  openai:
    binary: codex
    cost_per_1k_input: 0.00175
    cost_per_1k_output: 0.014
    model_costs:
      gpt-5.3-codex:
        cost_per_1k_input: 0.00875
        cost_per_1k_output: 0.070
      gpt-5.1-codex-mini:
        cost_per_1k_input: 0.00025
        cost_per_1k_output: 0.0015
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

	openai := cfg.Providers["openai"]
	if len(openai.ModelCosts) != 2 {
		t.Fatalf("openai.ModelCosts has %d entries, want 2", len(openai.ModelCosts))
	}

	high := openai.ModelCosts["gpt-5.3-codex"]
	if high == nil {
		t.Fatal("model_costs missing gpt-5.3-codex entry")
	}
	if high.CostPer1kInput != 0.00875 {
		t.Errorf("gpt-5.3-codex CostPer1kInput = %v, want 0.00875", high.CostPer1kInput)
	}
	if high.CostPer1kOutput != 0.070 {
		t.Errorf("gpt-5.3-codex CostPer1kOutput = %v, want 0.070", high.CostPer1kOutput)
	}

	low := openai.ModelCosts["gpt-5.1-codex-mini"]
	if low == nil {
		t.Fatal("model_costs missing gpt-5.1-codex-mini entry")
	}
	if low.CostPer1kInput != 0.00025 {
		t.Errorf("gpt-5.1-codex-mini CostPer1kInput = %v, want 0.00025", low.CostPer1kInput)
	}
	if low.CostPer1kOutput != 0.0015 {
		t.Errorf("gpt-5.1-codex-mini CostPer1kOutput = %v, want 0.0015", low.CostPer1kOutput)
	}
}

func TestProviderDefReasoningEffortUnmarshal(t *testing.T) {
	yamlContent := `
providers:
  openai:
    binary: codex
    reasoning_effort:
      high: high
      medium: medium
      low: low
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

	openai := cfg.Providers["openai"]
	if openai.ReasoningEffort["high"] != "high" {
		t.Errorf("openai.ReasoningEffort[high] = %q, want %q", openai.ReasoningEffort["high"], "high")
	}
	if openai.ReasoningEffort["medium"] != "medium" {
		t.Errorf("openai.ReasoningEffort[medium] = %q, want %q", openai.ReasoningEffort["medium"], "medium")
	}
	if openai.ReasoningEffort["low"] != "low" {
		t.Errorf("openai.ReasoningEffort[low] = %q, want %q", openai.ReasoningEffort["low"], "low")
	}
}

func TestProviderDefCostFieldsDefaultToZero(t *testing.T) {
	yamlContent := `
providers:
  claude:
    binary: claude
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

	claude := cfg.Providers["claude"]
	if claude.CostPer1kInput != 0 {
		t.Errorf("claude.CostPer1kInput = %v, want 0", claude.CostPer1kInput)
	}
	if claude.CostPer1kOutput != 0 {
		t.Errorf("claude.CostPer1kOutput = %v, want 0", claude.CostPer1kOutput)
	}
}

func TestProviderDefEstimateCost(t *testing.T) {
	tests := []struct {
		name         string
		def          ProviderDef
		inputTokens  int
		outputTokens int
		want         float64
	}{
		{
			name:         "both pricing configured",
			def:          ProviderDef{CostPer1kInput: 0.015, CostPer1kOutput: 0.075},
			inputTokens:  1000,
			outputTokens: 500,
			want:         0.015 + 0.075*0.5,
		},
		{
			name:         "input only pricing",
			def:          ProviderDef{CostPer1kInput: 0.010},
			inputTokens:  2000,
			outputTokens: 0,
			want:         0.020,
		},
		{
			name:         "output only pricing",
			def:          ProviderDef{CostPer1kOutput: 0.060},
			inputTokens:  0,
			outputTokens: 3000,
			want:         0.180,
		},
		{
			name:         "zero tokens returns zero",
			def:          ProviderDef{CostPer1kInput: 0.015, CostPer1kOutput: 0.075},
			inputTokens:  0,
			outputTokens: 0,
			want:         0,
		},
		{
			name:         "no pricing configured returns zero",
			def:          ProviderDef{},
			inputTokens:  1000,
			outputTokens: 1000,
			want:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.EstimateCost(tt.inputTokens, tt.outputTokens)
			if got != tt.want {
				t.Errorf("EstimateCost(%d, %d) = %v, want %v", tt.inputTokens, tt.outputTokens, got, tt.want)
			}
		})
	}
}

func TestProviderDefEstimateCostForModel(t *testing.T) {
	def := ProviderDef{
		CostPer1kInput:  0.00175,
		CostPer1kOutput: 0.014,
	ModelCosts: map[string]*ModelCost{
		"gpt-5.3-codex": {
			CostPer1kInput:  0.00875,
			CostPer1kOutput: 0.070,
		},
		"gpt-5.1-codex-mini": {
			CostPer1kInput:  0.00025,
			CostPer1kOutput: 0.0015,
		},
		"gpt-5.4-codex-zero": {
			CostPer1kInput:  0,
			CostPer1kOutput: 0,
		},
	},
}

	tests := []struct {
		name         string
		model        string
		inputTokens  int
		outputTokens int
		want         float64
	}{
		{
			name:         "model-specific high tier",
			model:        "gpt-5.3-codex",
			inputTokens:  1000,
			outputTokens: 1000,
			want:         0.00875 + 0.070,
		},
		{
			name:         "model-specific low tier",
			model:        "gpt-5.1-codex-mini",
			inputTokens:  1000,
			outputTokens: 1000,
			want:         0.00025 + 0.0015,
		},
		{
			name:         "unknown model falls back to provider rate",
			model:        "gpt-5.2-codex",
			inputTokens:  1000,
			outputTokens: 1000,
			want:         0.00175 + 0.014,
		},
		{
			name:         "empty model falls back to provider rate",
			model:        "",
			inputTokens:  1000,
			outputTokens: 1000,
			want:         0.00175 + 0.014,
		},
		{
			name:         "zero-value model-specific costs fall back to provider rate",
			model:        "gpt-5.4-codex-zero",
			inputTokens:  1000,
			outputTokens: 1000,
			want:         0.00175 + 0.014,
		},
		{
			name:         "zero tokens returns zero regardless of model",
			model:        "gpt-5.3-codex",
			inputTokens:  0,
			outputTokens: 0,
			want:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := def.EstimateCostForModel(tt.model, tt.inputTokens, tt.outputTokens)
			const epsilon = 1e-9
			if diff := got - tt.want; diff < -epsilon || diff > epsilon {
				t.Errorf("EstimateCostForModel(%q, %d, %d) = %v, want %v", tt.model, tt.inputTokens, tt.outputTokens, got, tt.want)
			}
		})
	}
}

func TestSpecGateConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	assertSpecGateDefaults(t, cfg, "after SetDefaults")
}

func TestSpecGateConfigDefaultWhenOmittedFromYAML(t *testing.T) {
	yamlContent := `
models:
  p0: opus
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

	assertSpecGateDefaults(t, cfg, "when omitted")
}

func TestSpecGateConfigYAMLDeserialization(t *testing.T) {
	yamlContent := `
spec_gate:
  enabled: true
  max_cycles: 5
  model: opus
  auto_trigger: false
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

	if cfg.SpecGate.Enabled == nil {
		t.Fatal("SpecGate.Enabled is nil, want non-nil pointer")
	}
	if !*cfg.SpecGate.Enabled {
		t.Errorf("SpecGate.Enabled = false, want true")
	}
	if cfg.SpecGate.MaxCycles != 5 {
		t.Errorf("SpecGate.MaxCycles = %d, want 5", cfg.SpecGate.MaxCycles)
	}
	if cfg.SpecGate.Model != "opus" {
		t.Errorf("SpecGate.Model = %q, want %q", cfg.SpecGate.Model, "opus")
	}
	if cfg.SpecGate.AutoTrigger == nil {
		t.Fatal("SpecGate.AutoTrigger is nil, want non-nil pointer")
	}
	if *cfg.SpecGate.AutoTrigger {
		t.Errorf("*SpecGate.AutoTrigger = true, want false")
	}
}

func TestSpecGateConfigYAMLRoundTrip(t *testing.T) {
	enabled := false
	autoTrigger := false

	in := Config{
		SpecGate: SpecGateConfig{
			Enabled:     &enabled,
			MaxCycles:   7,
			Model:       ModelOpus,
			AutoTrigger: &autoTrigger,
		},
	}

	data, err := yaml.Marshal(in)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var out Config
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	out.SetDefaults()

	if out.SpecGate.Enabled == nil {
		t.Fatal("out.SpecGate.Enabled is nil, want non-nil pointer")
	}
	if *out.SpecGate.Enabled {
		t.Errorf("*out.SpecGate.Enabled = true, want false")
	}
	if out.SpecGate.MaxCycles != 7 {
		t.Errorf("out.SpecGate.MaxCycles = %d, want 7", out.SpecGate.MaxCycles)
	}
	if out.SpecGate.Model != ModelOpus {
		t.Errorf("out.SpecGate.Model = %q, want %q", out.SpecGate.Model, ModelOpus)
	}
	if out.SpecGate.AutoTrigger == nil {
		t.Fatal("out.SpecGate.AutoTrigger is nil, want non-nil pointer")
	}
	if *out.SpecGate.AutoTrigger {
		t.Errorf("*out.SpecGate.AutoTrigger = true, want false")
	}
}

func TestSpecGateIsEnabled(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{"nil defaults to true", nil, true},
		{"enabled true", &trueVal, true},
		{"enabled false", &falseVal, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SpecGateConfig{Enabled: tt.enabled}
			if got := s.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpecGateIsAutoTrigger(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name        string
		autoTrigger *bool
		want        bool
	}{
		{"nil defaults to true", nil, true},
		{"explicit true", &trueVal, true},
		{"explicit false", &falseVal, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SpecGateConfig{AutoTrigger: tt.autoTrigger}
			if got := s.IsAutoTrigger(); got != tt.want {
				t.Errorf("IsAutoTrigger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func assertSpecGateDefaults(t *testing.T, cfg *Config, context string) {
	t.Helper()
	if context != "" {
		context = " " + context
	}

	if cfg.SpecGate.MaxCycles != DefaultSpecGateMaxCycles {
		t.Errorf("SpecGate.MaxCycles = %d, want %d%s", cfg.SpecGate.MaxCycles, DefaultSpecGateMaxCycles, context)
	}
	if cfg.SpecGate.Model != ModelSonnet {
		t.Errorf("SpecGate.Model = %q, want %q%s", cfg.SpecGate.Model, ModelSonnet, context)
	}
	if cfg.SpecGate.Enabled == nil {
		t.Fatalf("SpecGate.Enabled is nil, want non-nil pointer%s", context)
	}
	if !*cfg.SpecGate.Enabled {
		t.Errorf("*SpecGate.Enabled = false, want true%s", context)
	}
	if cfg.SpecGate.AutoTrigger == nil {
		t.Fatalf("SpecGate.AutoTrigger is nil, want non-nil pointer%s", context)
	}
	if !*cfg.SpecGate.AutoTrigger {
		t.Errorf("*SpecGate.AutoTrigger = false, want true%s", context)
	}
}

func TestRoutingCircuitBreakerDefaultsWhenEnabled(t *testing.T) {
	yamlContent := `
routing:
  circuit_breaker:
    enabled: true
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

	if !cfg.Routing.CircuitBreaker.Enabled {
		t.Fatal("Routing.CircuitBreaker.Enabled = false, want true")
	}
	if cfg.Routing.CircuitBreaker.WindowSize != 10 {
		t.Errorf("Routing.CircuitBreaker.WindowSize = %d, want 10", cfg.Routing.CircuitBreaker.WindowSize)
	}
	if cfg.Routing.CircuitBreaker.FailureThreshold != 0.3 {
		t.Errorf("Routing.CircuitBreaker.FailureThreshold = %v, want 0.3", cfg.Routing.CircuitBreaker.FailureThreshold)
	}
	if cfg.Routing.CircuitBreaker.DegradedFloor != 20 {
		t.Errorf("Routing.CircuitBreaker.DegradedFloor = %d, want 20", cfg.Routing.CircuitBreaker.DegradedFloor)
	}
	if cfg.Routing.CircuitBreaker.RecoverySuccesses != 5 {
		t.Errorf("Routing.CircuitBreaker.RecoverySuccesses = %d, want 5", cfg.Routing.CircuitBreaker.RecoverySuccesses)
	}
}

func TestRoutingCircuitBreakerOmittedNoBehaviorChange(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	if cfg.Routing.CircuitBreaker.Enabled {
		t.Error("Routing.CircuitBreaker.Enabled = true, want false when omitted")
	}
	if cfg.Routing.CircuitBreaker.WindowSize != 0 {
		t.Errorf("Routing.CircuitBreaker.WindowSize = %d, want 0 when omitted", cfg.Routing.CircuitBreaker.WindowSize)
	}
	if cfg.Routing.CircuitBreaker.FailureThreshold != 0 {
		t.Errorf("Routing.CircuitBreaker.FailureThreshold = %v, want 0 when omitted", cfg.Routing.CircuitBreaker.FailureThreshold)
	}
	if cfg.Routing.CircuitBreaker.DegradedFloor != 0 {
		t.Errorf("Routing.CircuitBreaker.DegradedFloor = %d, want 0 when omitted", cfg.Routing.CircuitBreaker.DegradedFloor)
	}
	if cfg.Routing.CircuitBreaker.RecoverySuccesses != 0 {
		t.Errorf("Routing.CircuitBreaker.RecoverySuccesses = %d, want 0 when omitted", cfg.Routing.CircuitBreaker.RecoverySuccesses)
	}
}

func TestRoutingCircuitBreakerValidationRejectsInvalidWhenEnabled(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantError   string
	}{
		{
			name: "invalid window_size",
			yamlContent: `
routing:
  circuit_breaker:
    enabled: true
    window_size: -1
`,
			wantError: "routing.circuit_breaker.window_size",
		},
		{
			name: "invalid failure_threshold",
			yamlContent: `
routing:
  circuit_breaker:
    enabled: true
    failure_threshold: 1.2
`,
			wantError: "routing.circuit_breaker.failure_threshold",
		},
		{
			name: "invalid degraded_floor",
			yamlContent: `
routing:
  circuit_breaker:
    enabled: true
    degraded_floor: 101
`,
			wantError: "routing.circuit_breaker.degraded_floor",
		},
		{
			name: "invalid recovery_successes",
			yamlContent: `
routing:
  circuit_breaker:
    enabled: true
    recovery_successes: -1
`,
			wantError: "routing.circuit_breaker.recovery_successes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfgPath := filepath.Join(tmpDir, "gromit.yaml")
			if err := os.WriteFile(cfgPath, []byte(tt.yamlContent), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := Load(cfgPath)
			if err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Load() error = %v, want error containing %q", err, tt.wantError)
			}
		})
	}
}

func TestLoadLegacyCompatibilityConfig_EmitsMigrationWarningByDefault(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte("models:\n  p0: opus\n"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	var warning strings.Builder
	originalWriter := configWarningWriter
	configWarningWriter = &warning
	t.Cleanup(func() {
		configWarningWriter = originalWriter
	})

	if _, err := Load(cfgPath); err != nil {
		t.Fatalf("Load(%q) error = %v", cfgPath, err)
	}

	warningText := warning.String()
	if !strings.Contains(warningText, CompatibilityDeprecationMarkerLegacyHardcodedDefaults) {
		t.Fatalf("warning = %q, want legacy deprecation marker %q", warningText, CompatibilityDeprecationMarkerLegacyHardcodedDefaults)
	}
	if !strings.Contains(warningText, CompatibilityStrictDefaultCutoverDate) {
		t.Fatalf("warning = %q, want cutoff date %q", warningText, CompatibilityStrictDefaultCutoverDate)
	}
}

func TestLoadLegacyCompatibilityConfig_StrictModeRejectsLegacyFallback(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	yaml := `compatibility:
  strict_legacy_fallback: true
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() error = nil, want strict legacy fallback failure")
	}
	if !strings.Contains(err.Error(), "compatibility.strict_legacy_fallback") {
		t.Fatalf("Load() error = %q, want mention of compatibility.strict_legacy_fallback", err.Error())
	}
}

func TestLoadBackwardCompatibility_PartialConfigsLoadSuccessfully(t *testing.T) {
	testCases := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, cfg *Config)
	}{
		{
			name: "empty config uses all legacy defaults",
			yaml: "",
			validate: func(t *testing.T, cfg *Config) {
				resolved := cfg.ResolveCompatibilityContext()
				if resolved.Profile.Value != "go" {
					t.Errorf("Profile.Value = %q, want go", resolved.Profile.Value)
				}
				if resolved.Profile.Source != CompatibilitySourceLegacyFallback {
					t.Errorf("Profile.Source = %q, want legacy_fallback", resolved.Profile.Source)
				}
				if resolved.TrackerBackend.Value != "bd" {
					t.Errorf("TrackerBackend.Value = %q, want bd", resolved.TrackerBackend.Value)
				}
				if resolved.MethodologyAdapter.Value != "go" {
					t.Errorf("MethodologyAdapter.Value = %q, want go", resolved.MethodologyAdapter.Value)
				}
			},
		},
		{
			name: "config with only profile specified",
			yaml: `project:
  profile: "python"
`,
			validate: func(t *testing.T, cfg *Config) {
				resolved := cfg.ResolveCompatibilityContext()
				if resolved.Profile.Value != "python" {
					t.Errorf("Profile.Value = %q, want python", resolved.Profile.Value)
				}
				if resolved.Profile.Source != CompatibilitySourceExplicit {
					t.Errorf("Profile.Source = %q, want explicit", resolved.Profile.Source)
				}
				// Tracker backend should fall to profile default
				if resolved.TrackerBackend.Source != CompatibilitySourceProfileDefault {
					t.Errorf("TrackerBackend.Source = %q, want profile_default", resolved.TrackerBackend.Source)
				}
				if resolved.TrackerBackend.Value != "bd" {
					t.Errorf("TrackerBackend.Value = %q, want bd", resolved.TrackerBackend.Value)
				}
			},
		},
		{
			name: "config with tracker backend but no profile falls back to legacy",
			yaml: `tracker:
  backend: "bd"
`,
			validate: func(t *testing.T, cfg *Config) {
				resolved := cfg.ResolveCompatibilityContext()
				// Profile is not explicit, so it's legacy fallback
				if resolved.Profile.Source != CompatibilitySourceLegacyFallback {
					t.Errorf("Profile.Source = %q, want legacy_fallback", resolved.Profile.Source)
				}
				// Tracker backend is explicit
				if resolved.TrackerBackend.Value != "bd" {
					t.Errorf("TrackerBackend.Value = %q, want bd", resolved.TrackerBackend.Value)
				}
				if resolved.TrackerBackend.Source != CompatibilitySourceExplicit {
					t.Errorf("TrackerBackend.Source = %q, want explicit", resolved.TrackerBackend.Source)
				}
			},
		},
		{
			name: "config with methodology adapter but no explicit profile or tracker",
			yaml: `methodology:
  adapter: "go"
`,
			validate: func(t *testing.T, cfg *Config) {
				resolved := cfg.ResolveCompatibilityContext()
				if resolved.MethodologyAdapter.Value != "go" {
					t.Errorf("MethodologyAdapter.Value = %q, want go", resolved.MethodologyAdapter.Value)
				}
				if resolved.MethodologyAdapter.Source != CompatibilitySourceExplicit {
					t.Errorf("MethodologyAdapter.Source = %q, want explicit", resolved.MethodologyAdapter.Source)
				}
				// Profile and tracker should use legacy defaults
				if resolved.Profile.Source != CompatibilitySourceLegacyFallback {
					t.Errorf("Profile.Source = %q, want legacy_fallback", resolved.Profile.Source)
				}
				if resolved.TrackerBackend.Source != CompatibilitySourceLegacyFallback {
					t.Errorf("TrackerBackend.Source = %q, want legacy_fallback", resolved.TrackerBackend.Source)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfgPath := filepath.Join(tmpDir, "gromit.yaml")
			if err := os.WriteFile(cfgPath, []byte(tc.yaml), 0644); err != nil {
				t.Fatalf("writing config: %v", err)
			}

			cfg, err := Load(cfgPath)
			if err != nil {
				t.Fatalf("Load(%q) error = %v", cfgPath, err)
			}

			tc.validate(t, cfg)
		})
	}
}

func TestLoadBackwardCompatibility_PartialConfigsDoNotImplicitlyInject(t *testing.T) {
	testCases := []struct {
		name             string
		yaml             string
		shouldHaveProfile bool
		shouldHaveTracker bool
		shouldHaveAdapter bool
	}{
		{
			name: "explicit profile does not implicitly inject tracker backend",
			yaml: `project:
  profile: "node"
`,
			shouldHaveProfile: true,
			shouldHaveTracker: false,
			shouldHaveAdapter: false,
		},
		{
			name: "explicit tracker backend does not implicitly inject profile",
			yaml: `tracker:
  backend: "bd"
`,
			shouldHaveProfile: false,
			shouldHaveTracker: true,
			shouldHaveAdapter: false,
		},
		{
			name: "explicit methodology adapter does not implicitly inject profile or tracker",
			yaml: `methodology:
  adapter: "go"
`,
			shouldHaveProfile: false,
			shouldHaveTracker: false,
			shouldHaveAdapter: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfgPath := filepath.Join(tmpDir, "gromit.yaml")
			if err := os.WriteFile(cfgPath, []byte(tc.yaml), 0644); err != nil {
				t.Fatalf("writing config: %v", err)
			}

			cfg, err := Load(cfgPath)
			if err != nil {
				t.Fatalf("Load(%q) error = %v", cfgPath, err)
			}

			// Check that each field is independent
			if tc.shouldHaveProfile {
				if cfg.Project.Profile == "" {
					t.Error("expected explicit project.profile, got empty")
				}
			} else {
				if cfg.Project.Profile != "" {
					t.Errorf("unexpected project.profile = %q, want implicit injection", cfg.Project.Profile)
				}
			}

			if tc.shouldHaveTracker {
				if cfg.Tracker.Backend == "" {
					t.Error("expected explicit tracker.backend, got empty")
				}
			} else {
				if cfg.Tracker.Backend != "" {
					t.Errorf("unexpected tracker.backend = %q, want empty (no implicit injection)", cfg.Tracker.Backend)
				}
			}

			if tc.shouldHaveAdapter {
				if cfg.Methodology.Adapter == "" {
					t.Error("expected explicit methodology.adapter, got empty")
				}
			} else {
				if cfg.Methodology.Adapter != "" {
					t.Errorf("unexpected methodology.adapter = %q, want empty (no implicit injection)", cfg.Methodology.Adapter)
				}
			}
		})
	}
}
