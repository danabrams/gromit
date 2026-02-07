package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var (
	// binaryPath is the path to the built gromit binary
	binaryPath string

	// update is a flag to regenerate golden files
	update = flag.Bool("update", false, "update golden files")
)

// TestMain builds the gromit binary once before running tests
func TestMain(m *testing.M) {
	// Parse flags
	flag.Parse()

	// Create a temporary directory for the test binary
	tmpDir, err := os.MkdirTemp("", "gromit-cli-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	// Build the gromit binary
	binaryPath = filepath.Join(tmpDir, "gromit")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build gromit binary: %v\n", err)
		os.Exit(1)
	}

	// Run the tests
	exitCode := m.Run()

	// Exit with the test result code
	os.Exit(exitCode)
}

// runGromit executes the gromit binary with the given arguments and returns stdout, stderr, and exit code
func runGromit(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)

	outBuf, outErr := cmd.Output()
	exitCode = 0

	if outErr != nil {
		if exitErr, ok := outErr.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("Failed to run gromit %v: %v", args, outErr)
		}
	}

	stdout = string(outBuf)

	return stdout, stderr, exitCode
}

// goldenPath returns the path to a golden file for the given command
func goldenPath(command string) string {
	return filepath.Join("testdata", "golden", fmt.Sprintf("%s.help.txt", command))
}

// TestCLIContractInfrastructure is a smoke test to verify the test infrastructure works
func TestCLIContractInfrastructure(t *testing.T) {
	// Verify binary path is set
	if binaryPath == "" {
		t.Fatal("binaryPath is empty - TestMain did not run")
	}

	// Verify binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("binary does not exist at %s: %v", binaryPath, err)
	}

	// Verify runGromit works
	stdout, stderr, exitCode := runGromit(t, "--help")
	if exitCode != 0 {
		t.Errorf("gromit --help exited with code %d, stderr: %s", exitCode, stderr)
	}
	if stdout == "" {
		t.Error("gromit --help produced no output")
	}

	// Verify goldenPath works
	path := goldenPath("test")
	expected := filepath.Join("testdata", "golden", "test.help.txt")
	if path != expected {
		t.Errorf("goldenPath(test) = %s, want %s", path, expected)
	}
}
