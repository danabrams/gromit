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

// TryOverrideModel attempts to override a Claude agent's model when requested.
// If force is false, the override only happens when the CLI flag was changed.
// Setting force to true allows caller-managed overrides (e.g., config defaults).
func TryOverrideModel(cmd *cobra.Command, selectedAgent agent.Agent, modelValue string, cfg *config.Config, flagName string, force bool) agent.Agent {
	if selectedAgent == nil || modelValue == "" {
		return selectedAgent
	}

	if selectedAgent.Name() != "claude" {
		return selectedAgent
	}

	if !force {
		if cmd == nil {
			return selectedAgent
		}
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil || !cmd.Flags().Changed(flagName) {
			return selectedAgent
		}
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
