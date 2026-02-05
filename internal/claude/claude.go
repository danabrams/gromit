package claude

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Result represents the outcome of a Claude invocation
type Result struct {
	Success  bool
	Output   string
	ExitCode int
	Duration time.Duration
	Model    string
}

// Client wraps the Claude CLI
type Client struct {
	binary  string
	flags   []string
	timeout time.Duration
}

// NewClient creates a new Claude CLI client
func NewClient(binary string, flags []string, timeoutSecs int) *Client {
	return &Client{
		binary:  binary,
		flags:   flags,
		timeout: time.Duration(timeoutSecs) * time.Second,
	}
}

// Run invokes Claude with the given prompt and model
// Each invocation is a fresh process - no context carried over
func (c *Client) Run(ctx context.Context, prompt string, model string) (*Result, error) {
	start := time.Now()

	// Build command args
	args := []string{
		"-p", // Print mode - non-interactive
		"--model", model,
	}
	args = append(args, c.flags...)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binary, args...)

	// Pipe prompt to stdin
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	// Capture stdout and stderr
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting claude: %w", err)
	}

	// Write prompt to stdin
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, prompt)
	}()

	// Wait for completion
	err = cmd.Wait()
	duration := time.Since(start)

	result := &Result{
		Output:   stdout.String(),
		Duration: duration,
		Model:    model,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Success = false
			// Include stderr in output for debugging
			if stderr.Len() > 0 {
				result.Output += "\n\nSTDERR:\n" + stderr.String()
			}
			return result, nil // Not an error - just a failed run
		}
		return nil, fmt.Errorf("running claude: %w", err)
	}

	result.Success = true
	result.ExitCode = 0
	return result, nil
}

// RunValidation runs validation commands using Claude with haiku
func (c *Client) RunValidation(ctx context.Context, commands []string, model string, workDir string) (*Result, error) {
	// Build a prompt that asks Claude to run the validation commands
	prompt := fmt.Sprintf(`You are running validation checks. Execute the following commands in order and report results.

Working directory: %s

Commands to run:
%s

Execute each command. If any command fails, report the failure clearly.
After all commands complete successfully, output: VALIDATION_PASSED

If any command fails, output: VALIDATION_FAILED followed by the error details.
`, workDir, strings.Join(commands, "\n"))

	return c.Run(ctx, prompt, model)
}

// IsValidationPassed checks if the result indicates validation passed
func IsValidationPassed(result *Result) bool {
	return result.Success && strings.Contains(result.Output, "VALIDATION_PASSED")
}

// StreamRun invokes Claude and streams output to the provided writer
func (c *Client) StreamRun(ctx context.Context, prompt string, model string, output io.Writer) (*Result, error) {
	start := time.Now()

	args := []string{
		"-p",
		"--model", model,
	}
	args = append(args, c.flags...)

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binary, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	// Stream to both output writer and capture buffer
	var captured strings.Builder
	cmd.Stdout = io.MultiWriter(output, &captured)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting claude: %w", err)
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, prompt)
	}()

	err = cmd.Wait()
	duration := time.Since(start)

	result := &Result{
		Output:   captured.String(),
		Duration: duration,
		Model:    model,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Success = false
			return result, nil
		}
		return nil, fmt.Errorf("running claude: %w", err)
	}

	result.Success = true
	result.ExitCode = 0
	return result, nil
}
