package testutil

import (
	"fmt"
	"os/exec"
	"strings"
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
	cmd := exec.Command(binary, args...)

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

	// Set up stdin pipe
	stdinPipe, pipeErr := cmd.StdinPipe()
	if pipeErr != nil {
		return "", "", -1, fmt.Errorf("creating stdin pipe: %w", pipeErr)
	}

	// Start the command
	if startErr := cmd.Start(); startErr != nil {
		return "", "", -1, fmt.Errorf("starting command: %w", startErr)
	}

	// Write stdin data
	if _, writeErr := stdinPipe.Write([]byte(stdin)); writeErr != nil {
		stdinPipe.Close()
		cmd.Wait()
		return "", "", -1, fmt.Errorf("writing to stdin: %w", writeErr)
	}
	stdinPipe.Close()

	// Wait for command to complete
	runErr := cmd.Wait()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if runErr != nil {
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
