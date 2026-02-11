package config

import (
	"os"
	"strings"
	"testing"
)

// TestGromitYamlDocumentsModelTimeouts verifies that the reference gromit.yaml
// includes a properly commented model_timeouts section with recommended values.
func TestGromitYamlDocumentsModelTimeouts(t *testing.T) {
	content, err := os.ReadFile("../../gromit.yaml")
	if err != nil {
		t.Fatalf("failed to read gromit.yaml: %v", err)
	}

	text := string(content)

	// Verify model_timeouts section exists
	if !strings.Contains(text, "model_timeouts:") {
		t.Error("gromit.yaml missing model_timeouts section")
	}

	// Verify sonnet configuration is documented
	if !strings.Contains(text, "sonnet:") {
		t.Error("gromit.yaml missing sonnet timeout configuration")
	}

	// Verify opus configuration is documented
	if !strings.Contains(text, "opus:") {
		t.Error("gromit.yaml missing opus timeout configuration")
	}

	// Verify rationale comments exist for timeout adjustments
	expectedComments := []string{
		"# longer invocation timeout",
		"# longer active stall",
		"# longer bead budget",
	}

	for _, comment := range expectedComments {
		if !strings.Contains(text, comment) {
			t.Errorf("gromit.yaml missing expected rationale comment: %q", comment)
		}
	}
}
