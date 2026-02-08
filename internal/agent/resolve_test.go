package agent

import (
	"strings"
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

	agent, err := resolveByName("claude", cfg)
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

	agent, err := resolveByName("claude", cfg)
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

	agent, err := resolveByName("codex", cfg)
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

	agent, err := resolveByName("gemini", cfg)
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

	agent, err := resolveByName("my-tool", cfg)
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

	agent, err := resolveByName("claude", cfg)
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

	agent, err := resolveByName("nonexistent-agent", cfg)
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
	agent, err := resolveByName("codex", nil)
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

	agent, err := resolveByName("", cfg)
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

	agent, err := resolveByName("my-agent", cfg)
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
			agent, err := resolveByName(preset, cfg)
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

	agent, err := resolveByName("ordered", cfg)
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

// ========================================================================
// Acceptance tests for the new Resolve function with priority resolution
// ========================================================================

// TestResolveWithPriorityFlagOverride verifies flag override takes highest priority
func TestResolveWithPriorityFlagOverride(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *config.Config
		phase        string
		flagOverride string
		wantName     string
		wantErr      bool
	}{
		{
			name: "flag override beats phase config",
			cfg: &config.Config{
				Agents: config.AgentsConfig{
					Phases: config.PhasesConfig{
						Refine: "claude",
					},
				},
			},
			phase:        "refine",
			flagOverride: "codex",
			wantName:     "codex",
			wantErr:      false,
		},
		{
			name: "flag override beats default",
			cfg: &config.Config{
				Agents: config.AgentsConfig{
					Phases: config.PhasesConfig{},
				},
			},
			phase:        "refine",
			flagOverride: "gemini",
			wantName:     "gemini",
			wantErr:      false,
		},
		{
			name: "flag override with custom agent",
			cfg: &config.Config{
				Agents: config.AgentsConfig{
					Definitions: map[string]config.AgentDefinition{
						"my-agent": {
							Binary: "my-cli",
							Flags:  []string{"--custom"},
						},
					},
				},
			},
			phase:        "plan",
			flagOverride: "my-agent",
			wantName:     "my-agent",
			wantErr:      false,
		},
		{
			name:         "flag override with unknown agent returns error",
			cfg:          &config.Config{},
			phase:        "refine",
			flagOverride: "nonexistent",
			wantName:     "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg != nil {
				tt.cfg.SetDefaults()
				tt.cfg.NormalizeNilFields()
			}

			agent, err := Resolve(tt.cfg, tt.phase, tt.flagOverride, false, nil, nil)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Resolve() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("Resolve() error = %v, want nil", err)
			}

			if agent == nil {
				t.Fatal("Resolve() returned nil agent")
			}

			if agent.Name() != tt.wantName {
				t.Errorf("Agent.Name() = %q, want %q", agent.Name(), tt.wantName)
			}
		})
	}
}

// TestResolveWithPhaseConfigPriority verifies phase config is used when no flag override
func TestResolveWithPhaseConfigPriority(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		phase    string
		wantName string
	}{
		{
			name: "refine phase uses configured agent",
			cfg: &config.Config{
				Agents: config.AgentsConfig{
					Phases: config.PhasesConfig{
						Refine: "codex",
					},
				},
			},
			phase:    "refine",
			wantName: "codex",
		},
		{
			name: "plan phase uses configured agent",
			cfg: &config.Config{
				Agents: config.AgentsConfig{
					Phases: config.PhasesConfig{
						Plan: "gemini",
					},
				},
			},
			phase:    "plan",
			wantName: "gemini",
		},
		{
			name: "review phase uses configured agent",
			cfg: &config.Config{
				Agents: config.AgentsConfig{
					Phases: config.PhasesConfig{
						Review: "codex",
					},
				},
			},
			phase:    "review",
			wantName: "codex",
		},
		{
			name: "explore phase uses configured agent",
			cfg: &config.Config{
				Agents: config.AgentsConfig{
					Phases: config.PhasesConfig{
						Explore: "gemini",
					},
				},
			},
			phase:    "explore",
			wantName: "gemini",
		},
		{
			name: "phase config uses custom agent definition",
			cfg: &config.Config{
				Agents: config.AgentsConfig{
					Definitions: map[string]config.AgentDefinition{
						"my-tool": {
							Binary: "my-binary",
							Flags:  []string{"--flag"},
						},
					},
					Phases: config.PhasesConfig{
						Refine: "my-tool",
					},
				},
			},
			phase:    "refine",
			wantName: "my-tool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.SetDefaults()
			tt.cfg.NormalizeNilFields()

			agent, err := Resolve(tt.cfg, tt.phase, "", false, nil, nil)
			if err != nil {
				t.Fatalf("Resolve() error = %v, want nil", err)
			}

			if agent == nil {
				t.Fatal("Resolve() returned nil agent")
			}

			if agent.Name() != tt.wantName {
				t.Errorf("Agent.Name() = %q, want %q", agent.Name(), tt.wantName)
			}
		})
	}
}

// TestResolveDefaultsToClaudeWhenNoConfig verifies "claude" is the default
func TestResolveDefaultsToClaudeWhenNoConfig(t *testing.T) {
	tests := []struct {
		name  string
		cfg   *config.Config
		phase string
	}{
		{
			name:  "refine defaults to claude",
			cfg:   &config.Config{},
			phase: "refine",
		},
		{
			name:  "plan defaults to claude",
			cfg:   &config.Config{},
			phase: "plan",
		},
		{
			name:  "review defaults to claude",
			cfg:   &config.Config{},
			phase: "review",
		},
		{
			name:  "explore defaults to claude",
			cfg:   &config.Config{},
			phase: "explore",
		},
		{
			name:  "nil config defaults to claude",
			cfg:   nil,
			phase: "refine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg != nil {
				tt.cfg.SetDefaults()
				tt.cfg.NormalizeNilFields()
			}

			agent, err := Resolve(tt.cfg, tt.phase, "", false, nil, nil)
			if err != nil {
				t.Fatalf("Resolve() error = %v, want nil", err)
			}

			if agent == nil {
				t.Fatal("Resolve() returned nil agent")
			}

			if agent.Name() != "claude" {
				t.Errorf("Agent.Name() = %q, want %q", agent.Name(), "claude")
			}
		})
	}
}

// TestResolveCompletePriorityChain verifies the full priority order
func TestResolveCompletePriorityChain(t *testing.T) {
	t.Run("priority chain: flag > phase > default", func(t *testing.T) {
		cfg := &config.Config{
			Agents: config.AgentsConfig{
				Definitions: map[string]config.AgentDefinition{
					"custom1": {Binary: "bin1"},
					"custom2": {Binary: "bin2"},
				},
				Phases: config.PhasesConfig{
					Refine: "custom1",
				},
			},
		}
		cfg.SetDefaults()
		cfg.NormalizeNilFields()

		// Test 1: Flag override beats everything
		agent, err := Resolve(cfg, "refine", "custom2", false, nil, nil)
		if err != nil {
			t.Fatalf("Resolve() with flag override error = %v", err)
		}
		if agent.Name() != "custom2" {
			t.Errorf("With flag override: got %q, want %q", agent.Name(), "custom2")
		}

		// Test 2: Phase config used when no flag override
		agent, err = Resolve(cfg, "refine", "", false, nil, nil)
		if err != nil {
			t.Fatalf("Resolve() with phase config error = %v", err)
		}
		if agent.Name() != "custom1" {
			t.Errorf("With phase config: got %q, want %q", agent.Name(), "custom1")
		}

		// Test 3: Default when phase not configured
		agent, err = Resolve(cfg, "plan", "", false, nil, nil)
		if err != nil {
			t.Fatalf("Resolve() with default error = %v", err)
		}
		if agent.Name() != "claude" {
			t.Errorf("With default: got %q, want %q", agent.Name(), "claude")
		}
	})
}

// TestResolveInvalidPhaseConfigAgent verifies error when phase config references unknown agent
func TestResolveInvalidPhaseConfigAgent(t *testing.T) {
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Phases: config.PhasesConfig{
				Refine: "nonexistent-agent",
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	agent, err := Resolve(cfg, "refine", "", false, nil, nil)
	if err == nil {
		t.Error("Resolve() with invalid phase config: error = nil, want error")
	}

	if agent != nil {
		t.Errorf("Resolve() with error should return nil agent, got %v", agent)
	}
}

// TestResolveBuiltinPresetsWorkWithoutDefinition verifies built-in presets work without explicit definition
func TestResolveBuiltinPresetsWorkWithoutDefinition(t *testing.T) {
	tests := []struct {
		name      string
		agentName string
	}{
		{
			name:      "claude preset works without definition",
			agentName: "claude",
		},
		{
			name:      "codex preset works without definition",
			agentName: "codex",
		},
		{
			name:      "gemini preset works without definition",
			agentName: "gemini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Config has no agent definitions, but built-in presets should still work
			cfg := &config.Config{}
			cfg.SetDefaults()
			cfg.NormalizeNilFields()

			agent, err := Resolve(cfg, "refine", tt.agentName, false, nil, nil)
			if err != nil {
				t.Fatalf("Resolve() error = %v, want nil for built-in preset %q", err, tt.agentName)
			}

			if agent == nil {
				t.Fatal("Resolve() returned nil agent")
			}

			if agent.Name() != tt.agentName {
				t.Errorf("Agent.Name() = %q, want %q", agent.Name(), tt.agentName)
			}
		})
	}
}

// TestResolveCustomDefinitionOverridesPreset verifies custom definitions override presets
func TestResolveCustomDefinitionOverridesPreset(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *config.Config
		agentName       string
		wantBinaryField string
	}{
		{
			name: "custom claude definition overrides built-in preset",
			cfg: &config.Config{
				Agents: config.AgentsConfig{
					Definitions: map[string]config.AgentDefinition{
						"claude": {
							Binary: "custom-claude",
							Flags:  []string{"--custom-flag"},
						},
					},
				},
			},
			agentName:       "claude",
			wantBinaryField: "custom-claude",
		},
		{
			name: "custom codex definition overrides built-in preset",
			cfg: &config.Config{
				Agents: config.AgentsConfig{
					Definitions: map[string]config.AgentDefinition{
						"codex": {
							Binary: "my-codex-wrapper",
							Flags:  []string{"--wrapper-flag"},
						},
					},
				},
			},
			agentName:       "codex",
			wantBinaryField: "my-codex-wrapper",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.SetDefaults()
			tt.cfg.NormalizeNilFields()

			agent, err := Resolve(tt.cfg, "refine", tt.agentName, false, nil, nil)
			if err != nil {
				t.Fatalf("Resolve() error = %v, want nil", err)
			}

			if agent == nil {
				t.Fatal("Resolve() returned nil agent")
			}

			// Verify custom definition was used by checking internal field
			if ca, ok := agent.(*cliAgent); ok {
				if ca.binary != tt.wantBinaryField {
					t.Errorf("agent.binary = %q, want %q (custom definition should override preset)",
						ca.binary, tt.wantBinaryField)
				}
			} else {
				t.Error("Expected *cliAgent type")
			}
		})
	}
}

// TestResolveClaudePresetUsesClaudeConfig verifies claude preset uses cfg.Claude values
func TestResolveClaudePresetUsesClaudeConfig(t *testing.T) {
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary: "custom-claude-path",
			Flags:  []string{"--custom-flag", "--another-flag"},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	agent, err := Resolve(cfg, "refine", "claude", false, nil, nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if agent == nil {
		t.Fatal("Resolve() returned nil agent")
	}

	// Verify the claude preset uses the ClaudeConfig values
	if ca, ok := agent.(*cliAgent); ok {
		if ca.binary != "custom-claude-path" {
			t.Errorf("claude preset binary = %q, want %q (from cfg.Claude.Binary)",
				ca.binary, "custom-claude-path")
		}

		if len(ca.flags) != 2 || ca.flags[0] != "--custom-flag" || ca.flags[1] != "--another-flag" {
			t.Errorf("claude preset flags = %v, want [--custom-flag --another-flag] (from cfg.Claude.Flags)",
				ca.flags)
		}

		if ca.promptDelivery != FileRef {
			t.Errorf("claude preset should use FileRef delivery, got %v", ca.promptDelivery)
		}
	} else {
		t.Error("Expected *cliAgent type")
	}
}

// TestResolveWithPickerChoosesAgent verifies the picker is used when chooseAgent is true
func TestResolveWithPickerChoosesAgent(t *testing.T) {
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Definitions: map[string]config.AgentDefinition{
				"custom": {
					Binary: "custom-cli",
					Flags:  []string{"--flag"},
				},
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	tests := []struct {
		name       string
		input      string
		wantName   string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:     "choose first option (claude)",
			input:    "1\n",
			wantName: "claude",
			wantErr:  false,
		},
		{
			name:     "choose second option (codex)",
			input:    "2\n",
			wantName: "codex",
			wantErr:  false,
		},
		{
			name:     "choose third option (custom)",
			input:    "3\n",
			wantName: "custom",
			wantErr:  false,
		},
		{
			name:       "invalid choice - zero",
			input:      "0\n",
			wantErr:    true,
			wantErrMsg: "invalid choice",
		},
		{
			name:       "invalid choice - out of range",
			input:      "99\n",
			wantErr:    true,
			wantErrMsg: "invalid choice",
		},
		{
			name:       "invalid choice - non-numeric",
			input:      "abc\n",
			wantErr:    true,
			wantErrMsg: "invalid choice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			w := &strings.Builder{}

			agent, err := Resolve(cfg, "refine", "", true, r, w)

			if tt.wantErr {
				if err == nil {
					t.Error("Resolve() error = nil, want error")
					return
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("Resolve() error = %q, want error containing %q", err.Error(), tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("Resolve() error = %v, want nil", err)
			}

			if agent == nil {
				t.Fatal("Resolve() returned nil agent")
			}

			if agent.Name() != tt.wantName {
				t.Errorf("Agent.Name() = %q, want %q", agent.Name(), tt.wantName)
			}

			// Verify picker output was written
			output := w.String()
			if !strings.Contains(output, "Select agent for refine:") {
				t.Error("Picker output missing 'Select agent for refine:'")
			}
		})
	}
}

// TestResolvePickerShowsDefaultMarker verifies the picker marks the default agent
func TestResolvePickerShowsDefaultMarker(t *testing.T) {
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Phases: config.PhasesConfig{
				Refine: "codex",
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	r := strings.NewReader("1\n")
	w := &strings.Builder{}

	_, err := Resolve(cfg, "refine", "", true, r, w)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	output := w.String()

	// Verify codex is marked as default
	if !strings.Contains(output, "codex (default)") {
		t.Error("Picker output should mark codex as default for refine phase")
	}

	// Verify claude is not marked as default
	if strings.Contains(output, "claude (default)") {
		t.Error("Picker output should not mark claude as default when phase is configured differently")
	}
}
