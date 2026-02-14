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

// TestGromitYamlDocumentsCodexProvider verifies that the reference gromit.yaml
// includes a commented-out Codex provider configuration example showing how to
// configure Codex as an alternative provider.
func TestGromitYamlDocumentsCodexProvider(t *testing.T) {
	content, err := os.ReadFile("../../gromit.yaml")
	if err != nil {
		t.Fatalf("failed to read gromit.yaml: %v", err)
	}

	text := string(content)

	t.Run("has_commented_codex_provider", func(t *testing.T) {
		// Look for a commented-out codex provider configuration
		// The exact format should be: # codex:
		if !strings.Contains(text, "# codex:") {
			t.Error("gromit.yaml missing commented-out Codex provider configuration example")
		}
	})

	t.Run("documents_codex_binary", func(t *testing.T) {
		// The example should show binary: codex
		if !strings.Contains(text, "#     binary: codex") {
			t.Error("gromit.yaml missing Codex binary configuration in example")
		}
	})

	t.Run("documents_codex_models", func(t *testing.T) {
		// The example should show model tier configuration using gpt-5.3-codex
		if !strings.Contains(text, "gpt-5.3-codex") {
			t.Error("gromit.yaml missing gpt-5.3-codex model in Codex provider example")
		}
	})

	t.Run("codex_example_positioned_after_routing", func(t *testing.T) {
		// Find the routing section
		routingIndex := strings.Index(text, "# Routing")
		if routingIndex == -1 {
			routingIndex = strings.Index(text, "routing:")
		}

		// Find the codex example
		codexIndex := strings.Index(text, "# codex:")

		if codexIndex == -1 {
			t.Skip("Codex example not found, skipping position test")
		}

		// Verify codex comes after routing (around line 30)
		if codexIndex < routingIndex {
			t.Error("Codex provider example should be positioned after routing section")
		}
	})
}
