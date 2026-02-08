package agent

import (
	"fmt"

	"github.com/danabrams/gromit/internal/config"
)

const (
	// defaultPromptFlag is the default flag name for passing prompt files to agents
	defaultPromptFlag = "--prompt"
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
			return resolveCustomAgent(name, def), nil
		}
	}

	// Fall back to built-in presets
	switch name {
	case "claude":
		return resolveClaudePreset(cfg), nil
	case "codex", "gemini":
		return resolvePromptFileArgPreset(name), nil
	default:
		return nil, fmt.Errorf("unknown agent: %s", name)
	}
}

// resolveClaudePreset creates an agent for Claude using config.Claude values
func resolveClaudePreset(cfg *config.Config) Agent {
	binary := "claude"
	var flags []string

	if cfg != nil {
		binary = cfg.Claude.Binary
		flags = cfg.Claude.Flags
	}

	return New("claude", binary, flags, FileRef, "", nil)
}

// resolvePromptFileArgPreset creates an agent using prompt_file_arg delivery
func resolvePromptFileArgPreset(name string) Agent {
	return New(name, name, nil, PromptFileArg, defaultPromptFlag, nil)
}

// resolveCustomAgent creates an agent from a custom definition
func resolveCustomAgent(name string, def config.AgentDefinition) Agent {
	binary := def.Binary
	if binary == "" {
		// Default to agent name as binary
		binary = name
	}

	// Custom agents default to prompt_file_arg delivery
	return New(name, binary, def.Flags, PromptFileArg, defaultPromptFlag, nil)
}
