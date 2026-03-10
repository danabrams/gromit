// Package inspect performs static analysis on a project repository to produce
// structured facts about the codebase.
//
// Inspection is the primary input-gathering phase. It runs a set of extractors
// to produce observed facts, then passes those to an inferrer to derive
// higher-level inferred facts.
package inspect

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/next/fact"
)

// DefaultInspector orchestrates the extract-then-infer pipeline.
type DefaultInspector struct {
	extractors []Extractor
	inferrer   Inferrer
}

// NewInspector creates a DefaultInspector with the given extractors and inferrer.
func NewInspector(extractors []Extractor, inferrer Inferrer) *DefaultInspector {
	return &DefaultInspector{
		extractors: extractors,
		inferrer:   inferrer,
	}
}

// Inspect runs all extractors against the cell's repo, then infers higher-level facts.
func (d *DefaultInspector) Inspect(ctx context.Context, cell Cell) (Result, error) {
	var observed []fact.Fact
	for _, ext := range d.extractors {
		facts, err := ext.Extract(cell.RepoPath)
		if err != nil {
			return Result{}, fmt.Errorf("extractor %s: %w", ext.Name(), err)
		}
		observed = append(observed, facts...)
	}

	inferred, err := d.inferrer.Infer(ctx, observed)
	if err != nil {
		return Result{}, fmt.Errorf("inferrer: %w", err)
	}

	return Result{
		Observed: observed,
		Inferred: inferred,
	}, nil
}
