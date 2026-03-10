package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// contextCmd is the parent command for context compilation subcommands.
// These commands operate on the new workspace-based architecture (internal/next/).
var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Compile context packets for LLM invocations (next-gen)",
}

var contextBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build a context packet from workspace artifacts",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("gromit context build: not implemented yet")
		fmt.Println("This will compile a token-budgeted context packet from project artifacts.")
		return nil
	},
}

func init() {
	contextCmd.AddCommand(contextBuildCmd)
	rootCmd.AddCommand(contextCmd)
}
