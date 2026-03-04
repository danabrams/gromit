package provider

import "strings"

// FailurePatterns contains provider-specific patterns for classifying failures.
type FailurePatterns struct {
	Auth      []string
	Startup   []string
	Transport []string
	RateLimit []string
}

// classifyFailure classifies a failure based on exit code and text using provider-specific patterns.
// Returns FailureCategoryNone if exit code is 0, FailureCategoryOther if text is empty,
// or the appropriate category based on pattern matching.
func classifyFailure(exitCode int, text string, patterns FailurePatterns) string {
	if exitCode == 0 {
		return FailureCategoryNone
	}

	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return FailureCategoryOther
	}

	// Check auth patterns
	for _, p := range patterns.Auth {
		if strings.Contains(text, p) {
			return FailureCategoryAuth
		}
	}

	// Check startup patterns
	for _, p := range patterns.Startup {
		if strings.Contains(text, p) {
			return FailureCategoryStartupError
		}
	}

	// Check transport patterns
	for _, p := range patterns.Transport {
		if strings.Contains(text, p) {
			return FailureCategoryTransportDisconnect
		}
	}

	// Check rate limit patterns
	for _, p := range patterns.RateLimit {
		if strings.Contains(text, p) {
			return FailureCategoryRateLimited
		}
	}

	return FailureCategoryOther
}
