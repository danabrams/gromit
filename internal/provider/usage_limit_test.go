package provider

import (
	"testing"
)

// TestIsUsageLimitErrorChecksOutputAndStderr verifies that IsUsageLimitError
// checks both Output and Stderr for usage limit indicators.
// Expected failure: IsUsageLimitError implementations do not check Stderr yet
func TestIsUsageLimitErrorChecksOutputAndStderr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result *Result
		want   bool
	}{
		{
			name:   "usage limit in Output",
			result: &Result{Output: "usage limit exceeded", ExitCode: 2},
			want:   true,
		},
		{
			name:   "usage limit in Stderr",
			result: &Result{Stderr: "rate limit exceeded", ExitCode: 2},
			want:   true,
		},
		{
			name:   "quota exceeded in Output",
			result: &Result{Output: "quota exceeded", ExitCode: 2},
			want:   true,
		},
		{
			name:   "quota exceeded in Stderr",
			result: &Result{Stderr: "quota exceeded", ExitCode: 2},
			want:   true,
		},
		{
			name:   "no usage limit indicator",
			result: &Result{Output: "something else", ExitCode: 2},
			want:   false,
		},
		{
			name:   "usage limit but wrong exit code",
			result: &Result{Output: "usage limit exceeded", ExitCode: 1},
			want:   false,
		},
		{
			name:   "nil result",
			result: nil,
			want:   false,
		},
	}

	t.Run("Claude", func(t *testing.T) {
		t.Parallel()
		cp := &ClaudeProvider{}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got := cp.IsUsageLimitError(tt.result, nil)
				if got != tt.want {
					t.Errorf("IsUsageLimitError() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		gp := &GeminiProvider{}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got := gp.IsUsageLimitError(tt.result, nil)
				if got != tt.want {
					t.Errorf("IsUsageLimitError() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("Codex", func(t *testing.T) {
		t.Parallel()
		codexTests := []struct {
			name   string
			result *Result
			want   bool
		}{
			{
				name:   "usage limit in Output",
				result: &Result{Output: "usage limit exceeded", ExitCode: 1, Success: false},
				want:   true,
			},
			{
				name:   "usage limit in Stderr",
				result: &Result{Stderr: "rate limit exceeded", ExitCode: 1, Success: false},
				want:   true,
			},
			{
				name:   "quota exceeded in Output",
				result: &Result{Output: "quota exceeded", ExitCode: 1, Success: false},
				want:   true,
			},
			{
				name:   "quota exceeded in Stderr",
				result: &Result{Stderr: "quota exceeded", ExitCode: 1, Success: false},
				want:   true,
			},
			{
				name:   "no usage limit indicator",
				result: &Result{Output: "something else", ExitCode: 1, Success: false},
				want:   false,
			},
			{
				name:   "success result is not a usage limit error",
				result: &Result{Output: "usage limit exceeded", ExitCode: 0, Success: true},
				want:   false,
			},
			{
				name:   "nil result",
				result: nil,
				want:   false,
			},
		}
		cp := &CodexProvider{}
		for _, tt := range codexTests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got := cp.IsUsageLimitError(tt.result, nil)
				if got != tt.want {
					t.Errorf("IsUsageLimitError() = %v, want %v", got, tt.want)
				}
			})
		}
	})
}
