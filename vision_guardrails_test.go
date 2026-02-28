package gromit

import (
	"os"
	"strings"
	"testing"
)

func TestVISION_GuardrailsIncludeDescriptions(t *testing.T) {
	content, err := os.ReadFile("VISION.md")
	if err != nil {
		t.Fatalf("failed to read VISION.md: %v", err)
	}

	contentStr := string(content)

	required := []string{
		"Non-negotiable guardrail: Safety -",
		"Non-negotiable guardrail: Matching intent -",
		"Non-negotiable guardrail: Ability to continue evolving the system -",
		"Non-negotiable guardrail: Cost efficiency -",
		"Non-negotiable guardrail: Documented verifiability -",
	}

	for _, phrase := range required {
		if !strings.Contains(contentStr, phrase) {
			t.Errorf("VISION.md must describe %s", phrase)
		}
	}
}
