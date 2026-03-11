package main

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/spf13/cobra"
)

// newExecListCmd creates the `exec list` command.
func newExecListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List runs for a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			storeDir, _ := cmd.Flags().GetString("store-dir")
			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			store := runstore.NewStore(storeDir)
			output, err := execList(project, store)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), output)
			return nil
		},
	}
	cmd.Flags().String("project", "", "Project name")
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}

// execList formats a table of runs for a project.
func execList(projectID string, store *runstore.Store) (string, error) {
	runs, err := store.List(projectID)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "RUN ID\tSPEC\tSTATUS\tSTARTED")
	for _, rs := range runs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			rs.RunID, rs.SpecID, rs.Status,
			rs.StartedAt.Format("2006-01-02 15:04:05"))
	}
	tw.Flush()
	return b.String(), nil
}
