package main

import (
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/spf13/cobra"
)

func resolveEffectiveInteractiveModel(cmd *cobra.Command, cfg *config.Config, commandName, flagName string) string {
	if flagName == "" {
		flagName = "model"
	}
	if cmd != nil && flagName != "" {
		if flag := cmd.Flags().Lookup(flagName); flag != nil && cmd.Flags().Changed(flagName) {
			return resolveInteractiveModel(cmd, flagName)
		}
	}

	if cfg == nil || cfg.Agents.InteractiveModels == nil {
		return ""
	}

	models := cfg.Agents.InteractiveModels
	var model string
	switch commandName {
	case refineSessionCommand:
		model = models.Refine
	case planSessionCommand:
		model = models.Plan
	case exploreSessionCommand:
		model = models.Explore
	case debugSessionCommand:
		model = models.Debug
	case reviewSessionCommand:
		model = models.Review
	default:
		return ""
	}

	return strings.TrimSpace(model)
}
