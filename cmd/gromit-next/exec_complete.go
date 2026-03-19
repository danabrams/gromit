package main

import (
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/spf13/cobra"
)

// newExecCompleteCmd creates the `exec complete` command.
func newExecCompleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "complete <run-id>",
		Short: "Mark a run as completed",
		Long:  `Mark a run as completed after manual review and merge. This removes the run from the resume picker.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			runID := args[0]
			storeDir, _ := cmd.Flags().GetString("store-dir")
			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			store := runstore.NewStore(storeDir)
			rs, err := store.Get(runID)
			if err != nil {
				return fmt.Errorf("load run %q: %w", runID, err)
			}

			rs.Status = runstore.StatusCompleted
			if rs.EndedAt.IsZero() {
				rs.EndedAt = time.Now()
			}

			if err := store.Save(rs); err != nil {
				return fmt.Errorf("save run %q: %w", runID, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Run %s marked as completed\n", runID)
			return nil
		},
	}
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	return cmd
}
