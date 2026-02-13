package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLearningsMaxLearningCharsDefault tests that SetDefaults sets MaxLearningChars to 8000.
func TestLearningsMaxLearningCharsDefault(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Learnings.MaxLearningChars != 8000 {
		t.Errorf("expected default MaxLearningChars=8000, got %d", cfg.Learnings.MaxLearningChars)
	}
}

// TestLearningsMaxLearningCharsFromYAML tests that max_learning_chars can be overridden via YAML.
func TestLearningsMaxLearningCharsFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yamlContent := `learnings:
  max_learning_chars: 4000
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Learnings.MaxLearningChars != 4000 {
		t.Errorf("expected MaxLearningChars=4000 from YAML, got %d", cfg.Learnings.MaxLearningChars)
	}
}

// TestLearningsMaxLearningCharsZeroMeansDefault tests that omitting the field applies the default.
func TestLearningsMaxLearningCharsZeroMeansDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	// Minimal config with no learnings section
	yamlContent := `models:
  p0: opus
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Learnings.MaxLearningChars != 8000 {
		t.Errorf("expected default MaxLearningChars=8000 when omitted from YAML, got %d", cfg.Learnings.MaxLearningChars)
	}
}
