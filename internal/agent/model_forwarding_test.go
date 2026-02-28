package agent

import (
	"testing"
)

// TestForwardModelToAgent_CodexPreset verifies model forwarding for codex preset agent
func TestForwardModelToAgent_CodexPreset(t *testing.T) {
	// Create a codex agent (preset)
	codexAgent := resolveCodexPreset()

	// Forward model to codex agent
	resultAgent, warning := ForwardModelToAgent(codexAgent, "gpt-4-codex")

	// Should not have a warning for known preset
	if warning != "" {
		t.Errorf("ForwardModelToAgent(codex, model) returned warning: %q, want empty", warning)
	}

	// Should return a non-nil agent
	if resultAgent == nil {
		t.Fatal("ForwardModelToAgent(codex, model) returned nil agent")
	}

	// Agent should still be functional
	if resultAgent.Name() != "codex" {
		t.Errorf("Agent name = %q, want %q", resultAgent.Name(), "codex")
	}
}
