//go:build acceptance

package main

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

// smoke-matrix: keep | rationale: Verifies critical end-to-end explore invocation with explicit agent override and prompt forwarding. | destination: cmd/gromit/explore_codex_help_acceptance_test.go:TestCmdSmoke_ExploreAgentSelectionEndToEnd
func TestCmdSmoke_ExploreAgentSelectionEndToEnd(t *testing.T) {
	configContent := "paths:\n  gromit_dir: .gromit\nclaude:\n  binary: \"nonexistent-claude\"\nagents:\n  definitions:\n    override-agent:\n      binary: \"echo\"\n      flags:\n        - \"--from-override\"\n"
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
