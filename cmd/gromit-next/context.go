package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

		compiler := contextpkt.NewCompiler(wrappedStore)
		ctxCell := contextpkt.Cell{Name: cell.Name, CellPath: cell.CellPath}
		packet, err := compiler.Compile(context.Background(), ctxCell, level, contextpkt.CompileOpts{
			SpecPath:    specPath,
			TaskID:      taskID,
			TokenBudget: budgetTokens,
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

func init() {
	contextBuildCmd.Flags().String("level", "project", "context level: project, spec, or task")
	contextBuildCmd.Flags().String("spec", "", "spec file path (required for spec and task levels)")
	contextBuildCmd.Flags().String("task", "", "task ID (required for task level)")
	contextBuildCmd.Flags().Int("budget", 0, "token budget (0 for unlimited)")
	contextCmd.AddCommand(contextBuildCmd)
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
