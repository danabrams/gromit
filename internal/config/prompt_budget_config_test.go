package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPromptBudgetMaxCharsDefault(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Prompt.Budget.MaxChars != 20000 {
		t.Errorf("expected default Prompt.Budget.MaxChars=20000, got %d", cfg.Prompt.Budget.MaxChars)
	}
}

func TestPromptBudgetLearningCapCharsDefault(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Prompt.Budget.LearningCapChars != 2000 {
		t.Errorf("expected default Prompt.Budget.LearningCapChars=2000, got %d", cfg.Prompt.Budget.LearningCapChars)
	}
}

func TestPromptBudgetDefaultsWhenOmittedFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
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

	if cfg.Prompt.Budget.MaxChars != 20000 {
		t.Errorf("expected default Prompt.Budget.MaxChars=20000 when omitted from YAML, got %d", cfg.Prompt.Budget.MaxChars)
	}
	if cfg.Prompt.Budget.LearningCapChars != 2000 {
		t.Errorf("expected default Prompt.Budget.LearningCapChars=2000 when omitted from YAML, got %d", cfg.Prompt.Budget.LearningCapChars)
	}
}

func TestPromptBudgetFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yamlContent := `prompt:
  budget:
    max_chars: 15000
    learning_cap_chars: 1500
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Prompt.Budget.MaxChars != 15000 {
		t.Errorf("expected Prompt.Budget.MaxChars=15000 from YAML, got %d", cfg.Prompt.Budget.MaxChars)
	}
	if cfg.Prompt.Budget.LearningCapChars != 1500 {
		t.Errorf("expected Prompt.Budget.LearningCapChars=1500 from YAML, got %d", cfg.Prompt.Budget.LearningCapChars)
	}
}
