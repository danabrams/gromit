package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadNormalizesLegacyReviewModelsToTiers(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  model: opus
  thorough:
    model: haiku
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

func TestLoadWarnsWhenMatchBuildModelIsConfigured(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `review:
  match_build_model: false
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	var warning bytes.Buffer
	originalWriter := configWarningWriter
	configWarningWriter = &warning
	t.Cleanup(func() {
		configWarningWriter = originalWriter
	})

	if _, err := Load(cfgPath); err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if !strings.Contains(warning.String(), "match_build_model") {
		t.Fatalf("warning = %q, want mention of match_build_model", warning.String())
	}
	if !strings.Contains(strings.ToLower(warning.String()), "deprecated") {
		t.Fatalf("warning = %q, want deprecated message", warning.String())
	}
}
