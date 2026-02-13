package runner

import "github.com/danabrams/gromit/internal/runner/validation"

// extractValidationSummary delegates to validation.ExtractValidationSummary.
// Kept for backward compatibility with existing tests in the runner package.
func extractValidationSummary(failureOutput string) string {
	return validation.ExtractValidationSummary(failureOutput)
}
