package benchmark

import (
	"os"
	"path/filepath"
	stdstrings "strings"
	"testing"
)

func TestLoadManifest_ParsesValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := `id: tdd-vs-single-pass
base_commit: abc123
beads:
  - gromit-1
modes:
  - single_pass
provider: openai
model_family: gpt-5
low_tier_model: gpt-5-mini
medium_tier_model: gpt-5.3-codex
high_tier_model: gpt-5.3-codex
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	if manifest.ID != "tdd-vs-single-pass" {
		t.Fatalf("manifest id = %q, want %q", manifest.ID, "tdd-vs-single-pass")
	}
	if len(manifest.Beads) != 1 || manifest.Beads[0] != "gromit-1" {
		t.Fatalf("manifest beads = %v, want [gromit-1]", manifest.Beads)
	}
	if len(manifest.Modes) != 1 || manifest.Modes[0] != "single_pass" {
		t.Fatalf("manifest modes = %v, want [single_pass]", manifest.Modes)
	}
}

func TestLoadManifest_RequiresID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := `base_commit: abc123
beads:
  - gromit-1
modes:
  - single_pass
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want error")
	}
	if !stdstrings.Contains(err.Error(), "id is required") {
		t.Fatalf("LoadManifest() error = %q, want contains %q", err.Error(), "id is required")
	}
}

func TestLoadManifest_RejectsUnsupportedMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := `id: tdd-vs-single-pass
base_commit: abc123
beads:
  - gromit-1
modes:
  - unsupported_mode
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want error")
	}
	if !stdstrings.Contains(err.Error(), "unsupported mode") {
		t.Fatalf("LoadManifest() error = %q, want contains %q", err.Error(), "unsupported mode")
	}
}

func TestLoadManifest_RequiresBeads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := `id: tdd-vs-single-pass
base_commit: abc123
modes:
  - single_pass
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want error")
	}
	if !stdstrings.Contains(err.Error(), "beads is required") {
		t.Fatalf("LoadManifest() error = %q, want contains %q", err.Error(), "beads is required")
	}
}

func TestLoadManifest_RequiresModes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := `id: tdd-vs-single-pass
base_commit: abc123
beads:
  - gromit-1
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want error")
	}
	if !stdstrings.Contains(err.Error(), "modes is required") {
		t.Fatalf("LoadManifest() error = %q, want contains %q", err.Error(), "modes is required")
	}
}
