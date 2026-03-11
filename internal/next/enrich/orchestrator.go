package enrich

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/next/fact"
)

// OrchestratorResult holds the aggregate outcome of a full enrichment run.
type OrchestratorResult struct {
	RunID            string
	TotalFacts       int
	FailedCategories []EnrichmentCategory
	CostUSD          float64
	InputTokens      int
	OutputTokens     int
	Facts            []InferredFact // populated for dry-run output
}

// Orchestrator runs all enrichment category passes, collects results,
// merges statuses with existing facts, and saves artifacts.
type Orchestrator struct {
	enricher  CategoryEnricher
	factStore *FactStore
	runStore  *RunStore
}

// NewOrchestrator creates an Orchestrator with the given dependencies.
func NewOrchestrator(enricher CategoryEnricher, factStore *FactStore, runStore *RunStore) *Orchestrator {
	return &Orchestrator{
		enricher:  enricher,
		factStore: factStore,
		runStore:  runStore,
	}
}

// Run executes all category enrichment passes, merges results with existing
// facts, and persists both facts and run artifacts. Partial failures are
// tolerated: categories that error are recorded but do not abort the run.
func (o *Orchestrator) Run(ctx context.Context, cellPath string, observed []fact.Fact, input EnrichInput, cfg Config) (OrchestratorResult, error) {
	if err := o.guardPreconditions(cellPath, observed); err != nil {
		return OrchestratorResult{}, err
	}

	runID := generateRunID()
	allFacts, categoryResults, failed, totals := o.runCategories(ctx, runID, observed, input)

	// Load existing facts and merge.
	existing, err := o.factStore.LoadFacts(cellPath)
	if err != nil {
		return OrchestratorResult{}, fmt.Errorf("loading existing facts: %w", err)
	}
	merged := o.factStore.MergeWithExisting(existing, allFacts)

	// Save merged facts.
	if err := o.factStore.SaveFacts(cellPath, merged); err != nil {
		return OrchestratorResult{}, fmt.Errorf("saving facts: %w", err)
	}

	// Build and save the run artifact.
	run := EnrichmentRun{
		RunID:        runID,
		Timestamp:    time.Now(),
		Provider:     cfg.Provider,
		Model:        cfg.Model,
		Reasoning:    cfg.Reasoning,
		Inputs:       input,
		Request:      RunRequest{Categories: AllCategories()},
		Results:      categoryResults,
		CostUSD:      totals.costUSD,
		InputTokens:  totals.inputTokens,
		OutputTokens: totals.outputTokens,
	}
	if err := o.runStore.SaveRun(cellPath, run); err != nil {
		return OrchestratorResult{}, fmt.Errorf("saving run: %w", err)
	}

	return OrchestratorResult{
		RunID:            runID,
		TotalFacts:       len(allFacts),
		FailedCategories: failed,
		CostUSD:          totals.costUSD,
		InputTokens:      totals.inputTokens,
		OutputTokens:     totals.outputTokens,
		Facts:            allFacts,
	}, nil
}

// DryRun executes all category enrichment passes but does not persist any
// artifacts. Useful for previewing what an enrichment run would produce.
func (o *Orchestrator) DryRun(ctx context.Context, cellPath string, observed []fact.Fact, input EnrichInput, cfg Config) (OrchestratorResult, error) {
	if err := o.guardPreconditions(cellPath, observed); err != nil {
		return OrchestratorResult{}, err
	}

	runID := generateRunID()
	allFacts, _, failed, totals := o.runCategories(ctx, runID, observed, input)

	return OrchestratorResult{
		RunID:            runID,
		TotalFacts:       len(allFacts),
		FailedCategories: failed,
		CostUSD:          totals.costUSD,
		InputTokens:      totals.inputTokens,
		OutputTokens:     totals.outputTokens,
		Facts:            allFacts,
	}, nil
}

// guardPreconditions checks that either observed facts are provided or
// the inferred artifacts directory already exists.
func (o *Orchestrator) guardPreconditions(cellPath string, observed []fact.Fact) error {
	if len(observed) > 0 {
		return nil
	}
	inferredDir := filepath.Join(cellPath, "inferred")
	if _, err := os.Stat(inferredDir); err != nil {
		return fmt.Errorf("no observed facts and no existing artifacts directory at %s", inferredDir)
	}
	return nil
}

type runTotals struct {
	costUSD      float64
	inputTokens  int
	outputTokens int
}

// runCategories executes the enricher for every category, collecting facts
// and results. Errors on individual categories are recorded but do not
// stop subsequent categories from running.
func (o *Orchestrator) runCategories(ctx context.Context, runID string, observed []fact.Fact, input EnrichInput) (
	allFacts []InferredFact,
	results []CategoryResult,
	failed []EnrichmentCategory,
	totals runTotals,
) {
	allFacts = []InferredFact{}
	results = []CategoryResult{}
	failed = []EnrichmentCategory{}

	for _, cat := range AllCategories() {
		res, err := o.enricher.Enrich(ctx, cat, observed, input)

		cr := CategoryResult{
			Category:     cat,
			Success:      res.Success,
			FactCount:    res.FactCount,
			Error:        res.Error,
			CostUSD:      res.CostUSD,
			InputTokens:  res.InputTokens,
			OutputTokens: res.OutputTokens,
		}
		results = append(results, cr)

		totals.costUSD += res.CostUSD
		totals.inputTokens += res.InputTokens
		totals.outputTokens += res.OutputTokens

		if err != nil {
			failed = append(failed, cat)
			continue
		}

		// Stamp each fact with run metadata, compute IDs.
		for i := range res.Facts {
			f := &res.Facts[i]
			f.SourceType = "inferred"
			f.InferenceRunID = runID
			f.NormalizeNilFields()
			if f.FactID == "" {
				f.FactID = f.ComputeID()
			}
			if f.CreatedAt.IsZero() {
				f.CreatedAt = time.Now()
			}
		}
		allFacts = append(allFacts, res.Facts...)
	}

	return allFacts, results, failed, totals
}

// generateRunID produces a timestamp-based run identifier with a random
// suffix to avoid collisions when multiple runs occur within the same second.
func generateRunID() string {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%x", time.Now().Format("20060102-150405"), b)
}
