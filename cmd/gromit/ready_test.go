package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadyCmd_ListsEligibleSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	writeSpec(t, specsDir, "dep", true, nil)
	writeSpec(t, specsDir, "ready-child", false, []string{"dep"})

	writeConfig(t, tmpDir, specsDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to tmp: %v", err)
	}

	stdout, stderr, exitCode := runGromitCobra(t, "ready")
	if exitCode != 0 {
		t.Fatalf("ready command exit = %d, want 0 (stderr: %s)", exitCode, stderr)
	}
	if stderr != "" {
		t.Fatalf("ready command stderr = %q, want empty", stderr)
	}

	want := fmt.Sprintf("ready-child\t%s\n", filepath.Join(specsDir, "ready-child.md"))
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}

	writeSpec(t, specsDir, "ready-child", true, []string{"dep"})

	stdout, stderr, exitCode = runGromitCobra(t, "ready")
	if exitCode != 0 {
		t.Fatalf("second ready exit = %d, want 0 (stderr: %s)", exitCode, stderr)
	}
	if stderr != "" {
		t.Fatalf("second ready stderr = %q, want empty", stderr)
	}
	if stdout != "" {
		t.Fatalf("second ready stdout = %q, want empty", stdout)
	}
}

func writeSpec(t *testing.T, specsDir, id string, accepted bool, deps []string) {
	t.Helper()

	var builder strings.Builder
	fmt.Fprintf(&builder, "id: %s\n", id)
	if accepted {
		builder.WriteString("accepted: true\n")
	}
	if len(deps) > 0 {
		builder.WriteString("dependencies:\n")
		for _, dep := range deps {
			fmt.Fprintf(&builder, "  - %s\n", dep)
		}
	}

	content := fmt.Sprintf("---\n%s---\n# %s spec\n", builder.String(), id)
	path := filepath.Join(specsDir, id+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write spec %s: %v", id, err)
	}
}

func writeConfig(t *testing.T, dir, specsDir string) {
	t.Helper()

	configPath := filepath.Join(dir, "gromit.yaml")
	content := fmt.Sprintf("paths:\n  specs: %s\n", specsDir)
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
}
