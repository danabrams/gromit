package claude

import (
	"context"
	"fmt"
)

// Runner is an interface for invoking Claude
type Runner interface {
	Run(ctx context.Context, prompt string, model string) (*Result, error)
}

// ClaudeRunnerAdapter wraps a Claude client and converts claude.Result to a standardized result format
type ClaudeRunnerAdapter struct {
	client Runner
}

// NewClaudeRunnerAdapter creates a new adapter from a Claude client
func NewClaudeRunnerAdapter(client Runner) *ClaudeRunnerAdapter {
	return &ClaudeRunnerAdapter{client: client}
}

// Run invokes the Claude client and returns a standardized result
func (a *ClaudeRunnerAdapter) Run(ctx context.Context, prompt string, model string) (*Result, error) {
	if a == nil {
		return nil, fmt.Errorf("adapter is nil")
	}

	if a.client == nil {
		return nil, fmt.Errorf("claude client is nil")
	}

	claudeResult, err := a.client.Run(ctx, prompt, model)
	if err != nil {
		return nil, err
	}

	// Include nil-result safety check before accessing fields
	if claudeResult == nil {
		return nil, fmt.Errorf("claude returned nil result")
	}

	// Convert and return the result
	return &Result{
		Success:  claudeResult.Success,
		Output:   claudeResult.Output,
		ExitCode: claudeResult.ExitCode,
		Duration: claudeResult.Duration,
		Model:    claudeResult.Model,
	}, nil
}
