package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/danabrams/gromit/internal/next/artifact"
	"github.com/danabrams/gromit/internal/next/contextpkt"

	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Context compilation commands",
}

var contextBuildCmd = &cobra.Command{
	Use:   "build [project-name]",
	Short: "Compile a context packet from project memory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		levelStr, _ := cmd.Flags().GetString("level")
		specPath, _ := cmd.Flags().GetString("spec")
		taskID, _ := cmd.Flags().GetString("task")
		budgetTokens, _ := cmd.Flags().GetInt("budget")

		var level contextpkt.Level
		switch levelStr {
		case "project":
			level = contextpkt.LevelProject
		case "spec":
			level = contextpkt.LevelSpec
		case "task":
			level = contextpkt.LevelTask
		default:
			return fmt.Errorf("invalid level %q: must be project, spec, or task", levelStr)
		}

		store, err := resolveProjectStore()
		if err != nil {
			return err
		}

		cell, err := store.Get(args[0])
		if err != nil {
			return err
		}

		artStore := artifact.NewJSONStore()
		// Wrap to read from artifacts subdirectory
		wrappedStore := &artifactsDirStore{
			store: artStore,
		}

		includeInferred, _ := cmd.Flags().GetBool("include-inferred")

		compiler := contextpkt.NewCompiler(wrappedStore)
		ctxCell := contextpkt.Cell{Name: cell.Name, CellPath: cell.CellPath}
		packet, err := compiler.Compile(context.Background(), ctxCell, level, contextpkt.CompileOpts{
			SpecPath:        specPath,
			TaskID:          taskID,
			TokenBudget:     budgetTokens,
			IncludeInferred: includeInferred,
		})
		if err != nil {
			return err
		}

		data, err := json.MarshalIndent(packet, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	},
}

var contextInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect context artifacts for a project cell",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName, _ := cmd.Flags().GetString("project")
		if projectName == "" {
			return fmt.Errorf("--project flag is required")
		}

		store, err := resolveProjectStore()
		if err != nil {
			return err
		}

		cell, err := store.Get(projectName)
		if err != nil {
			return err
		}

		subdirs := []string{"artifacts", "doctrine", "guide", "provenance"}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintf(w, "DIRECTORY\tFILE\tSIZE\n")

		totalFiles := 0
		for _, sub := range subdirs {
			dir := filepath.Join(cell.CellPath, sub)
			entries, err := os.ReadDir(dir)
			if err != nil {
				// Directory doesn't exist — skip
				continue
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "%s\t%s\t%d bytes\n", sub, e.Name(), info.Size())
				totalFiles++
			}
		}
		w.Flush()

		if totalFiles == 0 {
			fmt.Printf("\nNo context artifacts found for project %q.\n", projectName)
		} else {
			fmt.Printf("\n%d artifact(s) found for project %q.\n", totalFiles, projectName)
		}
		return nil
	},
}

var contextEnrichCmd = &cobra.Command{
	Use:   "enrich",
	Short: "Enrich context for a project cell",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName, _ := cmd.Flags().GetString("project")
		if projectName == "" {
			return fmt.Errorf("--project flag is required")
		}

		fmt.Printf("Context enrichment not yet implemented for project %q\n", projectName)
		return nil
	},
}

func init() {
	contextBuildCmd.Flags().String("level", "project", "context level: project, spec, or task")
	contextBuildCmd.Flags().String("spec", "", "spec file path (required for spec and task levels)")
	contextBuildCmd.Flags().String("task", "", "task ID (required for task level)")
	contextBuildCmd.Flags().Int("budget", 0, "token budget (0 for unlimited)")
	contextBuildCmd.Flags().Bool("include-inferred", false, "include inferred facts in the context packet")

	contextInspectCmd.Flags().String("project", "", "project name")
	contextEnrichCmd.Flags().String("project", "", "project name")

	contextCmd.AddCommand(contextBuildCmd)
	contextCmd.AddCommand(contextInspectCmd)
	contextCmd.AddCommand(contextEnrichCmd)
}

// artifactsDirStore wraps artifact.JSONStore to read from the artifacts/ subdirectory.
type artifactsDirStore struct {
	store artifact.Store
}

func (a *artifactsDirStore) Read(cellPath string, art string, dest any) error {
	return a.store.Read(filepath.Join(cellPath, "artifacts"), art, dest)
}

func (a *artifactsDirStore) Write(cellPath string, art string, src any) error {
	return a.store.Write(filepath.Join(cellPath, "artifacts"), art, src)
}

func (a *artifactsDirStore) Exists(cellPath string, art string) bool {
	return a.store.Exists(filepath.Join(cellPath, "artifacts"), art)
}
