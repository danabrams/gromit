//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
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

	return tmpDir
}

// Expected failure: debug command does not define debugAgent/debugChooseAgent flags yet,
// so help output does not include --agent or --choose-agent.
func TestDebugHelpIncludesAgentFlags(t *testing.T) {
	stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(
		binaryPath,
		"",
		nil,
		"",
		"debug", "--help",
	)
	if err != nil {
		t.Fatalf("failed to run gromit debug --help: %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", exitCode, stderr)
	}

	if !strings.Contains(stdout, "--agent") {
		t.Errorf("expected debug help to include --agent, got: %s", stdout)
	}

	if !strings.Contains(stdout, "--choose-agent") {
		t.Errorf("expected debug help to include --choose-agent, got: %s", stdout)
	}
}

// Expected failure: debug command does not accept --agent yet and still launches claude directly,
// so the custom agent override (debugAgent) is ignored.
func TestDebugAgentOverrideUsesAgentBinary(t *testing.T) {
	configContent := `
paths:
  gromit_dir: .gromit
claude:
  binary: "nonexistent-debug-claude"
agents:
  definitions:
    test-agent:
      binary: "echo"
      flags:
        - "--from-test"
`
	tmpDir := setupDebugAgentTestProject(t, configContent)

	stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(
		binaryPath,
		tmpDir,
		nil,
		"",
		"debug", "--agent", "test-agent", "login fails",
	)
	if err != nil {
		t.Fatalf("failed to run gromit debug with --agent: %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", exitCode, stderr)
	}

	if strings.Contains(stderr, "unknown flag") {
		t.Fatalf("unexpected flag error: %s", stderr)
	}

	if !strings.Contains(stdout, "--prompt") {
		t.Errorf("expected agent to receive --prompt arg, got: %s", stdout)
	}

	if !strings.Contains(stdout, "--from-test") {
		t.Errorf("expected agent flags to be passed through, got: %s", stdout)
	}
}

// Expected failure: debug command does not accept --choose-agent yet and does not prompt
// for agent selection, so debugChooseAgent behavior is missing.
func TestDebugChooseAgentUsesPicker(t *testing.T) {
	configContent := `
paths:
  gromit_dir: .gromit
claude:
  binary: "nonexistent-debug-claude"
agents:
  definitions:
    alpha:
      binary: "echo"
    beta:
      binary: "echo"
`
	tmpDir := setupDebugAgentTestProject(t, configContent)

	stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(
		binaryPath,
		tmpDir,
		nil,
		"1\n",
		"debug", "--choose-agent", "intermittent failure",
	)
	if err != nil {
		t.Fatalf("failed to run gromit debug with --choose-agent: %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", exitCode, stderr)
	}

	if strings.Contains(stderr, "unknown flag") {
		t.Fatalf("unexpected flag error: %s", stderr)
	}

	if !strings.Contains(stdout, "Select agent for debug:") {
		t.Errorf("expected picker prompt, got: %s", stdout)
	}

	if !strings.Contains(stdout, "1. alpha") {
		t.Errorf("expected picker to list alpha agent first, got: %s", stdout)
	}

	if !strings.Contains(stdout, "--prompt") {
		t.Errorf("expected selected agent to receive --prompt arg, got: %s", stdout)
	}
}
