package prepare

import (
	"context"
	"strings"

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

// BeadUpdater patches a bead with new acceptance criteria.
type BeadUpdater interface {
	UpdateAcceptanceCriteria(ctx context.Context, b *bead.Bead, criteria []string) (*bead.Bead, error)
}

// LLMCriteriaEnricher uses an LLM to auto-generate acceptance criteria for
// beads that have none, using related spec/plan documents as context.
type LLMCriteriaEnricher struct {
	provider CriteriaProvider
	loader   SpecLoader
	updater  BeadUpdater
}

// NewLLMCriteriaEnricher creates a new LLMCriteriaEnricher.
// Returns nil if any required dependency is nil.
func NewLLMCriteriaEnricher(provider CriteriaProvider, loader SpecLoader, updater BeadUpdater) *LLMCriteriaEnricher {
	if provider == nil || loader == nil || updater == nil {
		return nil
	}
	return &LLMCriteriaEnricher{
		provider: provider,
		loader:   loader,
		updater:  updater,
	}
}

// Enrich returns a copy of the bead with acceptance criteria populated from
// the LLM if the bead currently has none. Returns the original bead unchanged
// if it already has criteria or if enrichment yields no usable output.
func (e *LLMCriteriaEnricher) Enrich(ctx context.Context, b *bead.Bead) (*bead.Bead, error) {
	if e == nil || b == nil {
		return b, nil
	}
	if len(effectiveCriteria(b)) > 0 {
		return b, nil
	}

	prompt := e.buildPrompt(ctx, b)
	if strings.TrimSpace(prompt) == "" {
		return b, nil
	}

	result, err := e.provider.Run(ctx, prompt, provider.TierLow)
	if err != nil {
		return b, err
	}
	if result == nil {
		return b, nil
	}

	criteria := parseAcceptanceCriteria(result.Output)
	if len(criteria) == 0 {
		return b, nil
	}
	if len(criteria) > 5 {
		criteria = criteria[:5]
	}

	return e.updater.UpdateAcceptanceCriteria(ctx, b, criteria)
}

func (e *LLMCriteriaEnricher) buildPrompt(ctx context.Context, b *bead.Bead) string {
	sections := e.contextSections(ctx, b)
	if len(sections) == 0 {
		return ""
	}

	return "Generate acceptance criteria for the following work item:\n\n" + strings.Join(sections, "\n\n")
}

func (e *LLMCriteriaEnricher) contextSections(ctx context.Context, b *bead.Bead) []string {
	var sections []string
	if specID := bead.FindSpecLabel(b.Labels); specID != "" {
		if doc, ok, err := e.loader.LoadSpec(ctx, specID); err == nil && ok && doc != nil {
			if formatted := formatDocument("Spec", doc); formatted != "" {
				sections = append(sections, formatted)
			}
		}
		if plan, ok, err := e.loader.LoadPlan(ctx, specID); err == nil && ok && plan != nil {
			if formatted := formatDocument("Plan", plan); formatted != "" {
				sections = append(sections, formatted)
			}
		}
	}

	if len(sections) == 0 {
		if trimmed := strings.TrimSpace(b.Title); trimmed != "" {
			sections = append(sections, "Title: "+trimmed)
		}
		if trimmed := strings.TrimSpace(b.Description); trimmed != "" {
			sections = append(sections, "Description: "+trimmed)
		}
	}

	return sections
}

func formatDocument(label string, doc *Document) string {
	if doc == nil {
		return ""
	}

	var parts []string
	if trimmed := strings.TrimSpace(doc.Title); trimmed != "" {
		parts = append(parts, label+" title: "+trimmed)
	}
	if trimmed := strings.TrimSpace(doc.Body); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}
