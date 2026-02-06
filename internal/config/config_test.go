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
