package agent

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/danabrams/gromit/internal/config"
)

const (
	// defaultPromptFlag is the default flag name for passing prompt files to agents
	defaultPromptFlag = "--prompt"
)

// Resolve returns an Agent for the given phase using priority-based resolution.
// Resolution priority: flag override, picker (if chooseAgent or agents.prompt), phase config, "claude" default.
// Merges user-defined definitions with preset defaults.
func Resolve(cfg *config.Config, phase string, flagOverride string, chooseAgent bool, r io.Reader, w io.Writer) (Agent, error) {
	// Step 1: Flag override has highest priority
	if flagOverride != "" {
		return resolveByName(flagOverride, cfg)
	}

	// Step 2: Interactive picker (if chooseAgent is true OR agents.prompt is true)
	shouldPrompt := chooseAgent || (cfg != nil && cfg.Agents.Prompt)
	if shouldPrompt {
		agentName, err := pickAgent(cfg, phase, r, w)
		if err != nil {
			return nil, fmt.Errorf("agent picker: %w", err)
		}
		return resolveByName(agentName, cfg)
	}

	// Step 3: Phase config
	agentName := getPhaseAgent(cfg, phase)
	if agentName != "" {
		return resolveByName(agentName, cfg)
	}

	// Step 4: Default to "claude"
	return resolveByName("claude", cfg)
}

// resolveByName returns an Agent for the given agent name using the provided config.
// It checks for custom agent definitions first, then falls back to built-in presets.
// Returns an error if the agent name is unknown or empty.
func resolveByName(name string, cfg *config.Config) (Agent, error) {
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

// getPhaseAgent returns the configured agent name for the given phase, or empty string if not configured
func getPhaseAgent(cfg *config.Config, phase string) string {
	if cfg == nil {
		return ""
	}

	switch phase {
	case "refine":
		return cfg.Agents.Phases.Refine
	case "plan":
		return cfg.Agents.Phases.Plan
	case "review":
		return cfg.Agents.Phases.Review
	case "explore":
		return cfg.Agents.Phases.Explore
	default:
		return ""
	}
}

// pickAgent displays an interactive picker for selecting an agent
func pickAgent(cfg *config.Config, phase string, r io.Reader, w io.Writer) (string, error) {
	// Get list of all available agents (built-in presets + custom definitions)
	agents := getAvailableAgents(cfg)
	if len(agents) == 0 {
		return "", fmt.Errorf("no agents available")
	}

	// Get the default agent for this phase
	defaultAgent := getPhaseAgent(cfg, phase)
	if defaultAgent == "" {
		defaultAgent = "claude"
	}

	// Display picker
	fmt.Fprintf(w, "Select agent for %s:\n\n", phase)
	for i, agent := range agents {
		marker := ""
		if agent == defaultAgent {
			marker = " (default)"
		}
		fmt.Fprintf(w, "  %d. %s%s\n", i+1, agent, marker)
	}
	fmt.Fprintf(w, "\nChoice [1-%d]: ", len(agents))

	// Read choice
	var input string
	if _, err := fmt.Fscanln(r, &input); err != nil {
		return "", fmt.Errorf("reading choice: %w", err)
	}

	input = strings.TrimSpace(input)
	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > len(agents) {
		return "", fmt.Errorf("invalid choice: %s", input)
	}

	return agents[choice-1], nil
}

// getAvailableAgents returns a sorted list of all available agent names
func getAvailableAgents(cfg *config.Config) []string {
	agents := make(map[string]bool)

	// Add built-in presets
	agents["claude"] = true
	agents["codex"] = true
	agents["gemini"] = true

	// Add custom definitions
	if cfg != nil && cfg.Agents.Definitions != nil {
		for name := range cfg.Agents.Definitions {
			agents[name] = true
		}
	}

	// Convert to sorted slice
	result := make([]string, 0, len(agents))
	for name := range agents {
		result = append(result, name)
	}
	sort.Strings(result)

	return result
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
