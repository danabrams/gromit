package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectModelNilReceiver(t *testing.T) {
	var cfg *Config
	result := cfg.SelectModel(1, nil)
	if result != "sonnet" {
		t.Errorf("expected 'sonnet' for nil Config, got %q", result)
	}
}

func TestSelectModelNilLabels(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	result := cfg.SelectModel(0, nil)
	if result != "opus" {
		t.Errorf("expected 'opus' for P0, got %q", result)
	}
}

func TestSelectModelPriority(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()

	tests := []struct {
		priority int
		want     string
	}{
		{0, "opus"},
		{1, "sonnet"},
		{2, "haiku"},
		{99, "sonnet"}, // Unknown defaults to sonnet
	}

	for _, tt := range tests {
		result := cfg.SelectModel(tt.priority, nil)
		if result != tt.want {
			t.Errorf("SelectModel(%d) = %q, want %q", tt.priority, result, tt.want)
		}
	}
}

func TestSelectModelLabelOverride(t *testing.T) {
	cfg := &Config{
		Models: ModelsConfig{
			P1:     "sonnet",
			Labels: map[string]string{"complexity:high": "opus"},
		},
	}
	result := cfg.SelectModel(1, []string{"complexity:high"})
	if result != "opus" {
		t.Errorf("expected label override to 'opus', got %q", result)
	}
}

func TestNextEscalationModelNilReceiver(t *testing.T) {
	var cfg *Config
	result := cfg.NextEscalationModel("haiku")
	if result != "" {
		t.Errorf("expected empty string for nil Config, got %q", result)
	}
}

func TestNextEscalationModelDisabled(t *testing.T) {
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: false,
			Chain:   []string{"haiku", "sonnet", "opus"},
		},
	}
	result := cfg.NextEscalationModel("haiku")
	if result != "" {
		t.Errorf("expected empty string when escalation disabled, got %q", result)
	}
}

func TestNextEscalationModelChain(t *testing.T) {
	cfg := &Config{
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"haiku", "sonnet", "opus"},
		},
	}

	tests := []struct {
		current string
		want    string
	}{
		{"haiku", "sonnet"},
		{"sonnet", "opus"},
		{"opus", ""},   // End of chain
		{"unknown", ""}, // Not in chain
	}

	for _, tt := range tests {
		result := cfg.NextEscalationModel(tt.current)
		if result != tt.want {
			t.Errorf("NextEscalationModel(%q) = %q, want %q", tt.current, result, tt.want)
		}
	}
}

func TestLoadAndDefaults(t *testing.T) {
	// Create a minimal config file
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	if cfg.Paths.RalphDir != ".ralph" {
		t.Errorf("expected default RalphDir='.ralph', got %q", cfg.Paths.RalphDir)
	}
}

func TestStallTimeoutDefault(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	if cfg.Claude.StallTimeout != 120 {
		t.Errorf("expected default StallTimeout=120, got %d", cfg.Claude.StallTimeout)
	}
}

func TestStallTimeoutActiveDefault(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	if cfg.Claude.StallTimeoutActive != 300 {
		t.Errorf("expected default StallTimeoutActive=300, got %d", cfg.Claude.StallTimeoutActive)
	}
}

func TestStallTimeoutActiveFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/ralph.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfg.setDefaults()
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
	cfg.setDefaults()
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
	cfg.setDefaults()
	if cfg.Escalation.MaxRetriesPerBead != 10 {
		t.Errorf("expected default MaxRetriesPerBead=10, got %d", cfg.Escalation.MaxRetriesPerBead)
	}
}

func TestMaxRetriesPerBeadFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfg.setDefaults()

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
	cfg.setDefaults()

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
