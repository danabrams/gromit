package provider

import (
	"fmt"
	"strings"
)

// IsValidationPassed is a shared pure output-matching helper used by all providers.
// It detects successful validation by looking for the VALIDATION_PASSED marker.
func IsValidationPassed(result *Result) bool {
	if result == nil {
		return false
	}
	return result.Success && strings.Contains(result.Output, "VALIDATION_PASSED")
}

// IsScopeTooLarge is a shared pure output-matching helper.
// It detects SCOPE_TOO_LARGE at line start and returns its first-paragraph explanation.
func IsScopeTooLarge(result *Result) (bool, string) {
	if result == nil {
		return false, ""
	}

	idx := findStartOfLineMarker(result.Output)
	if idx == -1 {
		return false, ""
	}

	const marker = "SCOPE_TOO_LARGE:"
	remaining := result.Output[idx+len(marker):]
	explanation := strings.TrimSpace(remaining)

	// Extract first paragraph only
	if paragraphEnd := strings.Index(explanation, "\n\n"); paragraphEnd != -1 {
		explanation = explanation[:paragraphEnd]
	}

	// Join lines with spaces
	lines := strings.Split(explanation, "\n")
	var explanationLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			explanationLines = append(explanationLines, trimmed)
		}
	}

	explanation = strings.Join(explanationLines, " ")
	return true, explanation
}

// GetScopeTooLargeBreakdown returns all content after a SCOPE_TOO_LARGE line-start marker.
func GetScopeTooLargeBreakdown(result *Result) string {
	if result == nil {
		return ""
	}

	idx := findStartOfLineMarker(result.Output)
	if idx == -1 {
		return ""
	}

	const marker = "SCOPE_TOO_LARGE:"
	remaining := result.Output[idx+len(marker):]
	breakdown := strings.TrimSpace(remaining)
	return breakdown
}

// findStartOfLineMarker finds SCOPE_TOO_LARGE: only at the start of a line.
// Returns the index where the marker starts, or -1 if not found.
func findStartOfLineMarker(s string) int {
	const marker = "SCOPE_TOO_LARGE:"
	start := 0
	for {
		idx := strings.Index(s[start:], marker)
		if idx == -1 {
			return -1
		}
		abs := start + idx
		// Check if marker is at line start
		if abs == 0 || s[abs-1] == '\n' {
			return abs
		}
		start = abs + len(marker)
	}
}

// ValidateCommands validates that commands are safe and well-formed.
func ValidateCommands(commands []string) error {
	for i, cmd := range commands {
		if cmd == "" {
			return fmt.Errorf("command %d is empty", i+1)
		}
		if strings.Contains(cmd, "\n") || strings.Contains(cmd, "\r") {
			return fmt.Errorf("command %d must be a single line: %q", i+1, cmd)
		}
		if len(cmd) > 1024 {
			return fmt.Errorf("command %d exceeds maximum length of 1024 characters", i+1)
		}
	}
	return nil
}

// BuildValidationPrompt constructs a validation prompt with numbered commands.
func BuildValidationPrompt(commands []string, workDir string) string {
	var sb strings.Builder

	sb.WriteString("You are a validation assistant. Run the following numbered commands in the directory ")
	sb.WriteString(workDir)
	sb.WriteString(":\n\n```\n")

	for i, cmd := range commands {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, cmd))
	}

	sb.WriteString("```\n\n")
	sb.WriteString("If ALL commands succeed (exit code 0), output exactly:\nVALIDATION_PASSED\n\n")
	sb.WriteString("If ANY command fails, output exactly:\nVALIDATION_FAILED\n\n")
	sb.WriteString("Include the command output and error details in your response.\n")

	return sb.String()
}
