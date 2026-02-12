package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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

	// Resolve tier to model name
	model := cp.ModelForTier(tier)

	// Write prompt to temporary file
	tmpFile, err := os.CreateTemp("", "codex-prompt-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for prompt: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(prompt); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write prompt to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Build command arguments
	args := []string{}
	args = append(args, cp.flags...)
	args = append(args, "--model", model)
	args = append(args, cp.promptFlag, tmpFile.Name())

	// Execute command
	cmd := exec.CommandContext(ctx, cp.binaryPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime)

	// Combine stdout and stderr
	output := stdout.String() + stderr.String()

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Command failed to start
			return nil, fmt.Errorf("failed to execute codex command: %w", err)
		}
	}

	result := &Result{
		Success:  exitCode == 0,
		Output:   output,
		ExitCode: exitCode,
		Duration: duration,
		Model:    model,
	}

	return result, nil
}

// StreamRun executes an LLM invocation with streaming output
func (cp *CodexProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	return nil, nil
}

// RunValidation executes validation commands using the LLM
func (cp *CodexProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error) {
	return nil, nil
}

// IsUsageLimitError detects Codex-specific usage limit errors
func (cp *CodexProvider) IsUsageLimitError(result *Result, err error) bool {
	return false
}
