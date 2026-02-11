package provider

import (
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
