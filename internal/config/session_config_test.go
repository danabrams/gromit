package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	// Iterations default: 0 means unlimited, so no default to set
	// TestCommand default: empty string
	if cfg.Session.TestCommand != "" {
		t.Errorf("Session.TestCommand default = %q, want empty string", cfg.Session.TestCommand)
	}

	// MaxFixRetries default: 3
	if cfg.Session.MaxFixRetries != 3 {
		t.Errorf("Session.MaxFixRetries default = %d, want 3", cfg.Session.MaxFixRetries)
	}

	// FixTier default: "medium"
	if cfg.Session.FixTier != "medium" {
		t.Errorf("Session.FixTier default = %q, want %q", cfg.Session.FixTier, "medium")
	}

	// Review default: true
	if cfg.Session.Review == nil {
		t.Error("Session.Review default = nil, want non-nil (true)")
	} else if !*cfg.Session.Review {
		t.Errorf("Session.Review default = false, want true")
	}

	// Retro default: true
	if cfg.Session.Retro == nil {
		t.Error("Session.Retro default = nil, want non-nil (true)")
	} else if !*cfg.Session.Retro {
		t.Errorf("Session.Retro default = false, want true")
	}
}

func TestSessionConfigYAMLDeserialization(t *testing.T) {
	yamlContent := `
session:
  iterations: 10
  test_command: "go test ./..."
  max_fix_retries: 5
  fix_tier: "high"
  review: false
  retro: false
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Session.Iterations != 10 {
		t.Errorf("Session.Iterations = %d, want 10", cfg.Session.Iterations)
	}
	if cfg.Session.TestCommand != "go test ./..." {
		t.Errorf("Session.TestCommand = %q, want %q", cfg.Session.TestCommand, "go test ./...")
	}
	if cfg.Session.MaxFixRetries != 5 {
		t.Errorf("Session.MaxFixRetries = %d, want 5", cfg.Session.MaxFixRetries)
	}
	if cfg.Session.FixTier != "high" {
		t.Errorf("Session.FixTier = %q, want %q", cfg.Session.FixTier, "high")
	}
	if cfg.Session.Review == nil || *cfg.Session.Review {
		t.Errorf("Session.Review = %v, want false", cfg.Session.Review)
	}
	if cfg.Session.Retro == nil || *cfg.Session.Retro {
		t.Errorf("Session.Retro = %v, want false", cfg.Session.Retro)
	}
}

func TestGromitYamlDocumentsSessionConfig(t *testing.T) {
	content, err := os.ReadFile("../../gromit.yaml")
	if err != nil {
		t.Fatalf("failed to read gromit.yaml: %v", err)
	}

	text := string(content)

	if !strings.Contains(text, "# session:") {
		t.Error("gromit.yaml missing commented-out session: block")
	}
}
