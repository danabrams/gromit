package config

import "testing"

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
