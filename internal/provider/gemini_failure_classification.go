package provider

import "strings"

func classifyGeminiFailure(exitCode int, stderr string) string {
	if exitCode == 0 {
		return FailureCategoryNone
	}

	text := strings.ToLower(strings.TrimSpace(stderr))
	if text == "" {
		return FailureCategoryOther
	}

	authPatterns := []string{
		"unauthorized",
		"invalid api key",
		"authentication",
		"forbidden",
	}
	for _, p := range authPatterns {
		if strings.Contains(text, p) {
			return FailureCategoryAuth
		}
	}

	startupPatterns := []string{
		"failed to start",
		"initialization",
		"startup",
	}
	for _, p := range startupPatterns {
		if strings.Contains(text, p) {
			return FailureCategoryStartupError
		}
	}

	transportPatterns := []string{
		"connection reset",
		"connection refused",
		"connection timed out",
		"timeout",
		"service unavailable",
		"broken pipe",
		"could not resolve host",
		"temporary failure",
		"internal server error",
	}
	for _, p := range transportPatterns {
		if strings.Contains(text, p) {
			return FailureCategoryTransportDisconnect
		}
	}

	ratePatterns := []string{
		"rate limit",
		"quota exceeded",
		"too many requests",
		"429",
		"503",
	}
	for _, p := range ratePatterns {
		if strings.Contains(text, p) {
			return FailureCategoryRateLimited
		}
	}

	return FailureCategoryOther
}
