package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReviewConfigDefaultTier(t *testing.T) {
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

	if cfg.Review.Tier != "medium" {
		t.Errorf("expected default review tier 'medium', got %q", cfg.Review.Tier)
	}
}
