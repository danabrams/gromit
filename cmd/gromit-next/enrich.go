package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/internal/next/artifact"
	"github.com/danabrams/gromit/internal/next/enrich"
	"github.com/danabrams/gromit/internal/next/extract"
	"github.com/danabrams/gromit/internal/next/fact"
	"github.com/danabrams/gromit/internal/next/infer"
	"github.com/danabrams/gromit/internal/next/inspect"
	"github.com/danabrams/gromit/internal/next/provenance"
	"github.com/danabrams/gromit/internal/next/sourcemap"
	"github.com/spf13/cobra"
)

var enrichCmd = &cobra.Command{
	Use:   "enrich [project-name]",
	Short: "Run inference enrichment on a project's observed facts",
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

		// Load enrichment config, apply CLI flag overrides.
		cfg, err := enrich.LoadConfig(cell.CellPath)
		if err != nil {
			return fmt.Errorf("load enrichment config: %w", err)
		}
		if v, _ := cmd.Flags().GetString("provider"); v != "" {
			cfg.Provider = v
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			cfg.Model = v
		}
		if v, _ := cmd.Flags().GetString("reasoning"); v != "" {
			cfg.Reasoning = v
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		refresh, _ := cmd.Flags().GetBool("refresh")

		// Check provenance freshness.
		tracker := provenance.NewFSTracker(filepath.Join(cell.CellPath, "provenance", "provenance.json"))
		headSHA := gitHeadSHA(cell.RepoPath)
		if rec, err := tracker.Check("sourcemap"); err == nil {
			if warning := enrich.CheckObservedFreshness(rec.GitSHA, headSHA); warning != "" {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
			}
		}

		// If --refresh, re-run inspect first.
		if refresh {
			if err := runInspect(cell.Name, cell.RepoPath, cell.CellPath); err != nil {
				return fmt.Errorf("refresh inspect: %w", err)
			}
		}

		// Load observed facts from sourcemap artifact.
		artStore := artifact.NewJSONStore()
		artifactsDir := filepath.Join(cell.CellPath, "artifacts")

		var sm sourcemap.SourceMap
		if err := artStore.Read(artifactsDir, "sourcemap", &sm); err != nil {
			return fmt.Errorf("read sourcemap: %w", err)
		}

		observed := sourcemapToFacts(sm)

		// Build EnrichInput from artifacts.
		input := enrich.EnrichInput{
			ProjectName: cell.Name,
		}
		if data, err := readArtifactJSON(artStore, artifactsDir, "architecture"); err == nil {
			input.Architecture = string(data)
		}
		if data, err := readArtifactJSON(artStore, artifactsDir, "doctrine"); err == nil {
			input.Doctrine = string(data)
		}
		if data, err := readArtifactJSON(artStore, artifactsDir, "sourcemap"); err == nil {
			input.SourceMap = string(data)
		}
		if data, err := readArtifactJSON(artStore, artifactsDir, "validation"); err == nil {
			input.Validation = string(data)
		}
		if data, err := readArtifactJSON(artStore, artifactsDir, "glossary"); err == nil {
			input.Glossary = string(data)
		}

		// Build file tree from sourcemap entries.
		for _, e := range sm.Entries {
			input.FileTree = append(input.FileTree, e.Path)
		}
		if input.FileTree == nil {
			input.FileTree = []string{}
		}

		ctx := context.Background()

		if dryRun {
			// Dry run doesn't need a real provider — use mock enricher
			// to show which categories would be processed without calling an LLM.
			mockEnricher := &enrich.MockEnricher{
				Facts: []enrich.InferredFact{},
			}
			factStore := enrich.NewFactStore()
			runStore := enrich.NewRunStore()
			orch := enrich.NewOrchestrator(mockEnricher, factStore, runStore)

			result, err := orch.DryRun(ctx, cell.CellPath, observed, input, cfg)
			if err != nil {
				return fmt.Errorf("dry run: %w", err)
			}
			data, err := json.MarshalIndent(result.Facts, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal dry-run facts: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}

		// Guard: without a real provider, Run will fail.
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			return fmt.Errorf("provider not configured: set ANTHROPIC_API_KEY or use --dry-run")
		}

		// Create enricher and orchestrator.
		enricher := enrich.NewLLMEnricher(nil, cfg.Model, cfg.Reasoning) // TODO: wire up real provider
		factStore := enrich.NewFactStore()
		runStore := enrich.NewRunStore()
		orch := enrich.NewOrchestrator(enricher, factStore, runStore)

		result, err := orch.Run(ctx, cell.CellPath, observed, input, cfg)
		if err != nil {
			return fmt.Errorf("enrichment run: %w", err)
		}

		fmt.Printf("Enrichment complete: %d facts produced, cost $%.4f, %d input tokens, %d output tokens\n",
			result.TotalFacts, result.CostUSD, result.InputTokens, result.OutputTokens)
		if len(result.FailedCategories) > 0 {
			fmt.Printf("Failed categories: %v\n", result.FailedCategories)
		}
		return nil
	},
}

func init() {
	enrichCmd.Flags().String("provider", "", "LLM provider (overrides config)")
	enrichCmd.Flags().String("model", "", "LLM model (overrides config)")
	enrichCmd.Flags().String("reasoning", "", "reasoning level (overrides config)")
	enrichCmd.Flags().Bool("refresh", false, "re-run inspect before enriching")
	enrichCmd.Flags().Bool("dry-run", false, "preview enrichment without persisting")
	projectCmd.AddCommand(enrichCmd)
}

// runInspect executes the same inspection logic as inspectCmd.
func runInspect(name, repoPath, cellPath string) error {
	extractors := []inspect.Extractor{
		extract.NewFileTreeExtractor(),
		extract.NewGoModExtractor(),
		extract.NewValidationCommandsExtractor(),
	}
	inferrer := infer.NewStubInferrer()
	inspector := inspect.NewInspector(extractors, inferrer)

	inspCell := inspect.Cell{Name: name, RepoPath: repoPath, CellPath: cellPath}
	result, err := inspector.Inspect(context.Background(), inspCell)
	if err != nil {
		return err
	}

	artStore := artifact.NewJSONStore()
	artifactsDir := filepath.Join(cellPath, "artifacts")
	gitSHA := gitHeadSHA(repoPath)

	sm := sourcemap.BuildFromFacts(result.Observed)
	if err := artStore.Write(artifactsDir, "sourcemap", sm); err != nil {
		return fmt.Errorf("write sourcemap: %w", err)
	}

	tracker := provenance.NewFSTracker(filepath.Join(cellPath, "provenance", "provenance.json"))
	if err := tracker.Record(provenance.Record{
		Artifact: "sourcemap",
		GitSHA:   gitSHA,
	}); err != nil {
		return fmt.Errorf("record provenance: %w", err)
	}

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

	fmt.Printf("Refreshed %q: %d observed, %d inferred facts\n",
		name, len(result.Observed), len(result.Inferred))
	return nil
}

// sourcemapToFacts converts sourcemap entries to fact.Fact slices for enrichment input.
func sourcemapToFacts(sm sourcemap.SourceMap) []fact.Fact {
	facts := make([]fact.Fact, 0, len(sm.Entries))
	for _, e := range sm.Entries {
		data, err := json.Marshal(e)
		if err != nil {
			continue
		}
		facts = append(facts, fact.Fact{
			Category: fact.Observed,
			Content:  string(data),
			Source:   "file-tree",
		})
	}
	return facts
}

// readArtifactJSON reads an artifact file and returns its raw JSON bytes.
func readArtifactJSON(store *artifact.JSONStore, artifactsDir, name string) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := store.Read(artifactsDir, name, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
