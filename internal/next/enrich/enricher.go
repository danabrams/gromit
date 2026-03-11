package enrich

import (
	"context"

	"github.com/danabrams/gromit/internal/next/fact"
)

// EnrichResult holds the output of a single category enrichment pass.
type EnrichResult struct {
	Category     EnrichmentCategory
	Facts        []InferredFact
	FactCount    int
	Success      bool
	Error        string
	CostUSD      float64
	InputTokens  int
	OutputTokens int
}

// CategoryEnricher runs a single enrichment pass for a specific category.
type CategoryEnricher interface {
	Enrich(ctx context.Context, category EnrichmentCategory, observed []fact.Fact, input EnrichInput) (EnrichResult, error)
}

// MockEnricher returns preconfigured results for testing.
type MockEnricher struct {
	Facts []InferredFact
	Err   error
}

func (m *MockEnricher) Enrich(ctx context.Context, category EnrichmentCategory, observed []fact.Fact, input EnrichInput) (EnrichResult, error) {
	if m.Err != nil {
		return EnrichResult{Category: category, Success: false, Error: m.Err.Error()}, m.Err
	}
	return EnrichResult{
		Category:  category,
		Facts:     m.Facts,
		FactCount: len(m.Facts),
		Success:   true,
	}, nil
}
