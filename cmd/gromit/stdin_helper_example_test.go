package main

import (
	"strings"
	"testing"
)

// TestRunGromitWithStdinExample demonstrates usage of the runGromitWithStdin helper
func TestRunGromitWithStdinExample(t *testing.T) {
	t.Skip("Example test - demonstrates runGromitWithStdin usage")

	// Example: Test the refine command with "Something new" selection
	// The stdin input simulates user typing "3\n" to select the third option
	stdin := "3\n"
	stdout, stderr, exitCode := runGromitWithStdin(t, stdin, "refine")

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d\nstdout: %s\nstderr: %s", exitCode, stdout, stderr)
	}

	// Example assertions (would be customized for actual test)
	if !strings.Contains(stdout, "Select an idea") {
		t.Errorf("expected picker prompt in output")
	}
}

// TestRunGromitWithStdinMultipleInputs demonstrates multiple stdin inputs
func TestRunGromitWithStdinMultipleInputs(t *testing.T) {
	t.Skip("Example test - demonstrates multiple stdin inputs")

	// Simulate multiple user inputs: "y\n" followed by "some idea text\n"
	stdin := "y\nsome idea text\n"
	stdout, _, exitCode := runGromitWithStdin(t, stdin, "some-command")

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	// Verify the command processed both inputs
	_ = stdout // Use stdout in actual test assertions
}
