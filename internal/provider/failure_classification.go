package provider

import "strings"

// FailurePatterns contains provider-specific patterns for classifying failures.
type FailurePatterns struct {
	Auth      []string
	Startup   []string
	Transport []string
	RateLimit []string
}

var baseFailurePatterns = FailurePatterns{
	Auth: []string{
		"unauthorized",
		"invalid api key",
		"authentication",
		"forbidden",
	},
	Startup: []string{
		"failed to start",
		"startup",
		"initializ",
	},
	Transport: []string{
		"connection reset",
		"connection refused",
		"connection timed out",
		"timeout",
		"service unavailable",
		"broken pipe",
		"could not resolve host",
		"internal server error",
		"temporary failure",
	},
	RateLimit: []string{
		"rate limit",
		"too many requests",
		"quota exceeded",
		"429",
		"503",
	},
}

// hasUsageLimitKeywords checks if both output and stderr contain any usage limit keywords.
func hasUsageLimitKeywords(output, stderr string) bool {
	combined := strings.ToLower(output + "\n" + stderr)
	return containsAnyKeywordCaseInsensitive(combined, usageLimitKeywords)
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

func classifyFailureWithCommonPatterns(exitCode int, text string, extra FailurePatterns) string {
	return classifyFailure(exitCode, text, mergeFailurePatterns(baseFailurePatterns, extra))
}

func mergeFailurePatterns(base, extra FailurePatterns) FailurePatterns {
	return FailurePatterns{
		Auth:      append([]string{}, append(base.Auth, extra.Auth...)...),
		Startup:   append([]string{}, append(base.Startup, extra.Startup...)...),
		Transport: append([]string{}, append(base.Transport, extra.Transport...)...),
		RateLimit: append([]string{}, append(base.RateLimit, extra.RateLimit...)...),
	}
}
