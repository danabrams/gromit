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

// TestNewClaudeProviderConstructor verifies that NewClaudeProvider constructor
// creates a ClaudeProvider with the provided client and tier-to-model mapping.
// Expected failure: NewClaudeProvider() function does not exist yet
func TestNewClaudeProviderConstructor(t *testing.T) {
	mockClient := &claude.Client{}
	tierMap := map[string]string{
		TierHigh:   "opus",
		TierMedium: "sonnet",
		TierLow:    "haiku",
	}

	cp := NewClaudeProvider(mockClient, tierMap)

	if cp == nil {
		t.Fatal("NewClaudeProvider() returned nil")
	}

	if cp.client != mockClient {
		t.Error("ClaudeProvider.client not set correctly")
	}

	if cp.tierToModel == nil {
		t.Error("ClaudeProvider.tierToModel is nil")
	}

	if cp.tierToModel[TierHigh] != "opus" {
		t.Errorf("tierToModel[TierHigh] = %q, want %q", cp.tierToModel[TierHigh], "opus")
	}
}

// TestClaudeProviderResolveTier verifies that resolveTier() helper maps
// tier constants to model names using the tierToModel map.
// Expected failure: resolveTier() method does not exist yet
func TestClaudeProviderResolveTier(t *testing.T) {
	tierMap := map[string]string{
		TierHigh:   "opus",
		TierMedium: "sonnet",
		TierLow:    "haiku",
	}

	cp := &ClaudeProvider{
		tierToModel: tierMap,
	}

	tests := []struct {
		tier     string
		expected string
	}{
		{TierHigh, "opus"},
		{TierMedium, "sonnet"},
		{TierLow, "haiku"},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			modelName := cp.resolveTier(tt.tier)
			if modelName != tt.expected {
				t.Errorf("resolveTier(%q) = %q, want %q", tt.tier, modelName, tt.expected)
			}
		})
	}
}

// TestClaudeProviderRunMethodSignature verifies that Run() method exists
// with the correct signature matching the Provider interface.
// Expected failure: Run() method does not exist yet
func TestClaudeProviderRunMethodSignature(t *testing.T) {
	cp := &ClaudeProvider{}

	// Verify we can call Run() with the expected signature
	// We expect it to fail since client is nil, but the method should exist
	_, err := cp.Run(nil, "test prompt", TierMedium)

	// We expect an error (nil client), but the method signature should be correct
	if err == nil {
		t.Error("Run() with nil client should return an error")
	}
}
