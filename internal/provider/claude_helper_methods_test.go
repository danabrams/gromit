//go:build acceptance

package provider

import (
	"strings"
	"testing"
)

// TestClaudeProviderIsValidationPassedHelper verifies that ClaudeProvider
// has an IsValidationPassed() helper that delegates to claude.IsValidationPassed().
// Expected failure: IsValidationPassed() method does not exist on ClaudeProvider
func TestClaudeProviderIsValidationPassedHelper(t *testing.T) {
	cp := &ClaudeProvider{}

	tests := []struct {
		name     string
		result   *Result
		expected bool
	}{
		{
			name: "successful validation with VALIDATION_PASSED marker",
			result: &Result{
				Success:  true,
				ExitCode: 0,
				Output:   "All tests passed.\nVALIDATION_PASSED\nCompleted successfully.",
			},
			expected: true,
		},
		{
			name: "failed validation without marker",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "Test failed: some error",
			},
			expected: false,
		},
		{
			name: "successful run but no VALIDATION_PASSED marker",
			result: &Result{
				Success:  true,
				ExitCode: 0,
				Output:   "Tests completed but marker missing",
			},
			expected: false,
		},
		{
			name:     "nil result returns false",
			result:   nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cp.IsValidationPassed(tt.result)
			if got != tt.expected {
				t.Errorf("IsValidationPassed() = %v, want %v (output=%q)", got, tt.expected, tt.result.Output)
			}
		})
	}
}

// TestClaudeProviderIsScopeTooLargeHelper verifies that ClaudeProvider
// has an IsScopeTooLarge() helper that delegates to claude.IsScopeTooLarge().
// Expected failure: IsScopeTooLarge() method does not exist on ClaudeProvider
func TestClaudeProviderIsScopeTooLargeHelper(t *testing.T) {
	cp := &ClaudeProvider{}

	tests := []struct {
		name                string
		result              *Result
		expectedTooLarge    bool
		expectedExplanation string
	}{
		{
			name: "scope too large with explanation",
			result: &Result{
				Success: true,
				Output:  "Analysis:\nSCOPE_TOO_LARGE: This task touches 8 files across 4 packages and requires architectural changes.\n\nMore details...",
			},
			expectedTooLarge:    true,
			expectedExplanation: "This task touches 8 files across 4 packages and requires architectural changes.",
		},
		{
			name: "scope too large at start of line",
			result: &Result{
				Success: true,
				Output:  "\nSCOPE_TOO_LARGE: Too many files to modify in one bead.\nBreakdown: file1.go, file2.go",
			},
			expectedTooLarge:    true,
			expectedExplanation: "Too many files to modify in one bead.",
		},
		{
			name: "scope acceptable - no marker",
			result: &Result{
				Success: true,
				Output:  "This task looks good. The scope is appropriate for a single bead.",
			},
			expectedTooLarge:    false,
			expectedExplanation: "",
		},
		{
			name: "marker in middle of line - not detected",
			result: &Result{
				Success: true,
				Output:  "The pattern SCOPE_TOO_LARGE: should be at the start of a line to trigger.",
			},
			expectedTooLarge:    false,
			expectedExplanation: "",
		},
		{
			name:                "nil result returns false",
			result:              nil,
			expectedTooLarge:    false,
			expectedExplanation: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTooLarge, gotExplanation := cp.IsScopeTooLarge(tt.result)
			if gotTooLarge != tt.expectedTooLarge {
				t.Errorf("IsScopeTooLarge() tooLarge = %v, want %v", gotTooLarge, tt.expectedTooLarge)
			}
			if tt.expectedTooLarge && !strings.Contains(gotExplanation, tt.expectedExplanation) {
				t.Errorf("IsScopeTooLarge() explanation = %q, want it to contain %q", gotExplanation, tt.expectedExplanation)
			}
		})
	}
}
