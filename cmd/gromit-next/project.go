package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/next/architecture"
	"github.com/danabrams/gromit/internal/next/artifact"
	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/enrich"
	"github.com/danabrams/gromit/internal/next/extract"
	"github.com/danabrams/gromit/internal/next/fact"
	"github.com/danabrams/gromit/internal/next/guide"
	"github.com/danabrams/gromit/internal/next/infer"
	"github.com/danabrams/gromit/internal/next/inspect"
	"github.com/danabrams/gromit/internal/next/projectcell"
	"github.com/danabrams/gromit/internal/next/provenance"
	"github.com/danabrams/gromit/internal/next/sourcemap"
	"github.com/danabrams/gromit/internal/next/validation"
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
		artifactsDir := filepath.Join(cell.CellPath, "artifacts")

		// Get current git HEAD SHA for provenance
		gitSHA := gitHeadSHA(cell.RepoPath)

		// Build and write source map
		sm := sourcemap.BuildFromFacts(result.Observed)
		if err := artStore.Write(artifactsDir, "sourcemap", sm); err != nil {
			return fmt.Errorf("write sourcemap: %w", err)
		}

		// Record provenance per written artifact (keyed by artifact name,
		// so IsFresh lookups by artifact name find matching records).
		tracker := provenance.NewFSTracker(filepath.Join(cell.CellPath, "provenance", "provenance.json"))
		if err := tracker.Record(provenance.Record{
			Artifact: "sourcemap",
			GitSHA:   gitSHA,
		}); err != nil {
			return fmt.Errorf("record provenance: %w", err)
		}

		// Build and write validation command set from extracted facts
		cs := buildValidationCommandSet(result.Observed)
		if err := artStore.Write(artifactsDir, "validation", cs); err != nil {
			return fmt.Errorf("write validation: %w", err)
		}
		if err := tracker.Record(provenance.Record{
			Artifact: "validation",
			GitSHA:   gitSHA,
		}); err != nil {
			return fmt.Errorf("record provenance: %w", err)
		}

		// TODO: When a real Inferrer replaces StubInferrer, write additional artifacts:
		// - architecture (from inferred facts)
		// - glossary (from inferred facts)
		// - risks (from inferred facts)

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
		includeInferred, _ := cmd.Flags().GetBool("include-inferred")

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
		docStore.Dir = filepath.Join(cell.CellPath, "doctrine")
		doc, err := docStore.Load()
		if err != nil {
			return fmt.Errorf("load doctrine: %w", err)
		}
		for _, r := range doc.Rules {
			input.Doctrine = append(input.Doctrine, guide.DoctrineRule{ID: r.ID, Summary: r.Summary, Scope: r.Scope})
		}

		// Load inferred facts when flag is set
		if includeInferred {
			cfg, err := enrich.LoadConfig(cell.CellPath)
			if err != nil {
				return fmt.Errorf("load enrichment config: %w", err)
			}

			facts, err := enrich.NewFactStore().LoadFacts(cell.CellPath)
			if err != nil {
				return fmt.Errorf("load inferred facts: %w", err)
			}

			facts = enrich.FilterExpired(facts, cfg.StalenessExpiryDays)

			// Remove rejected and superseded facts from the guide.
			var activeFacts []enrich.InferredFact
			for _, f := range facts {
				if f.Status == enrich.StatusRejected || f.Status == enrich.StatusSuperseded {
					continue
				}
				activeFacts = append(activeFacts, f)
			}
			facts = activeFacts

			if len(facts) == 0 {
				fmt.Fprintln(os.Stderr, "Warning: all inferred facts have expired; guide will omit inferred sections")
			} else {
				for _, f := range facts {
					input.InferredFacts = append(input.InferredFacts, guide.InferredObservation{
						Category:   string(f.Category),
						Statement:  f.Statement,
						Confidence: f.Confidence,
					})
				}
				input.IncludeInferred = true
			}
		}

		renderer := guide.NewMarkdownRenderer()
		output, err := renderer.Render(input)
		if err != nil {
			return err
		}

		guidePath := filepath.Join(cell.CellPath, "guide", "agent-guide.md")
		if err := os.MkdirAll(filepath.Dir(guidePath), 0o755); err != nil {
			return fmt.Errorf("create guide directory: %w", err)
		}
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
	guideCmd.Flags().Bool("include-inferred", false, "include inferred facts in the agent guide")
	projectCmd.AddCommand(attachCmd)
	projectCmd.AddCommand(inspectCmd)
	projectCmd.AddCommand(guideCmd)
	projectCmd.AddCommand(listCmd)
}

// gitHeadSHA returns the current HEAD commit SHA for the repo at the given path.
// Returns an empty string if the SHA cannot be determined.
func gitHeadSHA(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// buildValidationCommandSet converts validation-commands facts into a CommandSet.
// Each fact has Content like "Makefile target 'test': go test ./..." or
// "CI workflow 'ci.yml' run step: go test ./...".
func buildValidationCommandSet(observed []fact.Fact) validation.CommandSet {
	cs := validation.NewCommandSet()
	for _, f := range observed {
		if f.Source != "validation-commands" {
			continue
		}
		name, run, source := parseValidationFact(f.Content)
		cs.Add(validation.Command{
			Name:   name,
			Kind:   inferValidationKind(run),
			Run:    run,
			Source: source,
		})
	}
	return cs
}

// parseValidationFact extracts a name, run command, and source from a
// validation-commands fact content string.
func parseValidationFact(content string) (name, run, source string) {
	// "Makefile target 'test': go test ./..."
	if strings.HasPrefix(content, "Makefile target '") {
		rest := strings.TrimPrefix(content, "Makefile target '")
		if idx := strings.Index(rest, "': "); idx >= 0 {
			name = rest[:idx]
			run = rest[idx+3:]
			source = "Makefile"
			return
		}
	}
	// "CI workflow 'ci.yml' run step: go test ./..."
	if strings.HasPrefix(content, "CI workflow '") {
		rest := strings.TrimPrefix(content, "CI workflow '")
		if idx := strings.Index(rest, "' run step: "); idx >= 0 {
			workflow := rest[:idx]
			run = rest[idx+12:]
			name = workflow + "-run"
			source = workflow
			return
		}
	}
	// Fallback: use the whole content as the run command.
	return "unknown", content, "unknown"
}

// inferValidationKind guesses the Kind of a command from its run string.
func inferValidationKind(run string) validation.Kind {
	lower := strings.ToLower(run)
	switch {
	case strings.Contains(lower, "lint") || strings.Contains(lower, "vet") || strings.Contains(lower, "staticcheck"):
		return validation.Lint
	case strings.Contains(lower, "build") || strings.Contains(lower, "compile"):
		return validation.Build
	default:
		return validation.Test
	}
}

func resolveProjectStore() (projectcell.Store, error) {
	resolver := workspace.NewEnvResolver()
	root, err := resolver.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	return projectcell.NewFSStore(root.ProjectsDir()), nil
}
