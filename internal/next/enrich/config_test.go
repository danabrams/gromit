package enrich

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Provider != "claude" {
		t.Errorf("Provider = %q, want claude", cfg.Provider)
	}
	if cfg.Model != "sonnet" {
		t.Errorf("Model = %q, want sonnet", cfg.Model)
	}
	if cfg.Reasoning != "medium" {
		t.Errorf("Reasoning = %q, want medium", cfg.Reasoning)
	}
	if cfg.StalenessExpiryDays != 30 {
		t.Errorf("StalenessExpiryDays = %d, want 30", cfg.StalenessExpiryDays)
	}
}

func TestConfig_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Model = "opus"

	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loaded.Model != "opus" {
		t.Errorf("Model = %q, want opus", loaded.Model)
	}
}

func TestConfig_LoadMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Provider != "claude" {
		t.Errorf("should return defaults when file missing, got Provider=%q", cfg.Provider)
	}
}

func TestConfig_LoadCorrupted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "enrichment.json"), []byte("{invalid"), 0o644)
	_, err := LoadConfig(dir)
	if err == nil {
		t.Error("expected error for corrupted config")
	}
}
