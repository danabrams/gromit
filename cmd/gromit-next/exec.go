package main

import (
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execution commands",
}

// dryRunStages is the set of stage names that run during a dry-run.
var dryRunStages = map[string]bool{
	"init":    true,
	"compile": true,
	"plan":    true,
}

// filterStagesForDryRun returns only the dry-run stages when dryRun is true,
// or all stages when dryRun is false.
func filterStagesForDryRun(stages []specloop.Stage, dryRun bool) []specloop.Stage {
	if !dryRun {
		return stages
	}
	var filtered []specloop.Stage
	for _, s := range stages {
		if dryRunStages[s.Name()] {
			filtered = append(filtered, s)
		}
	}
	return filtered
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
