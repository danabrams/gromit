package runner

import (
	"regexp"
	"strings"
)

// maxValidationSummaryLen caps the length of extracted validation summaries.
const maxValidationSummaryLen = 500

// vetDiagnosticPattern matches go vet diagnostic lines like:
// ./file.go:10:6: x declared and not used
var vetDiagnosticPattern = regexp.MustCompile(`^\./[^:]+:\d+:\d+: .+`)

// extractValidationSummary extracts key error lines from go test/vet output.
// It returns test failure names (--- FAIL: lines), package failure lines (FAIL\t),
// and go vet diagnostic lines. The result is capped at 500 characters.
func extractValidationSummary(failureOutput string) string {
	if failureOutput == "" {
		return ""
	}

	var lines []string
	for _, line := range strings.Split(failureOutput, "\n") {
		if strings.HasPrefix(line, "--- FAIL:") {
			lines = append(lines, line)
		} else if strings.HasPrefix(line, "FAIL\t") {
			lines = append(lines, line)
		} else if vetDiagnosticPattern.MatchString(line) {
			lines = append(lines, line)
		}
	}

	result := strings.Join(lines, "\n")
	if len(result) > maxValidationSummaryLen {
		result = result[:maxValidationSummaryLen]
	}
	return result
}
