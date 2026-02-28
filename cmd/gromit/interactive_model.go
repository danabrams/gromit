package main

import (
	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/config"
	"github.com/spf13/cobra"
)

// resolveInteractiveModel gets the model value from a cobra command flag.
// Returns empty string if command is nil or flag doesn't exist.
func resolveInteractiveModel(cmd *cobra.Command, flagName string) string {
	if cmd == nil {
		return ""
	}

	flag := cmd.Flags().Lookup(flagName)
	if flag == nil {
		return ""
	}

	value, _ := cmd.Flags().GetString(flagName)
	return value
}

// TryOverrideModel attempts to override an agent's model if:
// 1. The model flag was explicitly set (changed)
// 2. The selected agent is Claude
// If both conditions are true, returns a new agent with the model flag added.
// Otherwise returns the original agent unchanged.
// If a non-Claude agent has the flag changed, returns the original agent
// (warning should be issued separately if desired).
func TryOverrideModel(cmd *cobra.Command, selectedAgent agent.Agent, modelValue string, cfg *config.Config, flagName string) agent.Agent {
	if cmd == nil || selectedAgent == nil {
		return selectedAgent
	}

	// Check if agent is Claude
	if selectedAgent.Name() != "claude" {
		return selectedAgent
	}

	// Check if the model flag was actually changed
	if !cmd.Flags().Changed(flagName) {
		return selectedAgent
	}

	// Override the agent with the new model flag
	binary := "claude"
	var flags []string
	if cfg != nil {
		binary = cfg.Claude.Binary
		flags = cfg.Claude.Flags
	}

	// Copy existing flags and append model flag
	newFlags := append(append([]string{}, flags...), "--model", modelValue)
	return agent.New("claude", binary, newFlags, agent.FileRef, "", nil)
}
