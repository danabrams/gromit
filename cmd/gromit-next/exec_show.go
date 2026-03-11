package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/spf13/cobra"
)

// unwrapAll recursively unwraps an error chain to find the root cause.
func unwrapAll(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}

// newExecShowCmd creates the `exec show` command.
func newExecShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [run-id|latest]",
		Short: "Show details of a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			full, _ := cmd.Flags().GetBool("full")
			storeDir, _ := cmd.Flags().GetString("store-dir")
			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			store := runstore.NewStore(storeDir)
			runID, err := resolveRunID(args[0], project, store)
			if err != nil {
				return err
			}

			output, err := execShow(runID, store, full)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), output)
			return nil
		},
	}
	cmd.Flags().String("project", "", "Project name (required when using 'latest')")
	cmd.Flags().Bool("full", false, "Print complete evidence bundle contents")
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	return cmd
}

// resolveRunID resolves "latest" to the most recent run ID for a project.
func resolveRunID(id string, projectID string, store *runstore.Store) (string, error) {
	if id != "latest" {
		return id, nil
	}
	if projectID == "" {
		return "", fmt.Errorf("--project is required when using 'latest'")
	}
	runs, err := store.List(projectID)
	if err != nil {
		return "", fmt.Errorf("list runs: %w", err)
	}
	if len(runs) == 0 {
		return "", fmt.Errorf("no runs found for project %q", projectID)
	}
	// Sort by StartedAt descending to find the most recent.
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	return runs[0].RunID, nil
}

// execShow formats run details as a string.
func execShow(runID string, store *runstore.Store, full bool) (string, error) {
	rs, err := store.Get(runID)
	if err != nil {
		if os.IsNotExist(unwrapAll(err)) {
			return "", fmt.Errorf("run %q not found", runID)
		}
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Run:     %s\n", rs.RunID)
	fmt.Fprintf(&b, "Spec:    %s\n", rs.SpecID)
	fmt.Fprintf(&b, "Project: %s\n", rs.ProjectID)
	fmt.Fprintf(&b, "Status:  %s\n", rs.Status)
	fmt.Fprintf(&b, "Started: %s\n", rs.StartedAt.Format("2006-01-02 15:04:05"))
	if !rs.EndedAt.IsZero() {
		fmt.Fprintf(&b, "Ended:   %s\n", rs.EndedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Fprintf(&b, "Tasks:   %d\n", len(rs.Tasks))

	if full {
		evidenceDir := store.RunEvidenceDir(runID)
		entries, err := os.ReadDir(evidenceDir)
		if err == nil && len(entries) > 0 {
			fmt.Fprintf(&b, "\n--- Evidence ---\n")
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				path := filepath.Join(evidenceDir, entry.Name())
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					continue
				}
				fmt.Fprintf(&b, "\n=== %s ===\n%s\n", entry.Name(), string(data))
			}
		}
	}

	return b.String(), nil
}
