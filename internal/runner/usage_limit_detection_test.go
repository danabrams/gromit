package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/usagelimit"
)

// TestUsageLimitDetection_KeywordMatch verifies that when Claude returns with
// exit code != 0 and stderr contains usage limit keywords, usage limit is detected.
func TestUsageLimitDetection_KeywordMatch(t *testing.T) {
	signals := usagelimit.Signals{
		ExitCode:      1,
		Output:        "Error: usage limit exceeded",
		RateLimitHits: 0,
	}

	patterns := usagelimit.ClaudePatterns()

	if !usagelimit.Check(signals, patterns) {
		t.Error("expected usage limit to be detected via keyword match")
	}
}

// TestUsageLimitDetection_RateLimitHits verifies that when rate limit hits > 0
// and invocation failed, usage limit is detected.
func TestUsageLimitDetection_RateLimitHits(t *testing.T) {
	signals := usagelimit.Signals{
		ExitCode:      1,
		Output:        "generic error message",
		RateLimitHits: 3,
	}

	patterns := usagelimit.ClaudePatterns()

	if !usagelimit.Check(signals, patterns) {
		t.Error("expected usage limit to be detected via rate limit hits")
	}
}

// TestUsageLimitDetection_NoFalsePositive verifies that normal build failures
// do not trigger usage limit detection.
func TestUsageLimitDetection_NoFalsePositive(t *testing.T) {
	signals := usagelimit.Signals{
		ExitCode:      1,
		Output:        "Error: tests failed\nFAIL: TestSomething",
		RateLimitHits: 0,
	}

	patterns := usagelimit.ClaudePatterns()

	if usagelimit.Check(signals, patterns) {
		t.Error("expected normal build failure NOT to be detected as usage limit")
	}
}

// TestUsageLimitDetection_SuccessNotDetected verifies that successful invocations
// with exit code 0 never trigger usage limit detection, even with keywords.
func TestUsageLimitDetection_SuccessNotDetected(t *testing.T) {
	signals := usagelimit.Signals{
		ExitCode:      0,
		Output:        "Success: processed despite earlier rate limit warnings",
		RateLimitHits: 2,
	}

	patterns := usagelimit.ClaudePatterns()

	if usagelimit.Check(signals, patterns) {
		t.Error("expected successful invocation NOT to be detected as usage limit")
	}
}
