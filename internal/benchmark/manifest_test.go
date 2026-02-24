package benchmark

import (
	"os"
	"path/filepath"
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
