package main

import "github.com/spf13/cobra"

var experimentsJSON bool

var experimentsCmd = &cobra.Command{
	Use:   "experiments",
	Short: "Show registered experiments",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	experimentsCmd.Flags().BoolVar(&experimentsJSON, "json", false, "Output experiments report as JSON")
	rootCmd.AddCommand(experimentsCmd)
}
