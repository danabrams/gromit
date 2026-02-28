package gromit

import (
	"os"
	"strings"
	"testing"
)

func TestVISION_MissionAndTwoYearTargetStateExist(t *testing.T) {
	content, err := os.ReadFile("VISION.md")
	if err != nil {
		t.Fatalf("failed to read VISION.md: %v", err)
	}

	contentStr := string(content)
	required := []string{
		"## Mission and Outcomes",
		"## Two-Year Target State",
		"Mission",
		"Outcomes",
	}

	for _, phrase := range required {
		if !strings.Contains(contentStr, phrase) {
			t.Errorf("VISION.md must include %q", phrase)
		}
	}
}
