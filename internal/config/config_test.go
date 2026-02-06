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
			P0:     "opus",
			P1:     "sonnet",
			P2:     "haiku",
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
			P0:     "opus",
			P1:     "sonnet",
			P2:     "haiku",
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
			P0:     "opus",
			P1:     "sonnet",
			P2:     "haiku",
			Labels: map[string]string{
				"spec:auth":  "opus",
				"spec:ui":    "haiku",
				"spec:data":  "sonnet",
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
		{0, []string{"complexity:high"}, "opus"},  // P0 + label: should be opus
		{1, []string{"complexity:high"}, "opus"},  // P1 + label: should be opus
		{2, []string{"complexity:high"}, "opus"},  // P2 + label: should be opus
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
		{0, []string{"complexity:low"}, "haiku"},  // P0 + low complexity: should be haiku
		{1, []string{"complexity:low"}, "haiku"},  // P1 + low complexity: should be haiku
		{2, []string{"complexity:low"}, "haiku"},  // P2 + low complexity: should be haiku
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
	cfg.setDefaults()
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	if cfg.Claude.Timeout != 600 {
		t.Errorf("expected Timeout=600, got %d", cfg.Claude.Timeout)
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
	if cfg.Paths.RalphDir != ".ralph" {
		t.Errorf("expected RalphDir='.ralph', got %q", cfg.Paths.RalphDir)
	}
	if cfg.Paths.Templates != ".ralph/templates" {
		t.Errorf("expected Templates='.ralph/templates', got %q", cfg.Paths.Templates)
	}
	if cfg.Paths.Specs != ".ralph/specs" {
		t.Errorf("expected Specs='.ralph/specs', got %q", cfg.Paths.Specs)
	}
	if cfg.Paths.Logs != ".ralph/logs" {
		t.Errorf("expected Logs='.ralph/logs', got %q", cfg.Paths.Logs)
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
  ralph_dir: .custom-ralph
  templates: .custom-ralph/templates
  specs: .custom-ralph/specs
  logs: .custom-ralph/logs
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

	if cfg.Paths.RalphDir != ".custom-ralph" {
		t.Errorf("expected RalphDir='.custom-ralph', got %q", cfg.Paths.RalphDir)
	}
	if cfg.Paths.Templates != ".custom-ralph/templates" {
		t.Errorf("expected Templates='.custom-ralph/templates', got %q", cfg.Paths.Templates)
	}
	if cfg.Paths.Specs != ".custom-ralph/specs" {
		t.Errorf("expected Specs='.custom-ralph/specs', got %q", cfg.Paths.Specs)
	}
	if cfg.Paths.Logs != ".custom-ralph/logs" {
		t.Errorf("expected Logs='.custom-ralph/logs', got %q", cfg.Paths.Logs)
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
	if !contains(err.Error(), "reading config file") {
		t.Errorf("expected 'reading config file' in error, got: %v", err)
	}
}

func TestLoadInvalidYAMLSyntax(t *testing.T) {
	// Test error handling for invalid YAML syntax
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ralph.yaml")
	if err := os.WriteFile(cfgPath, []byte("invalid: [yaml: {broken"), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
	if !contains(err.Error(), "parsing config file") {
		t.Errorf("expected 'parsing config file' in error, got: %v", err)
	}
}

func TestLoadZeroTimeouts(t *testing.T) {
	// Test that zero timeouts are replaced with defaults
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	if cfg.Claude.Timeout != 600 {
		t.Errorf("expected Timeout=600, got %d", cfg.Claude.Timeout)
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
	yaml := `models:
  p0: ""
  p1: ""
  p2: ""
  validation: ""
claude:
  binary: ""
paths:
  ralph_dir: ""
  templates: ""
  specs: ""
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
	if cfg.Paths.RalphDir != ".ralph" {
		t.Errorf("expected RalphDir='.ralph', got %q", cfg.Paths.RalphDir)
	}
	if cfg.Paths.Templates != ".ralph/templates" {
		t.Errorf("expected Templates='.ralph/templates', got %q", cfg.Paths.Templates)
	}
	if cfg.Paths.Specs != ".ralph/specs" {
		t.Errorf("expected Specs='.ralph/specs', got %q", cfg.Paths.Specs)
	}
	if cfg.Paths.Logs != ".ralph/logs" {
		t.Errorf("expected Logs='.ralph/logs', got %q", cfg.Paths.Logs)
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfg.setDefaults()
	if cfg.Loop.StuckBeadThreshold != 3 {
		t.Errorf("expected default StuckBeadThreshold=3, got %d", cfg.Loop.StuckBeadThreshold)
	}
}

func TestStuckBeadThresholdFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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

func TestNormalizeNilFields(t *testing.T) {
	cfg := &Config{}
	cfg.normalizeNilFields()

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
	cfg.normalizeNilFields()

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
	cfgPath := filepath.Join(dir, "ralph.yaml")
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
func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
