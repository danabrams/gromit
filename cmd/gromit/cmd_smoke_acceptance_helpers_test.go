package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupDebugAgentTestProject(t *testing.T, configContent string) string {
	t.Helper()

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("Rules"), 0644); err != nil {
		t.Fatalf("failed to write RULES.md: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("Project context"), 0644); err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write gromit.yaml: %v", err)
	}

	initGitRepo(t, tmpDir)

	return tmpDir
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGitCommand(t, dir, "init")
	runGitCommand(t, dir, "config", "user.name", "Test User")
	runGitCommand(t, dir, "config", "user.email", "test@example.com")
	runGitCommand(t, dir, "add", ".")
	runGitCommand(t, dir, "commit", "-m", "initial commit")
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func setupExploreAgentTestProject(t *testing.T, configContent string) string {
	t.Helper()

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("Rules"), 0644); err != nil {
		t.Fatalf("failed to write RULES.md: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("Project context"), 0644); err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write gromit.yaml: %v", err)
	}

	return tmpDir
}

func setupReviewSpecSmokeProject(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("Rules"), 0644); err != nil {
		t.Fatalf("write RULES.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("Context"), 0644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "gromit.yaml"), []byte("paths:\n  gromit_dir: .gromit\n"), 0644); err != nil {
		t.Fatalf("write gromit.yaml: %v", err)
	}

	specBody := "---\nid: existing-spec\n---\n# Existing Spec\n"
	if err := os.WriteFile(filepath.Join(specsDir, "existing-spec.md"), []byte(specBody), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	return tmpDir
}
