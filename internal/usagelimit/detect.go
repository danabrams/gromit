package usagelimit

import "strings"

// Signals represents the signals from a CLI invocation that may indicate a usage limit error.
type Signals struct {
	ExitCode      int
	Output        string
	RateLimitHits int
}

// Patterns represents the patterns to match against CLI output for usage limit detection.
type Patterns struct {
	Keywords []string
}

// Check returns true if the signals indicate a usage limit error based on the provided patterns.
// It uses two detection paths:
// 1. Non-zero exit code AND keyword match in output (case-insensitive)
// 2. Non-zero exit code AND rate limit hits > 0
func Check(signals Signals, patterns Patterns) bool {
	// Path 2: Rate limit hits with failed invocation
	if signals.ExitCode != 0 && signals.RateLimitHits > 0 {
		return true
	}

	// Path 1: Keyword match with non-zero exit code
	if signals.ExitCode != 0 && len(patterns.Keywords) > 0 {
		lowerOutput := strings.ToLower(signals.Output)
		for _, keyword := range patterns.Keywords {
			if strings.Contains(lowerOutput, strings.ToLower(keyword)) {
				return true
			}
		}
	}

	return false
}

// ClaudePatterns returns the keyword patterns for Claude CLI usage limit detection.
func ClaudePatterns() Patterns {
	return Patterns{
		Keywords: []string{
			"usage limit",
			"rate limit",
			"quota",
			"exceeded",
			"capacity",
			"overloaded",
			"too many requests",
			"429",
		},
	}
}

// CodexPatterns returns the keyword patterns for Codex CLI usage limit detection.
func CodexPatterns() Patterns {
	return Patterns{
		Keywords: []string{
			"usage limit",
			"rate limit",
			"quota",
			"exceeded",
		},
	}
}
