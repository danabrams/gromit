package provider

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/claude"
)

// ClaudeProvider wraps the Claude CLI client and implements the Provider interface
type ClaudeProvider struct {
	client      *claude.Client
	tierToModel map[string]string
}

// NewClaudeProvider creates a new ClaudeProvider with the given client and tier mapping
func NewClaudeProvider(client *claude.Client, tierToModel map[string]string) *ClaudeProvider {
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
