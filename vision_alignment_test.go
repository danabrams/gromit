package gromit

import (
	"os"
	"strings"
	"testing"
)

func TestVISION_AlignmentAssessmentMethodExists(t *testing.T) {
	content, err := os.ReadFile("VISION.md")
	if err != nil {
		t.Fatalf("failed to read VISION.md: %v", err)
	}

	contentStr := string(content)

	required := []string{
		"## Alignment Assessment Method",
		"Alignment assessment method for rules, learnings, or spec changes",
		"Step 1:",
		"Step 2:",
		"Step 3:",
	}

	for _, phrase := range required {
		if !strings.Contains(contentStr, phrase) {
			t.Errorf("VISION.md must include alignment assessment phrase %q", phrase)
		}
	}
}
