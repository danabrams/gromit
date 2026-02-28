package gromit

import (
	"os"
	"strings"
	"testing"
)

func TestVISION_ScopeBoundariesExist(t *testing.T) {
	content, err := os.ReadFile("VISION.md")
	if err != nil {
		t.Fatalf("failed to read VISION.md: %v", err)
	}

	contentStr := string(content)

	required := []string{
		"## Scope Boundaries",
		"Vision is not a playbook",
		"Vision is not an implementation plan",
		"Vision does not cover daily operational details",
	}

	for _, phrase := range required {
		if !strings.Contains(contentStr, phrase) {
			t.Errorf("VISION.md must describe scope boundary phrase %q", phrase)
		}
	}
}
