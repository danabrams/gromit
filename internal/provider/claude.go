package provider

import (
	"github.com/danabrams/gromit/internal/claude"
)

// ClaudeProvider wraps the Claude CLI client and implements the Provider interface
type ClaudeProvider struct {
	client *claude.Client
}
