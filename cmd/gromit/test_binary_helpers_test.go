package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

// binaryPath is the path to the built gromit test binary.
var binaryPath string

// TestMain builds the gromit binary once before running tests.
func TestMain(m *testing.M) {
	flag.Parse()

	tmpDir, err := os.MkdirTemp("", "gromit-cli-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	binaryPath = filepath.Join(tmpDir, "gromit")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build gromit binary: %v\n", err)
		_ = os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	exitCode := m.Run()
	_ = os.RemoveAll(tmpDir)
	os.Exit(exitCode)
}

// runGromit executes the gromit binary with the given arguments and returns
// stdout, stderr, and exit code.
func runGromit(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(binaryPath, "", nil, "", args...)
	if err != nil {
		t.Fatalf("Failed to run gromit %v: %v", args, err)
	}

	return stdout, stderr, exitCode
}

// runGromitWithStdin executes the gromit binary with the given arguments and stdin input.
func runGromitWithStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(binaryPath, "", nil, stdin, args...)
	if err != nil {
		t.Fatalf("Failed to run gromit %v with stdin: %v", args, err)
	}

	return stdout, stderr, exitCode
}
