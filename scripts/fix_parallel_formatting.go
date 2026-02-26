package main

import (
	"fmt"
	"os"
	"strings"
)

// FixParallelFormatting corrects malformed t.Parallel( patterns
// where comments appear inside the parentheses
func FixParallelFormatting() error {
	testFiles := []string{
		"cmd/gromit/chain_integration_test.go",
		"cmd/gromit/adapter_integration_typed_test.go",
		"internal/bead/bead_test.go",
		"internal/bead/closed_bead_filter_test.go",
		"internal/runner/reviewpkg/post_success_review_test.go",
	}

	for _, testFile := range testFiles {
		if err := fixFile(testFile); err != nil {
			return fmt.Errorf("failed to fix %s: %w", testFile, err)
		}
	}

	return nil
}

func fixFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	fixed := fixLines(lines)

	// Write back the fixed content
	if err := os.WriteFile(filePath, []byte(strings.Join(fixed, "\n")), 0644); err != nil {
		return err
	}

	return nil
}

func fixLines(lines []string) []string {
	var result []string
	i := 0

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check for malformed t.Parallel( pattern
		if strings.HasPrefix(trimmed, "t.Parallel(") && !strings.HasPrefix(trimmed, "t.Parallel()") {
			// Get the indentation
			indent := getIndentation(line)

			// t.Parallel() goes on this line
			result = append(result, indent+"t.Parallel()")

			// Look for comment lines that follow
			i++
			for i < len(lines) {
				nextLine := lines[i]
				nextTrimmed := strings.TrimSpace(nextLine)

				// If it's a comment, add it with the same indentation
				if strings.HasPrefix(nextTrimmed, "//") {
					result = append(result, indent+nextTrimmed)
					i++
					// After comment, check for closing parenthesis
					if i < len(lines) {
						checkLine := lines[i]
						checkTrimmed := strings.TrimSpace(checkLine)
						if checkTrimmed == ")" {
							// Skip the closing parenthesis line
							i++
							break
						}
					}
				} else if nextTrimmed == ")" {
					// Just a closing parenthesis, skip it
					i++
					break
				} else {
					// Not a comment or closing paren, stop processing
					break
				}
			}
		} else {
			result = append(result, line)
			i++
		}
	}

	// Remove excessive blank lines created by the fix
	result = cleanupBlankLines(result)

	return result
}

func getIndentation(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	return ""
}

// cleanupBlankLines removes excessive consecutive blank lines (more than 1)
func cleanupBlankLines(lines []string) []string {
	var result []string
	blankCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			blankCount++
			if blankCount <= 1 {
				result = append(result, line)
			}
		} else {
			blankCount = 0
			result = append(result, line)
		}
	}

	return result
}

func main() {
	if err := FixParallelFormatting(); err != nil {
		fmt.Fprintf(os.Stderr, "Error fixing parallel formatting: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Successfully fixed t.Parallel formatting across all test files")
}
