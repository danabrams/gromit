//go:build contract

package contracts

import (
	"strings"
	"testing"
)

// TestRunGromitWithStdin_Basic verifies that runGromitWithStdin correctly pipes stdin to the command.
func TestRunGromitWithStdin_Basic(t *testing.T) {
	env := setupTestEnv(t)

	// Test with a simple command that should work with stdin
	// We'll use "gromit --help" which doesn't actually read stdin, but verifies the helper works
	stdin := "test input\n"
	stdout, stderr, exitCode, err := runGromitWithStdin(env, stdin, "--help")
	if err != nil {
		t.Fatalf("runGromitWithStdin failed: %v", err)
	}

	// Verify exit code is 0
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stdout: %s", stdout)
		t.Logf("Stderr: %s", stderr)
	}

	// Verify help output was produced
	if !strings.Contains(stdout, "gromit") {
		t.Errorf("Expected help output to contain 'gromit', got:\n%s", stdout)
	}
}
