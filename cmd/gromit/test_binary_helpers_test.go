package main

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

// binaryPath is the path to the built gromit test binary.
var binaryPath string

// TestMain builds the gromit binary once before running tests.
func TestMain(m *testing.M) {
	flag.Parse()

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to determine test binary path: %v\n", err)
		os.Exit(1)
	}

	binaryPath = exe

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestGromitHelperProcess(t *testing.T) {
	if os.Getenv("GROMIT_TEST_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			rootCmd.SetArgs(args[i+1:])
			if err := rootCmd.Execute(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	fmt.Fprintln(os.Stderr, "missing helper args separator")
	os.Exit(2)
}

// runGromit executes the gromit binary with the given arguments and returns
// stdout, stderr, and exit code.
func runGromit(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	helperArgs := append([]string{"-test.run=TestGromitHelperProcess", "--"}, args...)
	environ := append(os.Environ(), "GROMIT_TEST_HELPER_PROCESS=1")
	stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(binaryPath, "", environ, "", helperArgs...)
	if err != nil {
		t.Fatalf("Failed to run gromit %v: %v", args, err)
	}

	return stdout, stderr, exitCode
}

// runGromitWithStdin executes the gromit binary with the given arguments and stdin input.
func runGromitWithStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	helperArgs := append([]string{"-test.run=TestGromitHelperProcess", "--"}, args...)
	environ := append(os.Environ(), "GROMIT_TEST_HELPER_PROCESS=1")
	stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(binaryPath, "", environ, stdin, helperArgs...)
	if err != nil {
		t.Fatalf("Failed to run gromit %v with stdin: %v", args, err)
	}

	return stdout, stderr, exitCode
}
