package provider

import (
	"testing"
)

// TestClassifyFailureWithPatterns verifies that the shared classifyFailure helper
// correctly classifies failures using provider-specific patterns.
// Expected failure: classifyFailure function does not exist yet
func TestClassifyFailureWithPatterns(t *testing.T) {
	t.Parallel()

	authPatterns := []string{
		"unauthorized",
		"invalid api key",
		"authentication",
		"forbidden",
	}
	startupPatterns := []string{
		"failed to start",
		"startup",
	}
	transportPatterns := []string{
		"connection reset",
		"timeout",
	}
	ratePatterns := []string{
		"rate limit",
		"quota exceeded",
	}

	tests := []struct {
		name     string
		exitCode int
		text     string
		want     string
	}{
		{
			name:     "exit code 0 returns none",
			exitCode: 0,
			text:     "any text",
			want:     FailureCategoryNone,
		},
		{
			name:     "empty text returns other",
			exitCode: 1,
			text:     "",
			want:     FailureCategoryOther,
		},
		{
			name:     "auth pattern matches",
			exitCode: 1,
			text:     "unauthorized access",
			want:     FailureCategoryAuth,
		},
		{
			name:     "startup pattern matches",
			exitCode: 1,
			text:     "failed to start service",
			want:     FailureCategoryStartupError,
		},
		{
			name:     "transport pattern matches",
			exitCode: 1,
			text:     "connection reset by peer",
			want:     FailureCategoryTransportDisconnect,
		},
		{
			name:     "rate pattern matches",
			exitCode: 1,
			text:     "rate limit exceeded",
			want:     FailureCategoryRateLimited,
		},
		{
			name:     "unknown error returns other",
			exitCode: 1,
			text:     "something went wrong",
			want:     FailureCategoryOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			patterns := FailurePatterns{
				Auth:      authPatterns,
				Startup:   startupPatterns,
				Transport: transportPatterns,
				RateLimit: ratePatterns,
			}
			got := classifyFailure(tt.exitCode, tt.text, patterns)
			if got != tt.want {
				t.Errorf("classifyFailure(%d, %q, patterns) = %q, want %q",
					tt.exitCode, tt.text, got, tt.want)
			}
		})
	}
}

func TestClassifyFailureWithCommonPatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		text          string
		extraPatterns FailurePatterns
		want          string
	}{
		{
			name: "base auth pattern",
			text: "forbidden access",
			want: FailureCategoryAuth,
		},
		{
			name: "provider-specific startup pattern",
			text: "failed to create stdin pipe",
			extraPatterns: FailurePatterns{
				Startup: []string{"failed to create stdin pipe"},
			},
			want: FailureCategoryStartupError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := classifyFailureWithCommonPatterns(1, tt.text, tt.extraPatterns)
			if got != tt.want {
				t.Fatalf("classifyFailureWithCommonPatterns(%q, extras) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
