//go:build acceptance

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/test/testutil"
)

// smoke-matrix: keep | rationale: Covers critical end-to-end agent override wiring from CLI flag through process launch. | destination: cmd/gromit/debug_agent_acceptance_test.go:TestCmdSmoke_DebugAgentResolutionEndToEnd
func TestCmdSmoke_DebugAgentResolutionEndToEnd(t *testing.T) {
	configContent := "paths:\n  gromit_dir: .gromit\nclaude:\n  binary: \"nonexistent-debug-claude\"\nagents:\n  definitions:\n    test-agent:\n      binary: \"echo\"\n      flags:\n        - \"--from-test\"\n"
	tmpDir := setupDebugAgentTestProject(t, configContent)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
	stdout, stderr, exitCode, err := testutil.RunGromitHelperProcessWithStdin(
		ctx,
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
	if strings.Contains(stderr, "unknown flag") { t.Fatalf("unexpected flag error: %s", stderr) }
	if !strings.Contains(stdout, "--prompt") {
		t.Errorf("expected agent to receive --prompt arg, got: %s", stdout)
	}
	if !strings.Contains(stdout, "--from-test") {
		t.Errorf("expected agent flags to be passed through, got: %s", stdout)
	}
}
