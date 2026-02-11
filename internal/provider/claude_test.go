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

// TestClaudeProviderHasTierModelMap verifies that ClaudeProvider has a
// tierToModel map field for mapping abstract tiers to concrete model names.
// Expected failure: tierToModel field does not exist yet
func TestClaudeProviderHasTierModelMap(t *testing.T) {
	tierMap := map[string]string{
		TierHigh:   "opus",
		TierMedium: "sonnet",
		TierLow:    "haiku",
	}

	cp := &ClaudeProvider{
		tierToModel: tierMap,
	}

	if cp.tierToModel == nil {
		t.Error("ClaudeProvider.tierToModel should not be nil after assignment")
	}

	if cp.tierToModel[TierHigh] != "opus" {
		t.Errorf("tierToModel[TierHigh] = %q, want %q", cp.tierToModel[TierHigh], "opus")
	}
}

// TestClaudeProviderNameMethod verifies that ClaudeProvider implements
// Name() method returning "claude".
// Expected failure: Name() method does not exist yet
func TestClaudeProviderNameMethod(t *testing.T) {
	cp := &ClaudeProvider{}

	name := cp.Name()

	if name != "claude" {
		t.Errorf("Name() = %q, want %q", name, "claude")
	}
}
