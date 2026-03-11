package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/next/fact"
	"github.com/danabrams/gromit/internal/provider"
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

// LLMEnricher uses an LLM provider to infer facts for a given category.
type LLMEnricher struct {
	provider  provider.Provider
	model     string
	reasoning string
}

// NewLLMEnricher creates an LLMEnricher that calls the given provider.
func NewLLMEnricher(p provider.Provider, model, reasoning string) *LLMEnricher {
	return &LLMEnricher{provider: p, model: model, reasoning: reasoning}
}

// llmFact is the JSON shape returned by the LLM.
type llmFact struct {
	Statement    string   `json:"statement"`
	Rationale    string   `json:"rationale"`
	EvidenceRefs []string `json:"evidence_refs"`
	Confidence   string   `json:"confidence"`
	Scope        string   `json:"scope"`
}

// Enrich calls the LLM provider to infer facts for the given category.
func (e *LLMEnricher) Enrich(ctx context.Context, category EnrichmentCategory, observed []fact.Fact, input EnrichInput) (EnrichResult, error) {
	prompt := buildPrompt(category, observed, input)
	tier := provider.TierFromLegacyModel(e.model)

	res, err := e.provider.Run(ctx, prompt, tier)
	if err != nil {
		return EnrichResult{
			Category: category,
			Success:  false,
			Error:    err.Error(),
		}, fmt.Errorf("provider run: %w", err)
	}

	var raw []llmFact
	if err := json.Unmarshal([]byte(res.Output), &raw); err != nil {
		return EnrichResult{
			Category:     category,
			Success:      false,
			Error:        fmt.Sprintf("parse response: %v", err),
			CostUSD:      res.CostUSD,
			InputTokens:  res.InputTokens,
			OutputTokens: res.OutputTokens,
		}, fmt.Errorf("parse response: %w", err)
	}

	now := time.Now()
	facts := make([]InferredFact, 0, len(raw))
	for _, r := range raw {
		f := InferredFact{
			SourceType:   "inferred",
			Category:     category,
			Statement:    r.Statement,
			Rationale:    r.Rationale,
			EvidenceRefs: r.EvidenceRefs,
			Confidence:   r.Confidence,
			Scope:        r.Scope,
			Status:       StatusProposed,
			CreatedAt:    now,
		}
		f.NormalizeNilFields()
		f.FactID = f.ComputeID()
		facts = append(facts, f)
	}

	return EnrichResult{
		Category:     category,
		Facts:        facts,
		FactCount:    len(facts),
		Success:      true,
		CostUSD:      res.CostUSD,
		InputTokens:  res.InputTokens,
		OutputTokens: res.OutputTokens,
	}, nil
}
