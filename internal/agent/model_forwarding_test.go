package agent

import (
	"os"
	"path/filepath"
	"strings"
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

// TestForwardModelToAgent_GeminiPreset verifies model forwarding for gemini preset agent
func TestForwardModelToAgent_GeminiPreset(t *testing.T) {
	// Create a gemini agent (preset)
	geminiAgent := resolveGeminiPreset()

	// Forward model to gemini agent
	resultAgent, warning := ForwardModelToAgent(geminiAgent, "gemini-2.5-pro")

	// Should not have a warning for known preset
	if warning != "" {
		t.Errorf("ForwardModelToAgent(gemini, model) returned warning: %q, want empty", warning)
	}

	// Should return a non-nil agent
	if resultAgent == nil {
		t.Fatal("ForwardModelToAgent(gemini, model) returned nil agent")
	}

	// Agent should still be functional
	if resultAgent.Name() != "gemini" {
		t.Errorf("Agent name = %q, want %q", resultAgent.Name(), "gemini")
	}
}

// TestForwardModelToAgent_CustomAgent verifies warning fallback for custom agents
func TestForwardModelToAgent_CustomAgent(t *testing.T) {
	// Create a custom agent
	customAgent := New("my-custom-agent", "my-binary", nil, FileRef, "", nil)

	// Forward model to custom agent
	resultAgent, warning := ForwardModelToAgent(customAgent, "some-model")

	// Should return warning for custom agents
	if warning == "" {
		t.Error("ForwardModelToAgent(custom, model) returned empty warning, want non-empty")
	}

	// Should return the original agent unchanged
	if resultAgent != customAgent {
		t.Error("ForwardModelToAgent(custom, model) returned different agent, want original")
	}

	// Agent should still be functional
	if resultAgent.Name() != "my-custom-agent" {
		t.Errorf("Agent name = %q, want %q", resultAgent.Name(), "my-custom-agent")
	}
}

// TestForwardModelToAgent_EmptyModel verifies no forwarding when model is empty
func TestForwardModelToAgent_EmptyModel(t *testing.T) {
	// Create a codex agent
	codexAgent := resolveCodexPreset()

	// Forward empty model string
	resultAgent, warning := ForwardModelToAgent(codexAgent, "")

	// Should not have a warning when model is empty
	if warning != "" {
		t.Errorf("ForwardModelToAgent(codex, \"\") returned warning: %q, want empty", warning)
	}

	// Should return the original agent unchanged
	if resultAgent != codexAgent {
		t.Error("ForwardModelToAgent(codex, \"\") returned different agent, want original")
	}
}

// TestForwardModelToAgent_ClaudePreset verifies warning for claude preset agent
func TestForwardModelToAgent_ClaudePreset(t *testing.T) {
	// Create a claude agent (preset)
	claudeAgent := resolveClaudePreset(nil)

	// Forward model to claude agent
	resultAgent, warning := ForwardModelToAgent(claudeAgent, "opus-4.6")

	// Should return warning for claude (model is handled elsewhere)
	if warning == "" {
		t.Error("ForwardModelToAgent(claude, model) returned empty warning, want non-empty")
	}

	// Should return the original agent unchanged
	if resultAgent != claudeAgent {
		t.Error("ForwardModelToAgent(claude, model) returned different agent, want original")
	}

	// Agent should still be functional
	if resultAgent.Name() != "claude" {
		t.Errorf("Agent name = %q, want %q", resultAgent.Name(), "claude")
	}
}

// TestForwardModelToAgent_CodexIncludesModelInCommand verifies model flag in command args
func TestForwardModelToAgent_CodexIncludesModelInCommand(t *testing.T) {
	// Create codex agent and forward model
	codexAgent := resolveCodexPreset()
	modifiedAgent, warning := ForwardModelToAgent(codexAgent, "gpt-4-turbo")

	if warning != "" {
		t.Fatalf("ForwardModelToAgent failed: %v", warning)
	}

	// Create temp prompt file
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt"), 0644); err != nil {
		t.Fatalf("Failed to create temp prompt file: %v", err)
	}

	// Build command from modified agent
	cmd, err := modifiedAgent.Command(promptPath)
	if err != nil {
		t.Fatalf("Command() failed: %v", err)
	}

	// Verify --model flag and value are in args
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "--model") {
		t.Error("Command args missing --model flag")
	}
	if !strings.Contains(args, "gpt-4-turbo") {
		t.Error("Command args missing model value 'gpt-4-turbo'")
	}
}
