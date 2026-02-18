package main

import (
	"github.com/spf13/cobra"
)

var verifySpecCmd = &cobra.Command{
	Use:   "verify-spec <spec>",
	Short: "Verify a spec's acceptance criteria",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	rootCmd.AddCommand(verifySpecCmd)
}
