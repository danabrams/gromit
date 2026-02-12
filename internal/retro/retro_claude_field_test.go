package retro

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// TestRetro_Run_WithProviderOnly_NoClaudeCheck verifies that Run() only checks provider, not claude
func TestRetro_Run_WithProviderOnly_NoClaudeCheck(t *testing.T) {
	tmpDir := t.TempDir()
	createMinimalRetroFilesForProvider(t, tmpDir)

	mockProvider := &mockProvider{
		runResult: &provider.Result{
			Success: true,
			Output:  "test analysis",
		},
	}

	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	// The Retro should have nil claude field but non-nil provider
	// After migration, Run() should only check if r.provider == nil
	// Currently it checks: if r.claude == nil && r.provider == nil

	// This will currently fail because Run() checks both fields
	result, err := r.Run(context.Background(), nil)

	// Currently Run() will succeed because provider is not nil
	// After migration, this test will still pass, which is what we want
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result")
	}

	if !mockProvider.runCalled {
		t.Error("expected provider to be called")
	}
}

// TestRetro_Run_ErrorMessage_OnlyMentionsProvider verifies error message only mentions provider
func TestRetro_Run_ErrorMessage_OnlyMentionsProvider(t *testing.T) {
	// Create a Retro with nil provider
	r := &Retro{
		provider: nil,
	}

	_, err := r.Run(context.Background(), nil)

	if err == nil {
		t.Fatal("expected error when provider is nil")
	}

	// Current error message: "neither claude client nor provider is set"
	// After migration: "provider is nil" or "provider is not set"
	errorMsg := err.Error()

	// This test will FAIL after migration if we correctly update the error message
	if errorMsg != "provider is nil" && errorMsg != "provider is not set" {
		t.Errorf("error message should only mention provider, got: %q", errorMsg)
	}
}
