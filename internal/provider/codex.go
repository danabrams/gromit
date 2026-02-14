package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	// providerNameCodex is the name identifier for the Codex provider
	providerNameCodex = "codex"
)

// CodexProvider wraps the Codex CLI and implements the Provider interface
type CodexProvider struct {
	binaryPath     string
	flags          []string
	promptDelivery string
	promptFlag     string
	tierToModel    map[string]string
}

// Compile-time check to verify CodexProvider implements Provider interface
var _ Provider = (*CodexProvider)(nil)

// NewCodexProvider creates a new CodexProvider with the given configuration
func NewCodexProvider(binaryPath string, flags []string, promptDelivery string, promptFlag string, tierToModel map[string]string) *CodexProvider {
	return &CodexProvider{
		binaryPath:     binaryPath,
		flags:          flags,
		promptDelivery: promptDelivery,
		promptFlag:     promptFlag,
		tierToModel:    tierToModel,
	}
}

// Name returns the provider name "codex"
func (cp *CodexProvider) Name() string {
	return providerNameCodex
}

// ModelForTier returns the model name for a given tier without invoking the LLM
func (cp *CodexProvider) ModelForTier(tier string) string {
	if modelName, ok := cp.tierToModel[tier]; ok {
		return modelName
	}
	return tier
}

// Run executes an LLM invocation with the given prompt and tier
func (cp *CodexProvider) Run(ctx context.Context, prompt string, tier string) (*Result, error) {
	if cp == nil {
		return nil, fmt.Errorf("codex provider is nil")
	}

	model := cp.ModelForTier(tier)
	tmpFile, cleanup, err := cp.createPromptFile(prompt)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	args := cp.buildCommandArgs(model, tmpFile)
	cmd := exec.CommandContext(ctx, cp.binaryPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime)

	if ctx.Err() != nil {
		return nil, fmt.Errorf("codex command cancelled: %w", ctx.Err())
	}

	output := stdout.String() + stderr.String()
	exitCode, err := cp.extractExitCode(err)
	if err != nil {
		return nil, err
	}

	return &Result{
		Success:  exitCode == 0,
		Output:   output,
		ExitCode: exitCode,
		Duration: duration,
		Model:    model,
	}, nil
}

// StreamRun executes an LLM invocation with streaming output.
// EventHandler and ToolCallHandler are no-ops for Codex (no stream-json format).
func (cp *CodexProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	if cp == nil {
		return nil, fmt.Errorf("codex provider is nil")
	}

	model := cp.ModelForTier(tier)
	tmpFile, cleanup, err := cp.createPromptFile(prompt)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	args := cp.buildCommandArgs(model, tmpFile)
	cmd := exec.CommandContext(ctx, cp.binaryPath, args...)

	// Stream output to both the provided writer and our capture buffer
	var captureBuffer bytes.Buffer
	var multiWriter io.Writer
	if output != nil {
		multiWriter = io.MultiWriter(output, &captureBuffer)
	} else {
		multiWriter = &captureBuffer
	}

	var stderr bytes.Buffer
	cmd.Stdout = multiWriter
	cmd.Stderr = &stderr

	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime)

	if ctx.Err() != nil {
		return nil, fmt.Errorf("codex command cancelled: %w", ctx.Err())
	}

	combinedOutput := captureBuffer.String() + stderr.String()
	exitCode, err := cp.extractExitCode(err)
	if err != nil {
		return nil, err
	}

	// EventHandler and ToolCallHandler are intentionally not called for Codex
	// as it doesn't produce Claude-style stream-json events

	return &Result{
		Success:  exitCode == 0,
		Output:   combinedOutput,
		ExitCode: exitCode,
		Duration: duration,
		Model:    model,
	}, nil
}

// RunValidation is not implemented for CodexProvider.
// Codex CLI does not support the structured validation prompt pattern used by Claude.
func (cp *CodexProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error) {
	return nil, fmt.Errorf("RunValidation is not implemented for Codex provider")
}

// IsUsageLimitError detects Codex-specific usage limit errors.
// Checks for output containing "usage limit", "rate limit", or "quota exceeded"
// (case-insensitive) with a non-success result.
func (cp *CodexProvider) IsUsageLimitError(result *Result, err error) bool {
	if result == nil {
		return false
	}

	// Must be a failure to be a usage limit error
	if result.Success {
		return false
	}

	// Check for usage limit keywords (case-insensitive)
	outputLower := strings.ToLower(result.Output)
	keywords := []string{"usage limit", "rate limit", "quota exceeded"}
	for _, keyword := range keywords {
		if strings.Contains(outputLower, keyword) {
			return true
		}
	}

	return false
}

// createPromptFile writes the prompt to a temporary file and returns the filename
// and a cleanup function. The cleanup function should be called via defer.
func (cp *CodexProvider) createPromptFile(prompt string) (string, func(), error) {
	tmpFile, err := os.CreateTemp("", "codex-prompt-*.txt")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file for prompt: %w", err)
	}

	cleanup := func() { os.Remove(tmpFile.Name()) }

	if _, err := tmpFile.WriteString(prompt); err != nil {
		tmpFile.Close()
		cleanup()
		return "", nil, fmt.Errorf("failed to write prompt to temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpFile.Name(), cleanup, nil
}

// buildCommandArgs constructs the command arguments for the Codex CLI invocation
func (cp *CodexProvider) buildCommandArgs(model, promptFile string) []string {
	args := make([]string, 0, len(cp.flags)+4)
	args = append(args, cp.flags...)
	args = append(args, "--model", model)
	args = append(args, cp.promptFlag, promptFile)
	return args
}

// extractExitCode extracts the exit code from a command execution error.
// Returns 0 for success, the exit code for ExitError, or an error if the command
// failed to start.
func (cp *CodexProvider) extractExitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), nil
	}

	return 0, fmt.Errorf("failed to execute codex command: %w", err)
}

// codexUsage represents token usage data from Codex turn.completed events
type codexUsage struct {
	InputTokens       int `json:"input_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
	OutputTokens      int `json:"output_tokens"`
}

// codexErrorInfo represents error information from Codex turn.completed events
type codexErrorInfo struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// codexItem represents an item from Codex item.started or item.completed events
type codexItem struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Command  string `json:"command"`
	Path     string `json:"path"`
	ToolName string `json:"tool_name"`
}

// codexEvent represents a top-level Codex JSONL event
type codexEvent struct {
	Type      string          `json:"type"`
	Item      *codexItem      `json:"item,omitempty"`
	Status    string          `json:"status,omitempty"`
	Usage     *codexUsage     `json:"usage,omitempty"`
	ErrorInfo *codexErrorInfo `json:"error,omitempty"`
}

// processCodexStream reads Codex JSONL events from reader, converts them to StreamEvent format,
// and calls handlers for each event. Returns the final result text (from last agent_message),
// token usage data (from turn.completed), and any error encountered.
func processCodexStream(reader io.Reader, output io.Writer, handler EventHandler, toolHandler ToolCallHandler) (string, *codexUsage, error) {
	scanner := bufio.NewScanner(reader)
	var lastAgentText string
	var usage *codexUsage

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event codexEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Skip malformed lines silently
			continue
		}

		// Handle different event types
		switch event.Type {
		case "thread.started":
			if handler != nil {
				streamEvent := map[string]interface{}{
					"type": "system",
				}
				eventJSON, _ := json.Marshal(streamEvent)
				handler(eventJSON)
			}
		}

		_ = lastAgentText
		_ = usage
		_ = toolHandler
		_ = output
	}

	if err := scanner.Err(); err != nil {
		return "", nil, err
	}

	return lastAgentText, usage, nil
}
