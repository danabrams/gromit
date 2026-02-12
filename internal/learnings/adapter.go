package learnings

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/provider"
)

// ClaudeClientRunner is an interface for invoking claude.Client.
// This is used internally for dependency injection.
type ClaudeClientRunner interface {
	Run(ctx context.Context, prompt string, model string) (*claude.Result, error)
}

// claudeRunnerAdapter adapts claude.Client to the ClaudeRunner interface
// by converting claude.Result to learnings.Result.
type claudeRunnerAdapter struct {
	client ClaudeClientRunner
}

// NewClaudeRunnerAdapter creates a new adapter from a Claude client
func NewClaudeRunnerAdapter(client ClaudeClientRunner) *claudeRunnerAdapter {
	return &claudeRunnerAdapter{client: client}
}

// Run implements ClaudeRunner by calling the underlying Claude client and converting the result
func (a *claudeRunnerAdapter) Run(ctx context.Context, prompt string, model string) (*Result, error) {
	if a.client == nil {
		return nil, fmt.Errorf("claude client is nil")
	}

	claudeResult, err := a.client.Run(ctx, prompt, model)
	if err != nil {
		return nil, err
	}

	if claudeResult == nil {
		return nil, fmt.Errorf("claude returned nil result")
	}

	// Convert claude.Result to learnings.Result
	return &Result{
		Success: claudeResult.Success,
		Output:  claudeResult.Output,
	}, nil
}

// ProjectDescriptions holds standard project descriptions used across adapters
var ProjectDescriptions = struct {
	Gromit string
}{
	Gromit: "A Go CLI tool that runs the Gromit loop with fresh context on each iteration",
}

// ProviderRunner is an interface for invoking provider.Provider.
// This is used for dependency injection in learnings filtering.
type ProviderRunner interface {
	Run(ctx context.Context, prompt string, tier string) (*provider.Result, error)
}

// providerRunnerAdapter adapts provider.Provider to the ClaudeRunner interface
// by converting provider.Result to learnings.Result.
type providerRunnerAdapter struct {
	provider ProviderRunner
}

// NewProviderRunnerAdapter creates a new adapter from a Provider
func NewProviderRunnerAdapter(p ProviderRunner) *providerRunnerAdapter {
	return &providerRunnerAdapter{provider: p}
}

// Run implements ClaudeRunner by calling the underlying Provider and converting the result
func (a *providerRunnerAdapter) Run(ctx context.Context, prompt string, model string) (*Result, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("provider is nil")
	}

	providerResult, err := a.provider.Run(ctx, prompt, model)
	if err != nil {
		return nil, err
	}

	if providerResult == nil {
		return nil, fmt.Errorf("provider returned nil result")
	}

	// Convert provider.Result to learnings.Result
	return &Result{
		Success: providerResult.Success,
		Output:  providerResult.Output,
	}, nil
}
