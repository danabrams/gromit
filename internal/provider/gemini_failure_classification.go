package provider

func classifyGeminiFailure(exitCode int, stderr string) string {
	patterns := FailurePatterns{
		Auth: []string{
			"unauthorized",
			"invalid api key",
			"authentication",
			"forbidden",
		},
		Startup: []string{
			"failed to start",
			"initialization",
			"startup",
		},
		Transport: []string{
			"connection reset",
			"connection refused",
			"connection timed out",
			"timeout",
			"service unavailable",
			"broken pipe",
			"could not resolve host",
			"temporary failure",
			"internal server error",
		},
		RateLimit: []string{
			"rate limit",
			"quota exceeded",
			"too many requests",
			"429",
			"503",
		},
	}
	return classifyFailure(exitCode, stderr, patterns)
}
