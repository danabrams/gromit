package main

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// TestInitGoLogicFileIsShort verifies that init.go stays under 500 lines (template constants moved to separate file)
func TestInitGoLogicFileIsShort(t *testing.T) {
	t.Parallel()
	file, err := os.Open("init.go")
	if err != nil {
		t.Fatalf("failed to open init.go: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("error reading init.go: %v", err)
	}

	const maxLines = 500
	if lineCount > maxLines {
		t.Errorf("init.go has %d lines, expected <= %d lines (templates should be extracted)", lineCount, maxLines)
	}
}

// TestSeedProfileAwareCommandExamples_InjectsProfileGuidanceIntoValidateTemplate verifies that
// profile-aware command example notes are injected into templates when seeding for a specific profile
func TestSeedProfileAwareCommandExamples_InjectsProfileGuidanceIntoValidateTemplate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		profile          string
		wantGuidanceText string
	}{
		{
			profile:          "go",
			wantGuidanceText: "go test",
		},
		{
			profile:          "node",
			wantGuidanceText: "npm test",
		},
		{
			profile:          "python",
			wantGuidanceText: "pytest",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.profile, func(t *testing.T) {
			t.Parallel()

			seededTemplate := seedProfileAwareCommandExamples(tc.profile, defaultValidateTemplate)

			if !strings.Contains(seededTemplate, tc.wantGuidanceText) {
				t.Errorf("seeded template for profile %q missing expected guidance text %q in:\n%s",
					tc.profile, tc.wantGuidanceText, seededTemplate)
			}
		})
	}
}
