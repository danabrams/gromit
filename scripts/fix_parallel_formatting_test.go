package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestParallelFormattingMalformed detects malformed t.Parallel( patterns
// with comments inside parentheses
func TestParallelFormattingMalformed(t *testing.T) {
	testFiles := []string{
		"../cmd/gromit/chain_integration_test.go",
		"../cmd/gromit/adapter_integration_typed_test.go",
		"../internal/bead/bead_test.go",
		"../internal/bead/closed_bead_filter_test.go",
		"../internal/runner/reviewpkg/post_success_review_test.go",
	}

	// Pattern: t.Parallel( followed by anything other than ) on same line
	malformedPattern := regexp.MustCompile(`t\.Parallel\(\s*\n\s*//`)

	malformedCount := 0
	for _, testFile := range testFiles {
		absPath := filepath.Join(".", testFile)
		content, err := os.ReadFile(absPath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", testFile, err)
		}

		if malformedPattern.MatchString(string(content)) {
			matches := malformedPattern.FindAllString(string(content), -1)
			malformedCount += len(matches)
			t.Logf("Found %d malformed t.Parallel patterns in %s", len(matches), testFile)
		}
	}

	if malformedCount > 0 {
		t.Errorf("found %d malformed t.Parallel( patterns with comments inside parentheses", malformedCount)
	}
}

// TestParallelFormattingCorrected verifies that t.Parallel() is properly formatted
// with comments on the following line
func TestParallelFormattingCorrected(t *testing.T) {
	testFiles := []string{
		"../cmd/gromit/chain_integration_test.go",
		"../cmd/gromit/adapter_integration_typed_test.go",
		"../internal/bead/bead_test.go",
		"../internal/bead/closed_bead_filter_test.go",
		"../internal/runner/reviewpkg/post_success_review_test.go",
	}

	for _, testFile := range testFiles {
		absPath := filepath.Join(".", testFile)
		content, err := os.ReadFile(absPath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", testFile, err)
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Check for malformed pattern: t.Parallel( with comment
			if strings.HasPrefix(trimmed, "t.Parallel(") && !strings.HasPrefix(trimmed, "t.Parallel()") {
				// This line has t.Parallel( but not t.Parallel()
				// Check if the next line is a comment
				if i+1 < len(lines) {
					nextLine := strings.TrimSpace(lines[i+1])
					if strings.HasPrefix(nextLine, "//") {
						t.Errorf("%s:%d: malformed t.Parallel( with comment on next line: %s", testFile, i+1, trimmed)
					}
				}
			}
		}
	}
}
