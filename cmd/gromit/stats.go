package main

import (
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Display model performance statistics",
	Long: `Display per-model statistics including success rates, iteration counts,
cost per completed bead, and escalation frequency for both project and global scopes.

Examples:
  gromit stats              # Show project and global stats
  gromit stats --json       # Output stats as JSON`,
	RunE: runStats,
}

var statsJSON bool

func init() {
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output stats as JSON")
	rootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	return nil
}
