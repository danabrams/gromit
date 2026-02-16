package provider

import (
	"testing"
)

// TestClaudeProviderIsUsageLimitErrorDetection verifies that
// IsUsageLimitError() detects Claude-specific usage limit errors
// with exit code 2 and specific error messages (case-insensitive).
// Expected failure: IsUsageLimitError() always returns false currently,
// but should detect exit code 2 + "usage limit"/"rate limit"/"quota exceeded" patterns
func TestClaudeProviderIsUsageLimitErrorDetection(t *testing.T) {
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
			expected: false, // Wrong exit code - must be 2
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
		{
			name: "exit code 0 with usage limit in output - not an error",
			result: &Result{
				Success:  true,
				ExitCode: 0,
				Output:   "The system checks for usage limit.",
			},
			err:      nil,
			expected: false, // Success means no error
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

// TestClaudeProviderIsUsageLimitErrorExitCodeRequired verifies that
// IsUsageLimitError() REQUIRES exit code 2 - other exit codes should not
// be treated as usage limit errors even if the message contains keywords.
// Expected failure: current implementation always returns false, should check exit code 2
func TestClaudeProviderIsUsageLimitErrorExitCodeRequired(t *testing.T) {
	cp := &ClaudeProvider{}

	// Test all non-2 exit codes with usage limit message
	exitCodes := []int{0, 1, 3, 127, 255}

	for _, exitCode := range exitCodes {
		t.Run(string(rune('0'+exitCode)), func(t *testing.T) {
			result := &Result{
				Success:  false,
				ExitCode: exitCode,
				Output:   "Error: usage limit exceeded",
			}

			got := cp.IsUsageLimitError(result, nil)
			if got {
				t.Errorf("IsUsageLimitError() with exit code %d = true, want false (only exit code 2 should trigger)", exitCode)
			}
		})
	}
}

// TestClaudeProviderIsUsageLimitErrorKeywordRequired verifies that
// IsUsageLimitError() REQUIRES one of the usage limit keywords,
// not just exit code 2 alone.
// Expected failure: current implementation always returns false, should check for keywords
func TestClaudeProviderIsUsageLimitErrorKeywordRequired(t *testing.T) {
	cp := &ClaudeProvider{}

	nonLimitErrors := []string{
		"Error: invalid input",
		"Error: network timeout",
		"Error: authentication failed",
		"Error: file not found",
		"Compilation error",
		"Runtime panic",
	}

	for _, errorMsg := range nonLimitErrors {
		t.Run(errorMsg, func(t *testing.T) {
			result := &Result{
				Success:  false,
				ExitCode: 2, // Correct exit code but wrong message
				Output:   errorMsg,
			}

			got := cp.IsUsageLimitError(result, nil)
			if got {
				t.Errorf("IsUsageLimitError() with message %q = true, want false (no usage limit keywords)", errorMsg)
			}
		})
	}
}
