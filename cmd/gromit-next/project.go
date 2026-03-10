package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/internal/next/architecture"
	"github.com/danabrams/gromit/internal/next/artifact"
	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/extract"
	"github.com/danabrams/gromit/internal/next/guide"
	"github.com/danabrams/gromit/internal/next/infer"
	"github.com/danabrams/gromit/internal/next/inspect"
	"github.com/danabrams/gromit/internal/next/projectcell"
	"github.com/danabrams/gromit/internal/next/provenance"
	"github.com/danabrams/gromit/internal/next/sourcemap"
	"github.com/danabrams/gromit/internal/next/workspace"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage project cells",
}

var attachCmd = &cobra.Command{
	Use:   "attach [repo-path]",
	Short: "Attach a git repository as a project cell",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			name = filepath.Base(args[0])
		}

		store, err := resolveProjectStore()
		if err != nil {
			return err
		}

		cell, err := store.Create(name, args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Attached project %q at %s\n", cell.Name, cell.CellPath)
		return nil
	},
}

var inspectCmd = &cobra.Command{
	Use:   "inspect [project-name]",
	Short: "Inspect a project to extract and infer facts",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := resolveProjectStore()
		if err != nil {
			return err
		}

		cell, err := store.Get(args[0])
		if err != nil {
			return err
		}

		extractors := []inspect.Extractor{
			extract.NewFileTreeExtractor(),
			extract.NewGoModExtractor(),
			extract.NewValidationCommandsExtractor(),
		}
		inferrer := infer.NewStubInferrer()
		inspector := inspect.NewInspector(extractors, inferrer)

		inspCell := inspect.Cell{Name: cell.Name, RepoPath: cell.RepoPath, CellPath: cell.CellPath}
		result, err := inspector.Inspect(context.Background(), inspCell)
		if err != nil {
			return err
		}

		// Write artifacts
		artStore := artifact.NewJSONStore()
		allFacts := append(result.Observed, result.Inferred...)

		// Build and write source map
		sm := sourcemap.BuildFromFacts(result.Observed)
		if err := artStore.Write(filepath.Join(cell.CellPath, "artifacts"), "sourcemap", sm); err != nil {
			return fmt.Errorf("write sourcemap: %w", err)
		}

		// Record provenance
		tracker := provenance.NewFSTracker(filepath.Join(cell.CellPath, "provenance", "provenance.json"))
		for _, f := range allFacts {
			tracker.Record(provenance.Record{
				FactID:   f.ID,
				Artifact: f.Source,
				Category: f.Category.String(),
			})
		}

		fmt.Printf("Inspected %q: %d observed, %d inferred facts\n",
			args[0], len(result.Observed), len(result.Inferred))
		return nil
	},
}

var guideCmd = &cobra.Command{
	Use:   "guide [project-name]",
	Short: "Render agent guide for a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := resolveProjectStore()
		if err != nil {
			return err
		}

		cell, err := store.Get(args[0])
		if err != nil {
			return err
		}

		artStore := artifact.NewJSONStore()
		docStore := doctrine.NewFSStore()

		input := guide.RenderInput{
			ProjectName: cell.Name,
		}

		// Load sourcemap and convert to guide local types
		var sm sourcemap.SourceMap
		if err := artStore.Read(filepath.Join(cell.CellPath, "artifacts"), "sourcemap", &sm); err == nil {
			for _, e := range sm.Entries {
				input.SourceMap = append(input.SourceMap, guide.SourceMapEntry{Path: e.Path, Language: e.Language, Lines: e.Lines})
			}
		}

		// Load architecture and convert to guide local types
		var arch architecture.Architecture
		if err := artStore.Read(filepath.Join(cell.CellPath, "artifacts"), "architecture", &arch); err == nil {
			for _, m := range arch.Modules {
				input.Modules = append(input.Modules, guide.Module{Name: m.Name, Description: m.Description, Language: m.Language})
			}
		}

		// Load doctrine and convert to guide local types
		doc, _ := docStore.Load(filepath.Join(cell.CellPath, "doctrine"))
		for _, r := range doc.Rules {
			input.Doctrine = append(input.Doctrine, guide.DoctrineRule{ID: r.ID, Summary: r.Summary, Scope: r.Scope})
		}

		renderer := guide.NewMarkdownRenderer()
		output, err := renderer.Render(input)
		if err != nil {
			return err
		}

		guidePath := filepath.Join(cell.CellPath, "guide", "agent-guide.md")
		if err := os.WriteFile(guidePath, output, 0o644); err != nil {
			return err
		}

		fmt.Printf("Guide written to %s\n", guidePath)
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all project cells",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := resolveProjectStore()
		if err != nil {
			return err
		}

		cells, err := store.List()
		if err != nil {
			return err
		}

		if len(cells) == 0 {
			fmt.Println("No projects attached.")
			return nil
		}

		for _, c := range cells {
			fmt.Printf("  %s → %s\n", c.Name, c.RepoPath)
		}
		return nil
	},
}

func init() {
	attachCmd.Flags().String("name", "", "project name (defaults to directory name)")
	projectCmd.AddCommand(attachCmd)
	projectCmd.AddCommand(inspectCmd)
	projectCmd.AddCommand(guideCmd)
	projectCmd.AddCommand(listCmd)
}

func resolveProjectStore() (*projectcell.FSStore, error) {
	resolver := workspace.NewEnvResolver()
	root, err := resolver.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	return projectcell.NewFSStore(root.ProjectsDir()), nil
}
