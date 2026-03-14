package provider

import (
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/claude"
)

// TestClaudeProviderStructExists verifies that ClaudeProvider struct exists
// and can be instantiated.
// Expected failure: ClaudeProvider struct does not exist yet
func TestClaudeProviderStructExists(t *testing.T) {
	t.Parallel()
	var cp *ClaudeProvider
	if cp != nil {
		t.Error("nil ClaudeProvider should be nil")
	}
}

// TestClaudeProviderHasClientField verifies that ClaudeProvider has a claude.Client field.
// Expected failure: ClaudeProvider struct and client field do not exist yet
func TestClaudeProviderHasClientField(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
	cp := &ClaudeProvider{}

	// Verify we can call Run() with the expected signature
	// We expect it to fail since client is nil, but the method should exist
	_, err := cp.Run(nil, "test prompt", TierMedium)

	// We expect an error (nil client), but the method signature should be correct
	if err == nil {
		t.Error("Run() with nil client should return an error")
	}
}

// TestClaudeProviderStreamRunMethodSignature verifies that StreamRun() method
// exists with the correct signature matching the Provider interface.
// Expected failure: StreamRun() method does not exist yet
func TestClaudeProviderStreamRunMethodSignature(t *testing.T) {
	t.Parallel()
	cp := &ClaudeProvider{}

	var handler EventHandler
	var toolHandler ToolCallHandler

	// Verify we can call StreamRun() with the expected signature
	// We expect it to fail since client is nil, but the method should exist
	_, err := cp.StreamRun(nil, "test prompt", TierHigh, nil, handler, toolHandler)

	// We expect an error (nil client), but the method signature should be correct
	if err == nil {
		t.Error("StreamRun() with nil client should return an error")
	}
}

// TestClaudeProviderStreamRunInDirMethodSignature verifies that StreamRunInDir()
// method exists and implements the DirStreamRunner interface.
func TestClaudeProviderStreamRunInDirMethodSignature(t *testing.T) {
	t.Parallel()
	cp := &ClaudeProvider{}

	// Verify compile-time interface satisfaction
	var _ DirStreamRunner = cp

	// Verify we can call StreamRunInDir with the expected signature
	_, err := cp.StreamRunInDir(nil, "test prompt", TierHigh, "/tmp/dir", nil, nil, nil)
	if err == nil {
		t.Error("StreamRunInDir() with nil client should return an error")
	}
}

// TestClaudeProviderRunValidationMethodSignature verifies that RunValidation()
// method exists with the correct signature matching the Provider interface.
// Expected failure: RunValidation() method does not exist yet
func TestClaudeProviderRunValidationMethodSignature(t *testing.T) {
	t.Parallel()
	cp := &ClaudeProvider{}

	commands := []string{"go test ./..."}
	workDir := t.TempDir()

	// Verify we can call RunValidation() with the expected signature
	// We expect it to fail since client is nil, but the method should exist
	_, err := cp.RunValidation(nil, commands, TierLow, workDir)

	// We expect an error (nil client), but the method signature should be correct
	if err == nil {
		t.Error("RunValidation() with nil client should return an error")
	}
}

// TestClaudeProviderIsUsageLimitError verifies that IsUsageLimitError()
// returns false for Claude, since Claude CLI does not return usage limit errors
// in a detectable way currently.
// Expected failure: IsUsageLimitError() method does not exist yet
func TestClaudeProviderIsUsageLimitError(t *testing.T) {
	t.Parallel()
	cp := &ClaudeProvider{}

	tests := []struct {
		name     string
		result   *Result
		err      error
		expected bool
	}{
		{
			name:     "nil result and error",
			result:   nil,
			err:      nil,
			expected: false,
		},
		{
			name: "failed result with generic error",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "some error",
			},
			err:      nil,
			expected: false,
		},
		{
			name:     "error without result",
			result:   nil,
			err:      fmt.Errorf("context deadline exceeded"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := cp.IsUsageLimitError(tt.result, tt.err)
			if got != tt.expected {
				t.Errorf("IsUsageLimitError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestClaudeProviderIsValidationPassedDelegation verifies that
// IsValidationPassed() delegates to claude.IsValidationPassed().
func TestClaudeProviderIsValidationPassedDelegation(t *testing.T) {
	t.Parallel()
	cp := &ClaudeProvider{}

	result := &Result{
		Success: true,
		Output:  "All tests passed.\nVALIDATION_PASSED",
	}

	got := cp.IsValidationPassed(result)
	if !got {
		t.Errorf("IsValidationPassed() = %v, want true for result with VALIDATION_PASSED marker", got)
	}
}

// TestClaudeProviderIsScopeTooLargeDelegation verifies that
// IsScopeTooLarge() delegates to claude.IsScopeTooLarge().
func TestClaudeProviderIsScopeTooLargeDelegation(t *testing.T) {
	t.Parallel()
	cp := &ClaudeProvider{}

	result := &Result{
		Success: true,
		Output:  "Analysis:\nSCOPE_TOO_LARGE: This task touches 8 files across 4 packages.\n\nMore details...",
	}

	gotTooLarge, gotExplanation := cp.IsScopeTooLarge(result)
	if !gotTooLarge {
		t.Errorf("IsScopeTooLarge() tooLarge = %v, want true for result with SCOPE_TOO_LARGE marker", gotTooLarge)
	}
	if gotExplanation == "" {
		t.Error("IsScopeTooLarge() explanation should not be empty when marker is present")
	}
}

// TestClaudeProviderIsUsageLimitErrorExitCode2 verifies that
// IsUsageLimitError() detects exit code 2 with usage limit keywords.
func TestClaudeProviderIsUsageLimitErrorExitCode2(t *testing.T) {
	t.Parallel()
	cp := &ClaudeProvider{}

	result := &Result{
		Success:  false,
		ExitCode: 2,
		Output:   "Error: usage limit exceeded",
	}

	got := cp.IsUsageLimitError(result, nil)
	if !got {
		t.Errorf("IsUsageLimitError() = %v, want true for exit code 2 with 'usage limit'", got)
	}
}
