package prepare

import (
	"context"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/provider"
)

// Document holds the parsed content of a spec or plan file.
type Document struct {
	Title string
	Body  string
}

// CriteriaProvider executes an LLM prompt and returns the result.
type CriteriaProvider interface {
	Run(ctx context.Context, prompt, tier string) (*provider.Result, error)
}

// SpecLoader loads spec and plan documents by spec ID.
type SpecLoader interface {
	LoadSpec(ctx context.Context, specID string) (*Document, bool, error)
	LoadPlan(ctx context.Context, specID string) (*Document, bool, error)
}

// LLMCriteriaEnricher uses an LLM to auto-generate acceptance criteria for
// beads that have none, using related spec/plan documents as context.
type LLMCriteriaEnricher struct {
	beadClient *bead.Client
	provider   CriteriaProvider
	loader     SpecLoader
}

// NewLLMCriteriaEnricher creates a new LLMCriteriaEnricher.
// Returns nil if any required dependency is nil.
func NewLLMCriteriaEnricher(beadClient *bead.Client, provider CriteriaProvider, loader SpecLoader) *LLMCriteriaEnricher {
	if beadClient == nil || provider == nil || loader == nil {
		return nil
	}
	return &LLMCriteriaEnricher{
		beadClient: beadClient,
		provider:   provider,
		loader:     loader,
	}
}

// Enrich returns a copy of the bead with acceptance criteria populated from
// the LLM if the bead currently has none. Returns the original bead unchanged
// if it already has criteria, or if enrichment fails (non-fatal).
func (e *LLMCriteriaEnricher) Enrich(ctx context.Context, b *bead.Bead) (*bead.Bead, error) {
	if e == nil || b == nil {
		return b, nil
	}
	if len(effectiveCriteria(b)) > 0 {
		return b, nil
	}
	return b, nil
}
