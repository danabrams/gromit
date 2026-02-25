package provider

import (
	"testing"
)

// TestModelProviderAttributionKnownMappings tests that known model names
// are correctly mapped to their owning providers.
func TestModelProviderAttributionKnownMappings(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		provider     string
		wantValid    bool
		wantReason   string
	}{
		// Claude provider with Claude models
		{
			name:      "claude provider with opus model",
			model:     "opus",
			provider:  "claude",
			wantValid: true,
		},
		{
			name:      "claude provider with sonnet model",
			model:     "sonnet",
			provider:  "claude",
			wantValid: true,
		},
		{
			name:      "claude provider with haiku model",
			model:     "haiku",
			provider:  "claude",
			wantValid: true,
		},
		// Codex provider with Codex models
		{
			name:      "codex provider with gpt-5.3-codex model",
			model:     "gpt-5.3-codex",
			provider:  "codex",
			wantValid: true,
		},
		{
			name:      "codex provider with gpt-5.1-codex-mini model",
			model:     "gpt-5.1-codex-mini",
			provider:  "codex",
			wantValid: true,
		},
		// Cross-provider mismatches
		{
			name:      "claude provider with codex model is invalid",
			model:     "gpt-5.3-codex",
			provider:  "claude",
			wantValid: false,
		},
		{
			name:      "codex provider with claude model is invalid",
			model:     "opus",
			provider:  "codex",
			wantValid: false,
		},
		// Unknown models
		{
			name:      "unknown model with claude provider is invalid",
			model:     "unknown-model",
			provider:  "claude",
			wantValid: false,
		},
		{
			name:      "unknown model with codex provider is invalid",
			model:     "unknown-model",
			provider:  "codex",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, reason := ValidateModelProviderAttribution(tt.model, tt.provider)
			if valid != tt.wantValid {
				t.Errorf("ValidateModelProviderAttribution(%q, %q) valid = %v, want %v",
					tt.model, tt.provider, valid, tt.wantValid)
			}
			if !valid && reason == "" {
				t.Errorf("ValidateModelProviderAttribution(%q, %q) returned false but no reason",
					tt.model, tt.provider)
			}
		})
	}
}

// TestModelProviderAttributionEmptyData tests handling of empty/incomplete data.
func TestModelProviderAttributionEmptyData(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		provider  string
		wantValid bool
	}{
		{
			name:      "empty model and provider",
			model:     "",
			provider:  "",
			wantValid: false,
		},
		{
			name:      "empty model with provider",
			model:     "",
			provider:  "claude",
			wantValid: false,
		},
		{
			name:      "model with empty provider",
			model:     "opus",
			provider:  "",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, _ := ValidateModelProviderAttribution(tt.model, tt.provider)
			if valid != tt.wantValid {
				t.Errorf("ValidateModelProviderAttribution(%q, %q) = %v, want %v",
					tt.model, tt.provider, valid, tt.wantValid)
			}
		})
	}
}

// TestModelProviderAttributionCaseInsensitive tests case-insensitive matching
// for model and provider names.
func TestModelProviderAttributionCaseInsensitive(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		provider  string
		wantValid bool
	}{
		{
			name:      "uppercase model with lowercase provider",
			model:     "OPUS",
			provider:  "claude",
			wantValid: true,
		},
		{
			name:      "uppercase provider with lowercase model",
			model:     "opus",
			provider:  "CLAUDE",
			wantValid: true,
		},
		{
			name:      "mixed case both directions",
			model:     "OpUs",
			provider:  "ClAuDe",
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, _ := ValidateModelProviderAttribution(tt.model, tt.provider)
			if valid != tt.wantValid {
				t.Errorf("ValidateModelProviderAttribution(%q, %q) = %v, want %v",
					tt.model, tt.provider, valid, tt.wantValid)
			}
		})
	}
}
