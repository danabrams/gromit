package main

import (
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execution commands",
}

// newExecSpecCmd creates the `exec spec` command. Exported for testing.
func newExecSpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Execute a spec through the full pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Stub — wired in integration later.
			return nil
		},
	}
	cmd.Flags().String("spec", "", "Path to spec markdown file")
	cmd.Flags().String("project", "", "Project name")
	cmd.Flags().String("policy", "", "Path to execution policy JSON file")
	cmd.Flags().Bool("dry-run", false, "Compile plan but do not execute")
	_ = cmd.MarkFlagRequired("spec")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}
