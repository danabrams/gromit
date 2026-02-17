package provider

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/danabrams/gromit/internal/claude"
)

const (
	// providerNameClaude is the name identifier for the Claude provider
	providerNameClaude = "claude"
)

// claudeClient is an interface for the claude.Client methods used by ClaudeProvider.
// This allows for easier testing with mock implementations.
type claudeClient interface {
	Run(ctx context.Context, prompt string, model string) (*claude.Result, error)
	StreamRun(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error)
	RunValidation(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error)
}

// ClaudeProvider wraps the Claude CLI client and implements the Provider interface
type ClaudeProvider struct {
	client      claudeClient
	tierToModel map[string]string
}

// Compile-time check to verify ClaudeProvider implements Provider interface
var _ Provider = (*ClaudeProvider)(nil)

// NewClaudeProvider creates a new ClaudeProvider with the given client and tier mapping
func NewClaudeProvider(client claudeClient, tierToModel map[string]string) *ClaudeProvider {
	return &ClaudeProvider{
		client:      client,
		tierToModel: tierToModel,
	}
}

// Name returns the provider name "claude"
func (cp *ClaudeProvider) Name() string {
	return providerNameClaude
}

// validateProvider checks that the provider and its client are not nil
func (cp *ClaudeProvider) validateProvider() error {
	if cp == nil {
		return fmt.Errorf("claude provider is nil")
	}
	if cp.client == nil {
		return fmt.Errorf("claude client is nil")
	}
	return nil
}

// resolveTier maps an abstract tier to a concrete model name
func (cp *ClaudeProvider) resolveTier(tier string) string {
	if modelName, ok := cp.tierToModel[tier]; ok {
		return modelName
	}
	return tier
}

// ModelForTier returns the model name for a given tier without invoking the LLM.
func (cp *ClaudeProvider) ModelForTier(tier string) string {
	return cp.resolveTier(tier)
}

// convertResult converts a claude.Result to a provider.Result
func convertResult(claudeResult *claude.Result) *Result {
	return &Result{
		Success:      claudeResult.Success,
		Output:       claudeResult.Output,
		Stdout:       claudeResult.Output,
		ExitCode:     claudeResult.ExitCode,
		Duration:     claudeResult.Duration,
		Model:        claudeResult.Model,
		CostUSD:      claudeResult.CostUSD,
		InputTokens:  claudeResult.InputTokens,
		OutputTokens: claudeResult.OutputTokens,
	}
}

// convertToClaudeResult converts a provider.Result to a claude.Result
func convertToClaudeResult(result *Result) *claude.Result {
	return &claude.Result{
		Success:  result.Success,
		Output:   result.Output,
		ExitCode: result.ExitCode,
		Duration: result.Duration,
		Model:    result.Model,
	}
}

// Run executes an LLM invocation with the given prompt and tier.
// It resolves the tier to a model name and delegates to claude.Client.Run().
func (cp *ClaudeProvider) Run(ctx context.Context, prompt string, tier string) (*Result, error) {
	if err := cp.validateProvider(); err != nil {
		return nil, err
	}

	modelName := cp.resolveTier(tier)
	claudeResult, err := cp.client.Run(ctx, prompt, modelName)
	if err != nil {
		return nil, err
	}

	return convertResult(claudeResult), nil
}

// StreamRun executes an LLM invocation with streaming output.
// It resolves the tier to a model name and delegates to claude.Client.StreamRun().
func (cp *ClaudeProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	if err := cp.validateProvider(); err != nil {
		return nil, err
	}

	modelName := cp.resolveTier(tier)

	// Convert provider handlers to claude handlers
	var claudeHandler claude.EventHandler
	if handler != nil {
		claudeHandler = claude.EventHandler(handler)
	}

	var claudeToolHandler claude.ToolCallHandler
	if onToolCall != nil {
		claudeToolHandler = func(event claude.ToolEvent) {
			onToolCall(ToolEvent{
				ToolName:  event.ToolName,
				FilePath:  event.FilePath,
				Timestamp: event.Timestamp,
			})
		}
	}

	claudeResult, err := cp.client.StreamRun(ctx, prompt, modelName, output, claudeHandler, claudeToolHandler)
	if err != nil {
		return nil, err
	}

	return convertResult(claudeResult), nil
}

// RunValidation executes validation commands using the LLM.
// It resolves the tier to a model name and delegates to claude.Client.RunValidation().
func (cp *ClaudeProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error) {
	if err := cp.validateProvider(); err != nil {
		return nil, err
	}

	modelName := cp.resolveTier(tier)
	claudeResult, err := cp.client.RunValidation(ctx, commands, modelName, workDir)
	if err != nil {
		return nil, err
	}

	return convertResult(claudeResult), nil
}

// IsUsageLimitError detects Claude-specific usage limit errors.
// Checks for exit code 2 with stderr containing "usage limit", "rate limit",
// or "quota exceeded" (case-insensitive).
func (cp *ClaudeProvider) IsUsageLimitError(result *Result, err error) bool {
	if result == nil {
		return false
	}
	// Must have exit code 2 to be a usage limit error
	if result.ExitCode != 2 {
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

// IsValidationPassed checks if the result indicates validation passed.
// Delegates to claude.IsValidationPassed() to check for the VALIDATION_PASSED marker.
func (cp *ClaudeProvider) IsValidationPassed(result *Result) bool {
	return IsValidationPassed(result)
}

// IsScopeTooLarge checks if the result indicates the task scope is too large.
// Delegates to the shared helper function to check for the SCOPE_TOO_LARGE marker.
// Returns true and the explanation text if the marker is found.
func (cp *ClaudeProvider) IsScopeTooLarge(result *Result) (bool, string) {
	return IsScopeTooLarge(result)
}
