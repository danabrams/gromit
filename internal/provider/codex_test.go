package provider

import (
	"testing"
)

// TestCodexProviderStructExists verifies that CodexProvider struct exists
// and can be instantiated.
func TestCodexProviderStructExists(t *testing.T) {
	var cp *CodexProvider
	if cp != nil {
		t.Error("nil CodexProvider should be nil")
	}
}

// TestNewCodexProviderConstructor verifies that NewCodexProvider constructor
// creates a CodexProvider with all required fields set correctly.
func TestNewCodexProviderConstructor(t *testing.T) {
	binaryPath := "/usr/local/bin/codex"
	flags := []string{"--no-color"}
	promptDelivery := "prompt_file_arg"
	promptFlag := "--prompt"
	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}

	cp := NewCodexProvider(binaryPath, flags, promptDelivery, promptFlag, tierMap)

	if cp == nil {
		t.Fatal("NewCodexProvider() returned nil")
	}

	if cp.binaryPath != binaryPath {
		t.Errorf("binaryPath = %q, want %q", cp.binaryPath, binaryPath)
	}

	if len(cp.flags) != len(flags) || cp.flags[0] != flags[0] {
		t.Errorf("flags = %v, want %v", cp.flags, flags)
	}

	if cp.promptDelivery != promptDelivery {
		t.Errorf("promptDelivery = %q, want %q", cp.promptDelivery, promptDelivery)
	}

	if cp.promptFlag != promptFlag {
		t.Errorf("promptFlag = %q, want %q", cp.promptFlag, promptFlag)
	}

	if cp.tierToModel[TierHigh] != "o3" {
		t.Errorf("tierToModel[TierHigh] = %q, want %q", cp.tierToModel[TierHigh], "o3")
	}
}

// TestCodexProviderNameMethod verifies that CodexProvider implements
// Name() method returning "codex".
func TestCodexProviderNameMethod(t *testing.T) {
	cp := &CodexProvider{}

	name := cp.Name()

	if name != "codex" {
		t.Errorf("Name() = %q, want %q", name, "codex")
	}
}

// TestCodexProviderModelForTierReturnsCorrectModel verifies that ModelForTier()
// maps tier constants to the configured Codex model names.
func TestCodexProviderModelForTierReturnsCorrectModel(t *testing.T) {
	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}

	cp := &CodexProvider{
		tierToModel: tierMap,
	}

	tests := []struct {
		tier      string
		wantModel string
	}{
		{TierHigh, "o3"},
		{TierMedium, "gpt-4o"},
		{TierLow, "gpt-4o-mini"},
	}

	for _, tt := range tests {
		t.Run("tier_"+tt.tier, func(t *testing.T) {
			got := cp.ModelForTier(tt.tier)
			if got != tt.wantModel {
				t.Errorf("ModelForTier(%q) = %q, want %q", tt.tier, got, tt.wantModel)
			}
		})
	}
}
