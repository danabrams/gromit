package provider

import "strings"

const (
	geminiErrorCategorySetup            = "setup/binary-missing"
	geminiErrorCategoryModelInvalid     = "model-invalid"
	geminiErrorCategoryPermissionDenied = "permission-denied"
	geminiErrorCategoryFallback         = "fallback"
)

type geminiCLIErrorClassification struct {
	Category  string
	Retryable bool
}

func classifyGeminiCLIError(stderr string) geminiCLIErrorClassification {
	lower := strings.ToLower(stderr)
	upper := strings.ToUpper(stderr)

	switch {
	case strings.Contains(lower, "command not found: gemini"):
		return geminiCLIErrorClassification{Category: geminiErrorCategorySetup, Retryable: false}
	case strings.Contains(lower, "permission denied"):
		return geminiCLIErrorClassification{Category: geminiErrorCategoryPermissionDenied, Retryable: false}
	case strings.Contains(lower, "modelnotfounderror") || strings.Contains(upper, "NOT_FOUND"):
		return geminiCLIErrorClassification{Category: geminiErrorCategoryModelInvalid, Retryable: false}
	default:
		return geminiCLIErrorClassification{Category: geminiErrorCategoryFallback, Retryable: true}
	}
}
