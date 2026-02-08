package agent

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestResolveClaudePreset verifies that the claude preset uses config values and file_ref delivery
func TestResolveClaudePreset(t *testing.T) {
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary: "/usr/local/bin/claude",
			Flags:  []string{"--model", "opus"},
		},
	}

	agent, err := Resolve("claude", cfg)
	if err != nil {
		t.Fatalf("Resolve(claude) error = %v, want nil", err)
	}

	if agent == nil {
		t.Fatal("Resolve(claude) returned nil agent, want non-nil")
	}

	if agent.Name() != "claude" {
		t.Errorf("agent.Name() = %q, want %q", agent.Name(), "claude")
	}

	// Verify it's a cliAgent with correct configuration
	ca, ok := agent.(*cliAgent)
	if !ok {
		t.Fatal("Resolve(claude) should return *cliAgent")
	}

	if ca.binary != "/usr/local/bin/claude" {
		t.Errorf("claude agent binary = %q, want %q", ca.binary, "/usr/local/bin/claude")
	}

	if len(ca.flags) != 2 || ca.flags[0] != "--model" || ca.flags[1] != "opus" {
		t.Errorf("claude agent flags = %v, want [--model opus]", ca.flags)
	}

	if ca.promptDelivery != FileRef {
		t.Errorf("claude agent promptDelivery = %q, want %q", ca.promptDelivery, FileRef)
	}

	if ca.promptFlag != "" {
		t.Errorf("claude agent promptFlag = %q, want empty string", ca.promptFlag)
	}
}

// TestResolveClaudePresetWithDefaults verifies claude preset works when config has no explicit values
func TestResolveClaudePresetWithDefaults(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	agent, err := Resolve("claude", cfg)
	if err != nil {
		t.Fatalf("Resolve(claude) error = %v, want nil", err)
	}

	ca, ok := agent.(*cliAgent)
	if !ok {
		t.Fatal("Resolve(claude) should return *cliAgent")
	}

	// Should use default binary from config
	if ca.binary != "claude" {
		t.Errorf("claude agent binary = %q, want %q (config default)", ca.binary, "claude")
	}

	if ca.promptDelivery != FileRef {
		t.Errorf("claude agent promptDelivery = %q, want %q", ca.promptDelivery, FileRef)
	}
}

// TestResolveCodexPreset verifies codex preset uses prompt_file_arg delivery with --prompt flag
func TestResolveCodexPreset(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	agent, err := Resolve("codex", cfg)
	if err != nil {
		t.Fatalf("Resolve(codex) error = %v, want nil", err)
	}

	if agent == nil {
		t.Fatal("Resolve(codex) returned nil agent, want non-nil")
	}

	if agent.Name() != "codex" {
		t.Errorf("agent.Name() = %q, want %q", agent.Name(), "codex")
	}

	ca, ok := agent.(*cliAgent)
	if !ok {
		t.Fatal("Resolve(codex) should return *cliAgent")
	}

	if ca.binary != "codex" {
		t.Errorf("codex agent binary = %q, want %q", ca.binary, "codex")
	}

	if ca.promptDelivery != PromptFileArg {
		t.Errorf("codex agent promptDelivery = %q, want %q", ca.promptDelivery, PromptFileArg)
	}

	if ca.promptFlag != "--prompt" {
		t.Errorf("codex agent promptFlag = %q, want %q", ca.promptFlag, "--prompt")
	}
}

// TestResolveGeminiPreset verifies gemini preset uses prompt_file_arg delivery with --prompt flag
func TestResolveGeminiPreset(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	agent, err := Resolve("gemini", cfg)
	if err != nil {
		t.Fatalf("Resolve(gemini) error = %v, want nil", err)
	}

	if agent == nil {
		t.Fatal("Resolve(gemini) returned nil agent, want non-nil")
	}

	if agent.Name() != "gemini" {
		t.Errorf("agent.Name() = %q, want %q", agent.Name(), "gemini")
	}

	ca, ok := agent.(*cliAgent)
	if !ok {
		t.Fatal("Resolve(gemini) should return *cliAgent")
	}

	if ca.binary != "gemini" {
		t.Errorf("gemini agent binary = %q, want %q", ca.binary, "gemini")
	}

	if ca.promptDelivery != PromptFileArg {
		t.Errorf("gemini agent promptDelivery = %q, want %q", ca.promptDelivery, PromptFileArg)
	}

	if ca.promptFlag != "--prompt" {
		t.Errorf("gemini agent promptFlag = %q, want %q", ca.promptFlag, "--prompt")
	}
}

// TestResolveCustomAgent verifies custom agents defined in config can be resolved
func TestResolveCustomAgent(t *testing.T) {
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Definitions: map[string]config.AgentDefinition{
				"my-tool": {
					Binary: "/usr/local/bin/my-agent",
					Flags:  []string{"--verbose", "--mode=test"},
				},
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	agent, err := Resolve("my-tool", cfg)
	if err != nil {
		t.Fatalf("Resolve(my-tool) error = %v, want nil", err)
	}

	if agent == nil {
		t.Fatal("Resolve(my-tool) returned nil agent, want non-nil")
	}

	if agent.Name() != "my-tool" {
		t.Errorf("agent.Name() = %q, want %q", agent.Name(), "my-tool")
	}

	ca, ok := agent.(*cliAgent)
	if !ok {
		t.Fatal("Resolve(my-tool) should return *cliAgent")
	}

	if ca.binary != "/usr/local/bin/my-agent" {
		t.Errorf("my-tool agent binary = %q, want %q", ca.binary, "/usr/local/bin/my-agent")
	}

	if len(ca.flags) != 2 || ca.flags[0] != "--verbose" || ca.flags[1] != "--mode=test" {
		t.Errorf("my-tool agent flags = %v, want [--verbose --mode=test]", ca.flags)
	}

	// Custom agents should default to prompt_file_arg delivery
	if ca.promptDelivery != PromptFileArg {
		t.Errorf("my-tool agent promptDelivery = %q, want %q (default)", ca.promptDelivery, PromptFileArg)
	}

	if ca.promptFlag != "--prompt" {
		t.Errorf("my-tool agent promptFlag = %q, want %q (default)", ca.promptFlag, "--prompt")
	}
}

// TestResolveCustomAgentOverridesBuiltin verifies custom definition overrides built-in preset
func TestResolveCustomAgentOverridesBuiltin(t *testing.T) {
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary: "/usr/local/bin/claude",
			Flags:  []string{"--model", "opus"},
		},
		Agents: config.AgentsConfig{
			Definitions: map[string]config.AgentDefinition{
				"claude": {
					Binary: "/custom/claude-wrapper",
					Flags:  []string{"--custom-flag"},
				},
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	agent, err := Resolve("claude", cfg)
	if err != nil {
		t.Fatalf("Resolve(claude) error = %v, want nil", err)
	}

	ca, ok := agent.(*cliAgent)
	if !ok {
		t.Fatal("Resolve(claude) should return *cliAgent")
	}

	// Should use custom definition, not built-in preset
	if ca.binary != "/custom/claude-wrapper" {
		t.Errorf("claude agent binary = %q, want %q (custom override)", ca.binary, "/custom/claude-wrapper")
	}

	if len(ca.flags) != 1 || ca.flags[0] != "--custom-flag" {
		t.Errorf("claude agent flags = %v, want [--custom-flag] (custom override)", ca.flags)
	}
}

// TestResolveUnknownAgent verifies Resolve returns error for unknown agent names
func TestResolveUnknownAgent(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	agent, err := Resolve("nonexistent-agent", cfg)
	if err == nil {
		t.Error("Resolve(nonexistent-agent) error = nil, want error")
	}

	if agent != nil {
		t.Errorf("Resolve(nonexistent-agent) returned agent %v, want nil", agent)
	}

	// Error message should indicate the agent is unknown
	if err != nil && err.Error() == "" {
		t.Error("Resolve(nonexistent-agent) returned empty error message")
	}
}

// TestResolveWithNilConfig verifies Resolve handles nil config gracefully
func TestResolveWithNilConfig(t *testing.T) {
	// Built-in presets should work without config (using defaults)
	agent, err := Resolve("codex", nil)
	if err != nil {
		t.Fatalf("Resolve(codex, nil) error = %v, want nil", err)
	}

	if agent == nil {
		t.Fatal("Resolve(codex, nil) returned nil agent, want non-nil")
	}

	ca, ok := agent.(*cliAgent)
	if !ok {
		t.Fatal("Resolve(codex, nil) should return *cliAgent")
	}

	if ca.binary != "codex" {
		t.Errorf("codex agent binary = %q, want %q (built-in default)", ca.binary, "codex")
	}
}

// TestResolveEmptyAgentName verifies Resolve returns error for empty agent name
func TestResolveEmptyAgentName(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	agent, err := Resolve("", cfg)
	if err == nil {
		t.Error("Resolve(\"\") error = nil, want error")
	}

	if agent != nil {
		t.Errorf("Resolve(\"\") returned agent %v, want nil", agent)
	}
}

// TestResolvePreservesExtraArgs verifies that custom agents with extra config are supported
func TestResolveCustomAgentWithEmptyBinary(t *testing.T) {
	// Custom agent with no binary should get a sensible default (agent name as binary)
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Definitions: map[string]config.AgentDefinition{
				"my-agent": {
					Binary: "", // Empty binary
					Flags:  []string{"--flag"},
				},
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	agent, err := Resolve("my-agent", cfg)
	if err != nil {
		t.Fatalf("Resolve(my-agent) error = %v, want nil", err)
	}

	ca, ok := agent.(*cliAgent)
	if !ok {
		t.Fatal("Resolve(my-agent) should return *cliAgent")
	}

	// Should default to agent name as binary
	if ca.binary != "my-agent" {
		t.Errorf("my-agent agent binary = %q, want %q (default to agent name)", ca.binary, "my-agent")
	}
}

// TestResolveBuiltInPresetsExist verifies all three built-in presets are available
func TestResolveBuiltInPresetsExist(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	presets := []string{"claude", "codex", "gemini"}

	for _, preset := range presets {
		t.Run(preset, func(t *testing.T) {
			agent, err := Resolve(preset, cfg)
			if err != nil {
				t.Errorf("Resolve(%s) error = %v, want nil", preset, err)
			}

			if agent == nil {
				t.Errorf("Resolve(%s) returned nil agent, want non-nil", preset)
			}

			if agent != nil && agent.Name() != preset {
				t.Errorf("agent.Name() = %q, want %q", agent.Name(), preset)
			}
		})
	}
}

// TestResolvePreservesFlagsOrder verifies that agent flags are preserved in order
func TestResolvePreservesFlagsOrder(t *testing.T) {
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Definitions: map[string]config.AgentDefinition{
				"ordered": {
					Binary: "test",
					Flags:  []string{"--first", "--second", "--third", "--fourth"},
				},
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	agent, err := Resolve("ordered", cfg)
	if err != nil {
		t.Fatalf("Resolve(ordered) error = %v, want nil", err)
	}

	ca, ok := agent.(*cliAgent)
	if !ok {
		t.Fatal("Resolve(ordered) should return *cliAgent")
	}

	expectedFlags := []string{"--first", "--second", "--third", "--fourth"}
	if len(ca.flags) != len(expectedFlags) {
		t.Fatalf("flags length = %d, want %d", len(ca.flags), len(expectedFlags))
	}

	for i, expected := range expectedFlags {
		if ca.flags[i] != expected {
			t.Errorf("flags[%d] = %q, want %q", i, ca.flags[i], expected)
		}
	}
}
