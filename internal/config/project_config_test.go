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

	t.Run("has_model_timeouts_section", func(t *testing.T) {
		if !strings.Contains(text, "model_timeouts:") {
			t.Error("gromit.yaml missing model_timeouts section")
		}
	})

	t.Run("documents_sonnet_configuration", func(t *testing.T) {
		if !strings.Contains(text, "sonnet:") {
			t.Error("gromit.yaml missing sonnet timeout configuration")
		}
	})

	t.Run("documents_opus_configuration", func(t *testing.T) {
		if !strings.Contains(text, "opus:") {
			t.Error("gromit.yaml missing opus timeout configuration")
		}
	})

	t.Run("includes_rationale_comments", func(t *testing.T) {
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
	})
}
