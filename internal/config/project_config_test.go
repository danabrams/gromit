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

func TestGromitYamlDocumentsMaxCrossRunFailures(t *testing.T) {
	content, err := os.ReadFile("../../gromit.yaml")
	if err != nil {
		t.Fatalf("failed to read gromit.yaml: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "max_cross_run_failures:") {
		t.Error("gromit.yaml missing loop.max_cross_run_failures setting")
	}
}

// Expected failure: gromit.yaml does not have a commented worktree section yet
// TestGromitYamlDocumentsWorktreeConfig verifies that the reference gromit.yaml
// includes a commented-out worktree configuration section showing how to enable
// concurrent interactive sessions via git worktrees.
func TestGromitYamlDocumentsWorktreeConfig(t *testing.T) {
	content, err := os.ReadFile("../../gromit.yaml")
	if err != nil {
		t.Fatalf("failed to read gromit.yaml: %v", err)
	}

	text := string(content)

	// Expected failure: gromit.yaml does not have "# Worktree" comment yet
	t.Run("has_worktree_comment_header", func(t *testing.T) {
		// Look for a comment header describing the worktree section
		if !strings.Contains(text, "# Worktree") && !strings.Contains(text, "# worktree") {
			t.Error("gromit.yaml missing Worktree section comment header")
		}
	})

	// Expected failure: gromit.yaml does not have commented worktree config yet
	t.Run("has_commented_worktree_config", func(t *testing.T) {
		// Look for commented-out worktree configuration
		if !strings.Contains(text, "# worktree:") {
			t.Error("gromit.yaml missing commented-out worktree configuration")
		}
	})

	// Expected failure: gromit.yaml does not have worktree enabled field yet
	t.Run("documents_enabled_field", func(t *testing.T) {
		// Should show enabled: true with a comment
		if !strings.Contains(text, "#   enabled:") {
			t.Error("gromit.yaml missing enabled field in worktree configuration")
		}
	})

	// Expected failure: gromit.yaml does not have worktree auto_merge field yet
	t.Run("documents_auto_merge_field", func(t *testing.T) {
		// Should show auto_merge: true with a comment
		if !strings.Contains(text, "#   auto_merge:") {
			t.Error("gromit.yaml missing auto_merge field in worktree configuration")
		}
	})

	// Expected failure: gromit.yaml does not have worktree merge_failure field yet
	t.Run("documents_merge_failure_field", func(t *testing.T) {
		// Should show merge_failure: "warn" with a comment
		if !strings.Contains(text, "#   merge_failure:") {
			t.Error("gromit.yaml missing merge_failure field in worktree configuration")
		}
	})

	// Expected failure: gromit.yaml worktree section doesn't exist to position
	t.Run("worktree_positioned_after_git", func(t *testing.T) {
		// Find the git section
		gitIndex := strings.Index(text, "# Git settings")
		if gitIndex == -1 {
			gitIndex = strings.Index(text, "git:")
		}

		// Find the worktree section
		worktreeIndex := strings.Index(text, "# worktree:")

		if worktreeIndex == -1 {
			t.Error("Worktree section not found in gromit.yaml")
			return
		}

		// Verify worktree comes after git section (logically related)
		if worktreeIndex < gitIndex {
			t.Error("Worktree configuration should be positioned after git section")
		}
	})
}
