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
  - gromit-2
  - gromit-3
  - gromit-4
  - gromit-5
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
	if len(manifest.Beads) != 5 || manifest.Beads[0] != "gromit-1" || manifest.Beads[4] != "gromit-5" {
		t.Fatalf("manifest beads = %v, want [gromit-1 gromit-2 gromit-3 gromit-4 gromit-5]", manifest.Beads)
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
  - gromit-2
  - gromit-3
  - gromit-4
  - gromit-5
modes:
  - unsupported_mode
provider: openai
model_family: gpt-5
low_tier_model: gpt-5-mini
medium_tier_model: gpt-5.3-codex
high_tier_model: gpt-5.3-codex
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
  - gromit-2
  - gromit-3
  - gromit-4
  - gromit-5
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

func TestLoadManifest_RequiresProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := `id: tdd-vs-single-pass
base_commit: abc123
beads:
  - gromit-1
  - gromit-2
  - gromit-3
  - gromit-4
  - gromit-5
modes:
  - single_pass
model_family: gpt-5
low_tier_model: gpt-5-mini
medium_tier_model: gpt-5.3-codex
high_tier_model: gpt-5.3-codex
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want error")
	}
	if !stdstrings.Contains(err.Error(), "provider is required") {
		t.Fatalf("LoadManifest() error = %q, want contains %q", err.Error(), "provider is required")
	}
}

func TestLoadManifest_RequiresHighTierModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := `id: tdd-vs-single-pass
base_commit: abc123
beads:
  - gromit-1
  - gromit-2
  - gromit-3
  - gromit-4
  - gromit-5
modes:
  - single_pass
provider: openai
model_family: gpt-5
low_tier_model: gpt-5-mini
medium_tier_model: gpt-5.3-codex
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("LoadManifest() error = nil, want error")
	}
	if !stdstrings.Contains(err.Error(), "high_tier_model is required") {
		t.Fatalf("LoadManifest() error = %q, want contains %q", err.Error(), "high_tier_model is required")
	}
}

func TestLoadManifest_LoadsSampleManifestFile(t *testing.T) {
	path := filepath.Join("..", "..", ".gromit", "benchmarks", "tdd-vs-single-pass.yaml")

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if manifest.ID != "tdd-vs-single-pass" {
		t.Fatalf("manifest id = %q, want %q", manifest.ID, "tdd-vs-single-pass")
	}
}

func TestValidateManifest_AcceptsValidManifest(t *testing.T) {
	manifest := Manifest{
		ID:         "tdd-vs-single-pass",
		BaseCommit: "abc123",
		Beads:      []string{"gromit-1", "gromit-2", "gromit-3", "gromit-4", "gromit-5"},
		ModeConfig: ModeConfig{
			Modes: []string{"single_pass"},
		},
		ModelPinning: ModelPinning{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
	}

	if err := ValidateManifest(manifest); err != nil {
		t.Fatalf("ValidateManifest() error = %v", err)
	}
}

func TestValidateManifest_RequiresExactlyFiveBeads(t *testing.T) {
	manifest := Manifest{
		ID:         "tdd-vs-single-pass",
		BaseCommit: "abc123",
		Beads:      []string{"gromit-1", "gromit-2", "gromit-3", "gromit-4"},
		ModeConfig: ModeConfig{
			Modes: []string{"single_pass"},
		},
		ModelPinning: ModelPinning{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
	}

	if err := ValidateManifest(manifest); err == nil {
		t.Fatal("ValidateManifest() error = nil, want exact-size bead cohort error")
	}
}

func TestValidateManifest_RejectsDuplicateModes(t *testing.T) {
	manifest := Manifest{
		ID:         "tdd-vs-single-pass",
		BaseCommit: "abc123",
		Beads:      []string{"gromit-1", "gromit-2", "gromit-3", "gromit-4", "gromit-5"},
		ModeConfig: ModeConfig{
			Modes: []string{"single_pass", "single_pass"},
		},
		ModelPinning: ModelPinning{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
	}

	err := ValidateManifest(manifest)
	if err == nil {
		t.Fatal("ValidateManifest() error = nil, want duplicate mode error")
	}
	if !stdstrings.Contains(err.Error(), "duplicate mode") {
		t.Fatalf("ValidateManifest() error = %q, want contains %q", err.Error(), "duplicate mode")
	}
}

func TestManifest_TypedModeAndModelPinningModelsExist(t *testing.T) {
	var _ ModeConfig
	var _ ModelPinning
}
