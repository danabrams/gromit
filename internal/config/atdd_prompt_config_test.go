package config

import "testing"

func TestATDDPromptConfigSetDefaults(t *testing.T) {
	cfg := &ATDDPromptConfig{}
	cfg.SetDefaults()

	if cfg.IncludeRules == nil || !*cfg.IncludeRules {
		t.Fatal("expected IncludeRules default true")
	}
	if cfg.IncludeSpec == nil || !*cfg.IncludeSpec {
		t.Fatal("expected IncludeSpec default true")
	}
	if cfg.IncludeClaudeMD == nil || !*cfg.IncludeClaudeMD {
		t.Fatal("expected IncludeClaudeMD default true")
	}
	if cfg.MaxChars != defaultATDDPromptMaxChars {
		t.Fatalf("expected MaxChars default %d, got %d", defaultATDDPromptMaxChars, cfg.MaxChars)
	}
	if cfg.MaxConfirmedLearningChars != defaultATDDPromptLearningCharsCap {
		t.Fatalf("expected MaxConfirmedLearningChars default %d, got %d", defaultATDDPromptLearningCharsCap, cfg.MaxConfirmedLearningChars)
	}

	falseVal := false
	cfg = &ATDDPromptConfig{
		IncludeRules:              &falseVal,
		IncludeSpec:               &falseVal,
		IncludeClaudeMD:           &falseVal,
		MaxChars:                  123,
		MaxConfirmedLearningChars: 321,
	}
	cfg.SetDefaults()

	if cfg.IncludeRules == nil || *cfg.IncludeRules {
		t.Fatal("expected IncludeRules false when explicitly set")
	}
	if cfg.IncludeSpec == nil || *cfg.IncludeSpec {
		t.Fatal("expected IncludeSpec false when explicitly set")
	}
	if cfg.IncludeClaudeMD == nil || *cfg.IncludeClaudeMD {
		t.Fatal("expected IncludeClaudeMD false when explicitly set")
	}
	if cfg.MaxChars != 123 {
		t.Fatalf("expected MaxChars unchanged when set, got %d", cfg.MaxChars)
	}
	if cfg.MaxConfirmedLearningChars != 321 {
		t.Fatalf("expected MaxConfirmedLearningChars unchanged when set, got %d", cfg.MaxConfirmedLearningChars)
	}
}
