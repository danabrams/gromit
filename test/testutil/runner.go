package testutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// RunGromitWithStdin executes a gromit command with stdin input.
// Parameters:
//   - binary: path to the gromit binary
//   - dir: working directory (if empty, cmd.Dir is not set)
//   - environ: environment variables (if nil, inherits from parent)
//   - stdin: string to write to stdin
//   - args: command-line arguments
//
// Returns stdout, stderr, exitCode, and error (error is only non-nil for execution failures,
// not for non-zero exit codes).
func RunGromitWithStdin(binary, dir string, environ []string, stdin string, args ...string) (stdout, stderr string, exitCode int, err error) {
	return runWithStdin(context.Background(), binary, dir, environ, stdin, args...)
}

// RunGromitHelperProcessWithStdin executes the gromit test binary through the
// TestGromitHelperProcess entrypoint with timeout/cancellation support.
func RunGromitHelperProcessWithStdin(ctx context.Context, binary, dir string, environ []string, stdin string, args ...string) (stdout, stderr string, exitCode int, err error) {
	helperArgs := append([]string{"-test.run=TestGromitHelperProcess", "--"}, args...)
	helperEnv := append([]string(nil), environ...)
	if helperEnv == nil {
		helperEnv = os.Environ()
	}
	helperEnv = append(helperEnv, "GROMIT_TEST_HELPER_PROCESS=1")
	return runWithStdin(ctx, binary, dir, helperEnv, stdin, helperArgs...)
}

func runWithStdin(ctx context.Context, binary, dir string, environ []string, stdin string, args ...string) (stdout, stderr string, exitCode int, err error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	setProcessGroupCancellation(cmd)

	// Only set Dir if non-empty
	if dir != "" {
		cmd.Dir = dir
	}

	// Only set Env if non-nil
	if environ != nil {
		cmd.Env = environ
	}

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Stdin = strings.NewReader(stdin)

	// Run the command
	runErr := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if runErr != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("command canceled: %w", ctx.Err())
			exitCode = -1
			return stdout, stderr, exitCode, err
		}
		// Check if it's an ExitError (non-zero exit code)
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil // Not an error - just a non-zero exit code
		} else {
			// Some other error (e.g., command not found)
			err = runErr
			exitCode = -1
		}
	} else {
		exitCode = 0
	}

	return stdout, stderr, exitCode, err
}

func setProcessGroupCancellation(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Negative PID targets the process group so child processes are terminated too.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// ReplaceOrAppend replaces an environment variable in the env slice,
// or appends it if it doesn't exist.
func ReplaceOrAppend(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// RemoveEnvVar removes an environment variable from the env slice.
func RemoveEnvVar(env []string, key string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}

// FindRealGit searches for the real git binary in common locations.
// It checks PATH first, then falls back to standard locations.
func FindRealGit() string {
	// First, try to find git in the current PATH
	gitPath, err := exec.LookPath("git")
	if err == nil && gitPath != "" {
		return gitPath
	}

	// Fall back to common locations
	commonPaths := []string{
		"/usr/bin/git",
		"/usr/local/bin/git",
		"/opt/homebrew/bin/git",
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// CleanupClaudeFailOnceStateFiles removes any claude fail-once state files from a directory.
// These files are created by CLAUDE_FAIL_<MODEL>_ONCE mode to track first-time failures
// and should be cleaned up to prevent state leaks between test runs.
func CleanupClaudeFailOnceStateFiles(testDir string) {
	models := []string{"haiku", "sonnet", "opus"}
	for _, model := range models {
		stateFile := filepath.Join(testDir, ".claude_fail_"+model+"_once_state")
		// Ignore errors - file may not exist, which is fine
		os.Remove(stateFile)
	}
}
