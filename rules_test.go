package gromit

import (
	"os"
	"strings"
	"testing"
)

// TestRULES_ContainsVisionAlignment verifies that RULES.md explicitly
// references VISION.md as the strategic alignment source before introducing rules.
func TestRULES_ContainsVisionAlignment(t *testing.T) {
	content, err := os.ReadFile("RULES.md")
	if err != nil {
		t.Fatalf("failed to read RULES.md: %v", err)
	}

	contentStr := string(content)

	// Check that RULES.md contains a reference to VISION.md
	if !strings.Contains(contentStr, "VISION.md") {
		t.Error("RULES.md must contain a reference to VISION.md")
	}

	// Check that RULES.md mentions alignment or strategic
	if !strings.Contains(contentStr, "alignment") && !strings.Contains(contentStr, "strategic") {
		t.Error("RULES.md must reference strategic alignment guidance from VISION.md")
	}

	// Check that the reference appears early (before first rule section like "## Architecture")
	visionIdx := strings.Index(contentStr, "VISION.md")
	architectureIdx := strings.Index(contentStr, "## Architecture")

	if visionIdx > architectureIdx || visionIdx == -1 {
		t.Error("Reference to VISION.md must appear before the first rule section (## Architecture)")
	}
}
