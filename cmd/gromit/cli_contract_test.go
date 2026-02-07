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

// TestCLIContract_HelpText verifies help output for all commands matches golden files
func TestCLIContract_HelpText(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"root", []string{"--help"}},
		{"run", []string{"run", "--help"}},
		{"init", []string{"init", "--help"}},
		{"status", []string{"status", "--help"}},
		{"retro", []string{"retro", "--help"}},
		{"add", []string{"add", "--help"}},
		{"backlog", []string{"backlog", "--help"}},
		{"backlog-delete", []string{"backlog", "delete", "--help"}},
		{"board", []string{"board", "--help"}},
		{"queue", []string{"queue", "--help"}},
		{"triage", []string{"triage", "--help"}},
		{"refine", []string{"refine", "--help"}},
		{"plan", []string{"plan", "--help"}},
		{"review", []string{"review", "--help"}},
		{"decompose", []string{"decompose", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := runGromit(t, tt.args...)

			// Help commands should exit with code 0
			if exitCode != 0 {
				t.Errorf("gromit %v exited with code %d, stderr: %s", tt.args, exitCode, stderr)
			}

			// Get golden file path
			golden := goldenPath(tt.name)

			// If -update flag is set, write the golden file
			if *update {
				dir := filepath.Dir(golden)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("failed to create golden directory: %v", err)
				}
				if err := os.WriteFile(golden, []byte(stdout), 0644); err != nil {
					t.Fatalf("failed to write golden file: %v", err)
				}
				t.Logf("Updated golden file: %s", golden)
				return
			}

			// Read golden file
			expected, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("failed to read golden file %s: %v\nRun with -update flag to create it", golden, err)
			}

			// Compare output
			if stdout != string(expected) {
				t.Errorf("help output mismatch for %s\nRun with -update flag to update golden file\n\nGot:\n%s\n\nExpected:\n%s",
					tt.name, stdout, string(expected))
			}
		})
	}
}
