package provider

import (
	"context"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/claude"
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
	return "claude"
}

// resolveTier maps an abstract tier to a concrete model name
func (cp *ClaudeProvider) resolveTier(tier string) string {
	if modelName, ok := cp.tierToModel[tier]; ok {
		return modelName
	}
	return tier
}

// Run executes an LLM invocation with the given prompt and tier.
// It resolves the tier to a model name and delegates to claude.Client.Run().
func (cp *ClaudeProvider) Run(ctx context.Context, prompt string, tier string) (*Result, error) {
	if cp == nil {
		return nil, fmt.Errorf("claude provider is nil")
	}
	if cp.client == nil {
		return nil, fmt.Errorf("claude client is nil")
	}

	// Resolve tier to model name
	modelName := cp.resolveTier(tier)

	// Delegate to claude.Client
	claudeResult, err := cp.client.Run(ctx, prompt, modelName)
	if err != nil {
		return nil, err
	}

	// Convert claude.Result to provider.Result
	return &Result{
		Success:  claudeResult.Success,
		Output:   claudeResult.Output,
		ExitCode: claudeResult.ExitCode,
		Duration: claudeResult.Duration,
		Model:    claudeResult.Model,
	}, nil
}

// StreamRun executes an LLM invocation with streaming output.
// It resolves the tier to a model name and delegates to claude.Client.StreamRun().
func (cp *ClaudeProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	if cp == nil {
		return nil, fmt.Errorf("claude provider is nil")
	}
	if cp.client == nil {
		return nil, fmt.Errorf("claude client is nil")
	}

	// Resolve tier to model name
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

	// Delegate to claude.Client
	claudeResult, err := cp.client.StreamRun(ctx, prompt, modelName, output, claudeHandler, claudeToolHandler)
	if err != nil {
		return nil, err
	}

	// Convert claude.Result to provider.Result
	return &Result{
		Success:  claudeResult.Success,
		Output:   claudeResult.Output,
		ExitCode: claudeResult.ExitCode,
		Duration: claudeResult.Duration,
		Model:    claudeResult.Model,
	}, nil
}

// RunValidation executes validation commands using the LLM.
// It resolves the tier to a model name and delegates to claude.Client.RunValidation().
func (cp *ClaudeProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error) {
	if cp == nil {
		return nil, fmt.Errorf("claude provider is nil")
	}
	if cp.client == nil {
		return nil, fmt.Errorf("claude client is nil")
	}

	// Resolve tier to model name
	modelName := cp.resolveTier(tier)

	// Delegate to claude.Client
	claudeResult, err := cp.client.RunValidation(ctx, commands, modelName, workDir)
	if err != nil {
		return nil, err
	}

	// Convert claude.Result to provider.Result
	return &Result{
		Success:  claudeResult.Success,
		Output:   claudeResult.Output,
		ExitCode: claudeResult.ExitCode,
		Duration: claudeResult.Duration,
		Model:    claudeResult.Model,
	}, nil
}

// IsUsageLimitError detects Claude-specific usage limit errors.
// Currently returns false as Claude CLI does not return usage limit errors
// in a detectable pattern. This may be updated in the future as error patterns
// are identified.
func (cp *ClaudeProvider) IsUsageLimitError(result *Result, err error) bool {
	// Claude CLI does not currently have detectable usage limit error patterns
	// that are distinct from other errors. Always return false for now.
	return false
}
