package config

import (
	"os"
	"path/filepath"
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
