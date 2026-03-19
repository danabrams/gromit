package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/workspace"
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
			specsDir, _ := cmd.Flags().GetString("specs-dir")
			project, _ := cmd.Flags().GetString("project")

			store := runstore.NewStore(storeDir)
			rs, err := store.Get(runID)
			if err != nil {
				return fmt.Errorf("load run %q: %w", runID, err)
			}

			// Resolve specsDir if not explicitly provided
			if specsDir == "" && project != "" {
				resolver := workspace.NewEnvResolver()
				root, err := resolver.Resolve()
				if err != nil {
					return fmt.Errorf("resolve workspace root: %w", err)
				}
				projectDir, err := ResolveProjectConfigPath(root, project)
				if err != nil {
					return fmt.Errorf("resolve project config: %w", err)
				}
				cfg, err := LoadProjectConfig(projectDir)
				if err != nil {
					return fmt.Errorf("load project config: %w", err)
				}
				specsDir = cfg.SpecsDir
				if specsDir == "" && cfg.RepoPath != "" {
					specsDir = filepath.Join(cfg.RepoPath, "docs", "specs")
				}
			}

			// Mark run as completed
			rs.Status = runstore.StatusCompleted
			if rs.EndedAt.IsZero() {
				rs.EndedAt = time.Now()
			}

			if err := store.Save(rs); err != nil {
				return fmt.Errorf("save run %q: %w", runID, err)
			}

			// Attempt to mark spec file as DONE if specsDir is available
			if specsDir != "" && rs.SpecID != "" {
				specPath := filepath.Join(specsDir, rs.SpecID+".md")
				if data, err := os.ReadFile(specPath); err == nil {
					content := string(data)
					// Skip if already starts with DONE
					if _, isDone := ParseDoneDate(content); !isDone {
						today := time.Now().Format("2006-01-02")
						newContent := fmt.Sprintf("DONE %s\n%s", today, content)
						if err := os.WriteFile(specPath, []byte(newContent), 0o644); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not mark spec file as done: %v\n", err)
						}
					}
				}
				// Silently skip if spec file doesn't exist
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Run %s marked as completed\n", runID)
			return nil
		},
	}
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	cmd.Flags().String("specs-dir", "", "Override specs directory (for testing)")
	cmd.Flags().String("project", "", "Project name for resolving specsDir from config")
	return cmd
}
