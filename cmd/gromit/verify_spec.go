package main

import (
	"github.com/spf13/cobra"
)

var verifySpecCreateBeads bool

var verifySpecCmd = &cobra.Command{
	Use:   "verify-spec <spec>",
	Short: "Verify a spec's acceptance criteria",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	verifySpecCmd.Flags().BoolVar(&verifySpecCreateBeads, "create-beads", false, "Create fix beads for failing criteria")
	rootCmd.AddCommand(verifySpecCmd)
}
