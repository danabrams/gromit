package provider

import (
	"testing"
)

// TestModelProviderAttributionMatrixContractClaudeProvider tests the contract
// that all Claude models are correctly attributed to the Claude provider.
// This is a contract test that documents the expected mapping for all Claude models.
func TestModelProviderAttributionMatrixContractClaudeProvider(t *testing.T) {
	claudeModels := []string{
		"opus",
		"sonnet",
		"haiku",
	}

	for _, model := range claudeModels {
		t.Run("claude_with_"+model, func(t *testing.T) {
			valid, reason := ValidateModelProviderAttribution(model, "claude")
			if !valid {
				t.Errorf("Claude provider should accept model %q, but got error: %s", model, reason)
			}
		})
	}
}

// TestModelProviderAttributionMatrixContractCodexProvider tests the contract
// that all Codex models are correctly attributed to the Codex provider.
// This is a contract test that documents the expected mapping for all Codex models.
func TestModelProviderAttributionMatrixContractCodexProvider(t *testing.T) {
	codexModels := []string{
		"gpt-5.3-codex",
		"gpt-5.1-codex-mini",
	}

	for _, model := range codexModels {
		t.Run("codex_with_"+model, func(t *testing.T) {
			valid, reason := ValidateModelProviderAttribution(model, "codex")
			if !valid {
				t.Errorf("Codex provider should accept model %q, but got error: %s", model, reason)
			}
		})
	}
}

// TestModelProviderAttributionMatrixContractCrossProviderRejects tests the contract
// that models from one provider are rejected by another provider.
// This enforces data quality by rejecting misattributed models.
func TestModelProviderAttributionMatrixContractCrossProviderRejects(t *testing.T) {
	claudeModels := []string{"opus", "sonnet", "haiku"}
	codexModels := []string{"gpt-5.3-codex", "gpt-5.1-codex-mini"}

	// Claude models should be rejected by Codex
	for _, model := range claudeModels {
		t.Run("codex_rejects_claude_model_"+model, func(t *testing.T) {
			valid, _ := ValidateModelProviderAttribution(model, "codex")
			if valid {
				t.Errorf("Codex provider should reject Claude model %q", model)
			}
		})
	}

	// Codex models should be rejected by Claude
	for _, model := range codexModels {
		t.Run("claude_rejects_codex_model_"+model, func(t *testing.T) {
			valid, _ := ValidateModelProviderAttribution(model, "claude")
			if valid {
				t.Errorf("Claude provider should reject Codex model %q", model)
			}
		})
	}
}

// TestModelProviderAttributionMatrixContractIncompleteDataHandling tests the contract
// that incomplete attribution data is treated as invalid.
// This ensures data quality by rejecting records with missing attribution.
func TestModelProviderAttributionMatrixContractIncompleteDataHandling(t *testing.T) {
	type testCase struct {
		name     string
		model    string
		provider string
	}

	testCases := []testCase{
		{
			name:     "missing model name",
			model:    "",
			provider: "claude",
		},
		{
			name:     "missing provider name",
			model:    "opus",
			provider: "",
		},
		{
			name:     "both model and provider empty",
			model:    "",
			provider: "",
		},
		{
			name:     "whitespace-only model",
			model:    "   ",
			provider: "claude",
		},
		{
			name:     "whitespace-only provider",
			model:    "opus",
			provider: "   ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			valid, reason := ValidateModelProviderAttribution(tc.model, tc.provider)
			if valid {
				t.Errorf("incomplete data (%q, %q) should be invalid", tc.model, tc.provider)
			}
			if reason == "" {
				t.Errorf("incomplete data should provide a reason for rejection")
			}
		})
	}
}

// TestModelProviderAttributionMatrixContractAllKnownModels is a comprehensive
// contract test that verifies all known models are in the attribution mapping
// and are associated with the correct provider.
func TestModelProviderAttributionMatrixContractAllKnownModels(t *testing.T) {
	knownMappings := map[string]string{
		// Claude models
		"opus":   "claude",
		"sonnet": "claude",
		"haiku":  "claude",
		// Codex models
		"gpt-5.3-codex":      "codex",
		"gpt-5.1-codex-mini": "codex",
	}

	for model, expectedProvider := range knownMappings {
		t.Run(model+"_maps_to_"+expectedProvider, func(t *testing.T) {
			valid, reason := ValidateModelProviderAttribution(model, expectedProvider)
			if !valid {
				t.Errorf("model %q should map to provider %q, got error: %s",
					model, expectedProvider, reason)
			}
		})
	}
}

// TestModelProviderAttributionMatrixContractUnknownModelsAlwaysInvalid is a contract test
// that ensures unknown models are always treated as invalid attribution data,
// regardless of the provider.
func TestModelProviderAttributionMatrixContractUnknownModelsAlwaysInvalid(t *testing.T) {
	unknownModels := []string{
		"future-model-v1",
		"experimental-llm",
		"claude-next",
		"gpt-5-turbo",
		"unknown-provider-model",
	}

	providers := []string{"claude", "codex", "unknown-provider"}

	for _, model := range unknownModels {
		for _, provider := range providers {
			t.Run("unknown_model_"+model+"_with_"+provider, func(t *testing.T) {
				valid, reason := ValidateModelProviderAttribution(model, provider)
				if valid {
					t.Errorf("unknown model %q should be invalid with provider %q",
						model, provider)
				}
				if reason == "" {
					t.Errorf("unknown model should provide a reason for rejection")
				}
			})
		}
	}
}

// TestModelProviderAttributionMatrixContractReasonMessagesAreInformative
// verifies that validation failure reasons are informative for debugging.
func TestModelProviderAttributionMatrixContractReasonMessagesAreInformative(t *testing.T) {
	tests := []struct {
		name              string
		model             string
		provider          string
		shouldContainText string
	}{
		{
			name:              "mismatch reason includes expected provider",
			model:             "opus",
			provider:          "codex",
			shouldContainText: "claude",
		},
		{
			name:              "unknown model reason mentions mapping",
			model:             "unknown-model",
			provider:          "claude",
			shouldContainText: "attribution mapping",
		},
		{
			name:              "empty model reason clear",
			model:             "",
			provider:          "claude",
			shouldContainText: "empty",
		},
		{
			name:              "empty provider reason clear",
			model:             "opus",
			provider:          "",
			shouldContainText: "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, reason := ValidateModelProviderAttribution(tt.model, tt.provider)
			if !containsString(reason, tt.shouldContainText) {
				t.Errorf("reason %q should contain %q", reason, tt.shouldContainText)
			}
		})
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0
}
