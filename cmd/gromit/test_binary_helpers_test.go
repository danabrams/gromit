package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

func TestRunGromitCobra_HelpOutputsUsage(t *testing.T) {
	stdout, stderr, exitCode := runGromitCobra(t, "--help")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", exitCode, stderr)
	}

	if !strings.Contains(stdout, "Gromit") {
		t.Errorf("expected help output to include Gromit, got: %s", stdout)
	}
}

func TestRunCommandHelpIncludesGracefulStopHint(t *testing.T) {
	stdout, stderr, exitCode := runGromitCobra(t, "run", "--help")

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", exitCode, stderr)
	}

	const hint = "Press Ctrl+C once to stop after the current iteration"
	if !strings.Contains(stdout, hint) {
		t.Fatalf("expected run help hint %q, got: %s", hint, stdout)
	}
}

func TestRunGromitCobra_ResetsHelpFlag(t *testing.T) {
	_, _, _ = runGromitCobra(t, "--help")

	helpFlag := rootCmd.Flags().Lookup("help")
	if helpFlag == nil {
		t.Fatal("expected help flag to exist on root command")
	}

	if helpFlag.Value.String() != "false" {
		t.Fatalf("expected help flag to be reset, got: %s", helpFlag.Value.String())
	}
}

// runGromitCobra executes the cobra command directly and returns stdout, stderr, and exit code.
func runGromitCobra(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	prevStdout := os.Stdout
	prevStderr := os.Stderr
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = prevStdout
		os.Stderr = prevStderr
	}()

	prevOut := rootCmd.OutOrStdout()
	prevErr := rootCmd.ErrOrStderr()
	rootCmd.SetOut(stdoutWriter)
	rootCmd.SetErr(stderrWriter)
	defer func() {
		rootCmd.SetOut(prevOut)
		rootCmd.SetErr(prevErr)
	}()

	stdoutDone := make(chan error, 1)
	stderrDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&stdoutBuf, stdoutReader)
		stdoutDone <- copyErr
	}()
	go func() {
		_, copyErr := io.Copy(&stderrBuf, stderrReader)
		stderrDone <- copyErr
	}()

	resetCommandFlags(rootCmd)
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(stderrWriter, err)
		exitCode = 1
	}
	resetCommandFlags(rootCmd)

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	_ = <-stdoutDone
	_ = <-stderrDone
	_ = stdoutReader.Close()
	_ = stderrReader.Close()

	return stdoutBuf.String(), stderrBuf.String(), exitCode
}

func resetCommandFlags(cmd *cobra.Command) {
	resetFlagSet(cmd.Flags())
	resetFlagSet(cmd.PersistentFlags())
	for _, sub := range cmd.Commands() {
		resetCommandFlags(sub)
	}
}

func resetFlagSet(set *pflag.FlagSet) {
	set.VisitAll(func(flag *pflag.Flag) {
		_ = flag.Value.Set(flag.DefValue)
	})
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
