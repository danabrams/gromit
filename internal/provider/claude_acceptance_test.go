//go:build acceptance

package provider

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/claude"
)

// mockClaudeClient is a test double that implements the claudeClient interface
// for acceptance testing ClaudeProvider behavior
type mockClaudeClient struct {
	runValidationFn func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error)
	runFn           func(ctx context.Context, prompt string, model string) (*claude.Result, error)
}

func (m *mockClaudeClient) Run(ctx context.Context, prompt string, model string) (*claude.Result, error) {
	if m.runFn != nil {
		return m.runFn(ctx, prompt, model)
	}
	return &claude.Result{Success: true, Output: "test output"}, nil
}

func (m *mockClaudeClient) StreamRun(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
	return &claude.Result{Success: true}, nil
}

func (m *mockClaudeClient) RunValidation(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
	if m.runValidationFn != nil {
		return m.runValidationFn(ctx, commands, model, workDir)
	}
	return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
}

// TestClaudeProviderRunValidationDelegation verifies that ClaudeProvider.RunValidation()
// correctly delegates to claude.Client.RunValidation() and converts the result.
// Expected failure: RunValidation() method does not correctly delegate to claude.Client
func TestClaudeProviderRunValidationDelegation(t *testing.T) {
	// Track what parameters were passed to the underlying client
	var capturedCommands []string
	var capturedModel string
	var capturedWorkDir string

	mockClient := &mockClaudeClient{
		runValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			capturedCommands = commands
			capturedModel = model
			capturedWorkDir = workDir
			return &claude.Result{
				Success:  true,
				Output:   "VALIDATION_PASSED\nAll tests passed",
				ExitCode: 0,
				Model:    model,
			}, nil
		},
	}

	tierMap := map[string]string{
		TierLow:    "haiku",
		TierMedium: "sonnet",
		TierHigh:   "opus",
	}
	cp := NewClaudeProvider(mockClient, tierMap)

	commands := []string{"go test ./...", "go vet ./..."}
	workDir := "/tmp/test"
	ctx := context.Background()

	result, err := cp.RunValidation(ctx, commands, TierLow, workDir)

	if err != nil {
		t.Fatalf("RunValidation() returned error: %v", err)
	}

	// Verify correct tier resolution
	if capturedModel != "haiku" {
		t.Errorf("RunValidation() passed model=%q, want %q", capturedModel, "haiku")
	}

	// Verify parameters were passed through correctly
	if !stringSliceEqual(capturedCommands, commands) {
		t.Errorf("RunValidation() passed commands=%v, want %v", capturedCommands, commands)
	}
	if capturedWorkDir != workDir {
		t.Errorf("RunValidation() passed workDir=%q, want %q", capturedWorkDir, workDir)
	}

	// Verify result conversion
	if !result.Success {
		t.Errorf("RunValidation() result.Success=%v, want true", result.Success)
	}
	if result.ExitCode != 0 {
		t.Errorf("RunValidation() result.ExitCode=%d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Output, "VALIDATION_PASSED") {
		t.Errorf("RunValidation() result.Output missing VALIDATION_PASSED marker, got: %q", result.Output)
	}
}

// TestClaudeProviderRunValidationTierResolution verifies that RunValidation()
// resolves abstract tiers to concrete model names using the tierToModel map.
// Expected failure: RunValidation() does not resolve tiers correctly
func TestClaudeProviderRunValidationTierResolution(t *testing.T) {
	tests := []struct {
		tier          string
		expectedModel string
	}{
		{TierLow, "haiku"},
		{TierMedium, "sonnet"},
		{TierHigh, "opus"},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			var capturedModel string
			mockClient := &mockClaudeClient{
				runValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
					capturedModel = model
					return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
				},
			}

			tierMap := map[string]string{
				TierLow:    "haiku",
				TierMedium: "sonnet",
				TierHigh:   "opus",
			}
			cp := NewClaudeProvider(mockClient, tierMap)

			_, err := cp.RunValidation(context.Background(), []string{"go test"}, tt.tier, "/tmp")

			if err != nil {
				t.Fatalf("RunValidation() error: %v", err)
			}
			if capturedModel != tt.expectedModel {
				t.Errorf("RunValidation() with tier %q used model %q, want %q", tt.tier, capturedModel, tt.expectedModel)
			}
		})
	}
}

// TestClaudeProviderIsUsageLimitErrorWithExitCode2 verifies that
// IsUsageLimitError() detects Claude-specific usage limit errors
// with exit code 2 and specific error messages in stderr.
// Expected failure: IsUsageLimitError() does not detect exit code 2 + usage limit message patterns
func TestClaudeProviderIsUsageLimitErrorWithExitCode2(t *testing.T) {
	cp := &ClaudeProvider{}

	tests := []struct {
		name     string
		result   *Result
		err      error
		expected bool
	}{
		{
			name: "exit code 2 with usage limit message",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: usage limit exceeded. Please try again later.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 2 with rate limit message",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: rate limit exceeded. Please wait before retrying.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 2 with quota exceeded message",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: quota exceeded for this billing period.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 2 with case-insensitive USAGE LIMIT",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: USAGE LIMIT has been reached.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 2 with Rate Limit mixed case",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: Rate Limit exceeded.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 2 with Quota Exceeded mixed case",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: Quota Exceeded for your account.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 1 with usage limit message - not a limit error",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "Error: usage limit exceeded.",
			},
			err:      nil,
			expected: false, // Wrong exit code
		},
		{
			name: "exit code 2 with generic error - not a limit error",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Error: invalid API key.",
			},
			err:      nil,
			expected: false, // No usage limit keywords
		},
		{
			name: "exit code 2 with stderr containing usage limit",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Something failed.\n\nSTDERR:\nError: usage limit exceeded",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 2 with stderr containing rate limit",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Command failed.\n\nSTDERR:\nrate limit reached",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "exit code 2 with stderr containing quota exceeded",
			result: &Result{
				Success:  false,
				ExitCode: 2,
				Output:   "Failed to run.\n\nSTDERR:\nYour quota exceeded the monthly limit",
			},
			err:      nil,
			expected: true,
		},
		{
			name:     "nil result returns false",
			result:   nil,
			err:      nil,
			expected: false,
		},
		{
			name: "successful result returns false",
			result: &Result{
				Success:  true,
				ExitCode: 0,
				Output:   "All good",
			},
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cp.IsUsageLimitError(tt.result, tt.err)
			if got != tt.expected {
				t.Errorf("IsUsageLimitError() = %v, want %v (result=%+v)", got, tt.expected, tt.result)
			}
		})
	}
}

// NOTE: These tests are temporarily moved to claude_helper_methods_test.go
// because they reference methods that don't exist yet and prevent compilation.
// They will be restored once IsValidationPassed() and IsScopeTooLarge() are implemented.

// TestClaudeProviderRunValidationFailure verifies that RunValidation()
// correctly handles validation failures and returns appropriate Result.
// Expected failure: RunValidation() does not correctly handle failed validations
func TestClaudeProviderRunValidationFailure(t *testing.T) {
	mockClient := &mockClaudeClient{
		runValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{
				Success:  false,
				Output:   "VALIDATION_FAILED\nTest suite failed with 3 errors.",
				ExitCode: 1,
			}, nil
		},
	}

	tierMap := map[string]string{TierLow: "haiku"}
	cp := NewClaudeProvider(mockClient, tierMap)

	result, err := cp.RunValidation(context.Background(), []string{"go test"}, TierLow, "/tmp")

	if err != nil {
		t.Fatalf("RunValidation() returned error: %v", err)
	}

	if result.Success {
		t.Errorf("RunValidation() result.Success = true, want false for failed validation")
	}
	if result.ExitCode != 1 {
		t.Errorf("RunValidation() result.ExitCode = %d, want 1", result.ExitCode)
	}
	if !strings.Contains(result.Output, "VALIDATION_FAILED") {
		t.Errorf("RunValidation() output missing VALIDATION_FAILED, got: %q", result.Output)
	}
}

// TestClaudeProviderRunValidationError verifies that RunValidation()
// propagates errors from the underlying client.
// Expected failure: RunValidation() does not propagate errors correctly
func TestClaudeProviderRunValidationError(t *testing.T) {
	expectedErr := fmt.Errorf("client error: timeout")
	mockClient := &mockClaudeClient{
		runValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return nil, expectedErr
		},
	}

	tierMap := map[string]string{TierLow: "haiku"}
	cp := NewClaudeProvider(mockClient, tierMap)

	result, err := cp.RunValidation(context.Background(), []string{"go test"}, TierLow, "/tmp")

	if err == nil {
		t.Fatal("RunValidation() returned nil error, want error")
	}
	if result != nil {
		t.Errorf("RunValidation() returned result=%+v, want nil when error occurs", result)
	}
	if err != expectedErr {
		t.Errorf("RunValidation() error = %v, want %v", err, expectedErr)
	}
}

// TestClaudeProviderNilClientValidation verifies that RunValidation()
// returns an error when called with a nil client.
// Expected failure: RunValidation() does not validate nil client
func TestClaudeProviderNilClientValidation(t *testing.T) {
	cp := &ClaudeProvider{
		client:      nil,
		tierToModel: map[string]string{TierLow: "haiku"},
	}

	result, err := cp.RunValidation(context.Background(), []string{"go test"}, TierLow, "/tmp")

	if err == nil {
		t.Fatal("RunValidation() with nil client returned nil error, want error")
	}
	if result != nil {
		t.Errorf("RunValidation() with nil client returned result=%+v, want nil", result)
	}
}

// Helper function to compare string slices
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
