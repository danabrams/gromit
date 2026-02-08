package agent

import (
	"fmt"

	"github.com/danabrams/gromit/internal/config"
)

// Resolve returns an Agent for the given agent name using the provided config.
// It checks for custom agent definitions first, then falls back to built-in presets.
// Returns an error if the agent name is unknown or empty.
func Resolve(name string, cfg *config.Config) (Agent, error) {
	if name == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}

	// Check for custom definition first (overrides built-in presets)
	if cfg != nil && cfg.Agents.Definitions != nil {
		if def, ok := cfg.Agents.Definitions[name]; ok {
			return resolveCustomAgent(name, def)
		}
	}

	// Fall back to built-in presets
	switch name {
	case "claude":
		return resolveClaudePreset(cfg)
	case "codex":
		return resolveCodexPreset()
	case "gemini":
		return resolveGeminiPreset()
	default:
		return nil, fmt.Errorf("unknown agent: %s", name)
	}
}

// resolveClaudePreset creates an agent for Claude using config.Claude values
func resolveClaudePreset(cfg *config.Config) (Agent, error) {
	binary := "claude"
	var flags []string

	if cfg != nil {
		binary = cfg.Claude.Binary
		flags = cfg.Claude.Flags
	}

	return New("claude", binary, flags, FileRef, "", nil), nil
}

// resolveCodexPreset creates an agent for Codex with prompt_file_arg delivery
func resolveCodexPreset() (Agent, error) {
	return New("codex", "codex", nil, PromptFileArg, "--prompt", nil), nil
}

// resolveGeminiPreset creates an agent for Gemini with prompt_file_arg delivery
func resolveGeminiPreset() (Agent, error) {
	return New("gemini", "gemini", nil, PromptFileArg, "--prompt", nil), nil
}

// resolveCustomAgent creates an agent from a custom definition
func resolveCustomAgent(name string, def config.AgentDefinition) (Agent, error) {
	binary := def.Binary
	if binary == "" {
		// Default to agent name as binary
		binary = name
	}

	// Custom agents default to prompt_file_arg delivery with --prompt flag
	return New(name, binary, def.Flags, PromptFileArg, "--prompt", nil), nil
}
