//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

func runExploreHelp(t *testing.T) string {
	t.Helper()

	stdout, stderr, exitCode := runGromit(t, "explore", "--help")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", exitCode, stderr)
	}

	if strings.Contains(stderr, "unknown flag") {
		t.Fatalf("unexpected flag error: %s", stderr)
	}

	return stdout
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

// smoke-matrix: keep | rationale: Verifies critical end-to-end explore invocation with explicit agent override and prompt forwarding. | destination: cmd/gromit/explore_codex_help_acceptance_test.go:TestCmdSmoke_ExploreAgentSelectionEndToEnd
func TestCmdSmoke_ExploreAgentSelectionEndToEnd(t *testing.T) {
	// Expected failure: exploreAgentOverrideFlag constant does not exist yet and explore does not honor --agent override
	configContent := `
paths:
  gromit_dir: .gromit
claude:
  binary: "nonexistent-claude"
agents:
  definitions:
    override-agent:
      binary: "echo"
      flags:
        - "--from-override"
`
	tmpDir := setupExploreAgentTestProject(t, configContent)

	stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(
		binaryPath,
		tmpDir,
		nil,
		"",
		"explore", "--agent", "override-agent", "Codex override test",
	)
	if err != nil {
		t.Fatalf("failed to run gromit explore with --agent: %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", exitCode, stderr)
	}

	if strings.Contains(stderr, "unknown flag") {
		t.Fatalf("unexpected flag error: %s", stderr)
	}

	expectedFlag := exploreAgentOverrideFlag
	if !strings.Contains(stdout, expectedFlag) {
		t.Fatalf("expected explore output to include override flag %q, got: %s", expectedFlag, stdout)
	}

	if !strings.Contains(stdout, "--prompt") {
		t.Fatalf("expected explore output to include --prompt, got: %s", stdout)
	}
}

// smoke-matrix: move | rationale: Explore phase-configured agent selection is deterministic command behavior better covered in focused unit tests. | destination: cmd/gromit/explore_agent_test.go:TestExplorePhaseConfigSelectsAgent_Reclassified
func TestExplorePhaseConfigSelectsAgent(t *testing.T) {
	// Expected failure: explorePhaseConfigFlag constant does not exist yet and explore ignores agents.phases.explore
	configContent := `
paths:
  gromit_dir: .gromit
claude:
  binary: "nonexistent-claude"
agents:
  definitions:
    phase-agent:
      binary: "echo"
      flags:
        - "--from-phase-config"
  phases:
    explore: phase-agent
`
	tmpDir := setupExploreAgentTestProject(t, configContent)

	stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(
		binaryPath,
		tmpDir,
		nil,
		"",
		"explore", "Phase config test",
	)
	if err != nil {
		t.Fatalf("failed to run gromit explore with phase config: %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", exitCode, stderr)
	}

	if strings.Contains(stderr, "unknown flag") {
		t.Fatalf("unexpected flag error: %s", stderr)
	}

	expectedFlag := explorePhaseConfigFlag
	if !strings.Contains(stdout, expectedFlag) {
		t.Fatalf("expected explore output to include phase config flag %q, got: %s", expectedFlag, stdout)
	}

	if !strings.Contains(stdout, "--prompt") {
		t.Fatalf("expected explore output to include --prompt, got: %s", stdout)
	}
}
