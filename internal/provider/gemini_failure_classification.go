package provider

func classifyGeminiFailure(exitCode int, stderr string) string {
	extras := FailurePatterns{
		Startup: []string{"initialization"},
	}
	return classifyFailureWithCommonPatterns(exitCode, stderr, extras)
}
