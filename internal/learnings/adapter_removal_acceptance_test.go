//go:build acceptance

package learnings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClaudeRunnerAdapterRemoved verifies that the ClaudeClientRunner interface,
// claudeRunnerAdapter type, and NewClaudeRunnerAdapter constructor have been removed
// from internal/learnings/adapter.go after migration to provider-only approach.
// Expected failure: ClaudeClientRunner interface still exists in adapter.go
func TestClaudeRunnerAdapterRemoved(t *testing.T) {
	adapterPath := filepath.Join(".", "adapter.go")
	source, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("could not read adapter.go: %v", err)
	}

	sourceStr := string(source)

	// Should NOT contain ClaudeClientRunner interface
	if strings.Contains(sourceStr, "type ClaudeClientRunner interface") {
		t.Error("adapter.go should not define ClaudeClientRunner interface - it should be removed after provider migration")
	}

	// Should NOT contain claudeRunnerAdapter type
	if strings.Contains(sourceStr, "type claudeRunnerAdapter struct") {
		t.Error("adapter.go should not define claudeRunnerAdapter type - it should be removed after provider migration")
	}

	// Should NOT contain NewClaudeRunnerAdapter constructor
	if strings.Contains(sourceStr, "func NewClaudeRunnerAdapter") {
		t.Error("adapter.go should not define NewClaudeRunnerAdapter - it should be removed after provider migration")
	}

	// Should still contain providerRunnerAdapter (the replacement)
	if !strings.Contains(sourceStr, "type providerRunnerAdapter struct") {
		t.Error("adapter.go should still contain providerRunnerAdapter type for provider-based filtering")
	}

	// Should still contain NewProviderRunnerAdapter (the replacement constructor)
	if !strings.Contains(sourceStr, "func NewProviderRunnerAdapter") {
		t.Error("adapter.go should still contain NewProviderRunnerAdapter constructor")
	}
}

// TestReviewGoUsesProviderAdapter verifies that cmd/gromit/review.go has migrated
// from using learnings.NewClaudeRunnerAdapter to using a provider-based approach.
// Expected failure: review.go still calls learnings.NewClaudeRunnerAdapter(m.claudeClient)
func TestReviewGoUsesProviderAdapter(t *testing.T) {
	reviewPath := filepath.Join(".", "..", "..", "cmd", "gromit", "review.go")
	source, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("could not read review.go: %v", err)
	}

	sourceStr := string(source)

	// Should NOT call NewClaudeRunnerAdapter anymore
	if strings.Contains(sourceStr, "learnings.NewClaudeRunnerAdapter") {
		t.Error("review.go should not call learnings.NewClaudeRunnerAdapter - should use provider-based approach instead")
	}

	// Should use provider-based approach (NewProviderRunnerAdapter or direct provider)
	if !strings.Contains(sourceStr, "learnings.NewProviderRunnerAdapter") && !strings.Contains(sourceStr, "learnings.NewLLMFilter") {
		t.Error("review.go should use learnings.NewProviderRunnerAdapter or pass provider directly to NewLLMFilter")
	}
}

// TestAdapterTestsRemoved verifies that the tests for ClaudeRunnerAdapter
// have been removed from adapter_test.go since the code under test no longer exists.
// Expected failure: adapter_test.go still contains TestClaudeRunnerAdapter_Success and related tests
func TestAdapterTestsRemoved(t *testing.T) {
	testPath := filepath.Join(".", "adapter_test.go")
	source, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("could not read adapter_test.go: %v", err)
	}

	sourceStr := string(source)

	// Should NOT contain tests for ClaudeRunnerAdapter
	if strings.Contains(sourceStr, "func TestClaudeRunnerAdapter_Success") {
		t.Error("adapter_test.go should not contain TestClaudeRunnerAdapter_Success - test subject was removed")
	}

	if strings.Contains(sourceStr, "func TestClaudeRunnerAdapter_Error") {
		t.Error("adapter_test.go should not contain TestClaudeRunnerAdapter_Error - test subject was removed")
	}

	if strings.Contains(sourceStr, "func TestClaudeRunnerAdapter_NilClient") {
		t.Error("adapter_test.go should not contain TestClaudeRunnerAdapter_NilClient - test subject was removed")
	}

	if strings.Contains(sourceStr, "func TestClaudeRunnerAdapter_NilResult") {
		t.Error("adapter_test.go should not contain TestClaudeRunnerAdapter_NilResult - test subject was removed")
	}

	// Should NOT contain mock for ClaudeClientRunner
	if strings.Contains(sourceStr, "type mockClaudeClientRunner struct") {
		t.Error("adapter_test.go should not contain mockClaudeClientRunner - test subject was removed")
	}

	// Should still contain tests for ProviderRunnerAdapter if they exist
	// (Don't enforce their presence, just verify ClaudeRunnerAdapter tests are gone)
}
