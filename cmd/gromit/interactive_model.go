package main

import "github.com/spf13/cobra"

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
