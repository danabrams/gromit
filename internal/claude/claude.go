package claude

import (
	"bufio"
	"context"
	"encoding/json"
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

// EventHandler is called for each line of stream-json output from Claude CLI.
// The raw JSON line is passed for external parsing and logging.
type EventHandler func(line []byte)

// StreamRun invokes Claude and streams output to the provided writer.
// If an EventHandler is provided, it uses --output-format stream-json --verbose
// to get structured events for firehose logging, and extracts the text result
// from the JSON stream. Otherwise it streams raw text output.
func (c *Client) StreamRun(ctx context.Context, prompt string, model string, output io.Writer, handler EventHandler) (*Result, error) {
	start := time.Now()

	args := []string{
		"-p",
		"--model", model,
	}

	useStreamJSON := handler != nil
	if useStreamJSON {
		args = append(args, "--output-format", "stream-json", "--verbose")
	}

	args = append(args, c.flags...)

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.binary, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting claude: %w", err)
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, prompt)
	}()

	// Read and process stdout
	var resultText string
	if useStreamJSON {
		resultText = c.processStreamJSON(stdout, output, handler)
	} else {
		var captured strings.Builder
		io.Copy(io.MultiWriter(output, &captured), stdout)
		resultText = captured.String()
	}

	err = cmd.Wait()
	duration := time.Since(start)

	result := &Result{
		Output:   resultText,
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

// processStreamJSON reads stream-json lines, calls the handler for each,
// and extracts the final result text.
func (c *Client) processStreamJSON(stdout io.Reader, output io.Writer, handler EventHandler) string {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for large events

	var resultText string

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Call the handler for firehose logging
		lineCopy := make([]byte, len(line))
		copy(lineCopy, line)
		handler(lineCopy)

		// Extract text content from assistant messages for terminal output
		var event struct {
			Type    string `json:"type"`
			Subtype string `json:"subtype,omitempty"`
			Message *struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text,omitempty"`
				} `json:"content"`
			} `json:"message,omitempty"`
			Result string `json:"result,omitempty"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		// Stream assistant text to terminal output
		if event.Type == "assistant" && event.Message != nil {
			for _, block := range event.Message.Content {
				if block.Type == "text" && block.Text != "" {
					fmt.Fprint(output, block.Text)
				}
			}
		}

		// Capture the final result text
		if event.Type == "result" {
			resultText = event.Result
		}
	}

	return resultText
}
