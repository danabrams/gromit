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
// It uses two detection paths to handle different failure modes:
//  1. Rate limit hits from stream events - when the CLI observes rate limit events during
//     execution and ultimately fails, even if the error message is generic
//  2. Keyword match in terminal output - when the CLI fails with a usage-limit-related
//     error message in stdout/stderr (case-insensitive matching)
//
// Both paths require a non-zero exit code to avoid false positives on successful invocations.
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
