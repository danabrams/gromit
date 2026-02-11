package provider

import (
	"testing"

	"github.com/danabrams/gromit/internal/claude"
)

// TestClaudeProviderStructExists verifies that ClaudeProvider struct exists
// and can be instantiated.
// Expected failure: ClaudeProvider struct does not exist yet
func TestClaudeProviderStructExists(t *testing.T) {
	var cp *ClaudeProvider
	if cp != nil {
		t.Error("nil ClaudeProvider should be nil")
	}
}

// TestClaudeProviderHasClientField verifies that ClaudeProvider has a claude.Client field.
// Expected failure: ClaudeProvider struct and client field do not exist yet
func TestClaudeProviderHasClientField(t *testing.T) {
	mockClient := &claude.Client{}
	cp := &ClaudeProvider{
		client: mockClient,
	}

	if cp.client == nil {
		t.Error("ClaudeProvider.client should not be nil after assignment")
	}
}
