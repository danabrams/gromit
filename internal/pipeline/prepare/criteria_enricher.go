package prepare

import (
	"context"
	"fmt"
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

// BeadUpdater persists updated expected outputs for a bead.
type BeadUpdater interface {
	UpdateExpectedOutputs(ctx context.Context, id string, outputs []string) error
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

// Enrich populates acceptance criteria for the bead using the LLM when none exist.
func (e *LLMCriteriaEnricher) Enrich(ctx context.Context, b *bead.Bead) error {
	if e == nil || b == nil {
		return nil
	}
	if len(effectiveCriteria(b)) > 0 {
		return nil
	}

	prompt, err := e.buildPrompt(ctx, b)
	if err != nil {
		return err
	}
	if strings.TrimSpace(prompt) == "" {
		return nil
	}

	result, err := e.provider.Run(ctx, prompt, provider.TierLow)
	if err != nil {
		return err
	}
	if result == nil {
		return nil
	}

	criteria := parseAcceptanceCriteria(result.Output)
	if len(criteria) == 0 {
		return nil
	}
	if len(criteria) > 5 {
		criteria = criteria[:5]
	}

	if err := e.updater.UpdateExpectedOutputs(ctx, b.ID, criteria); err != nil {
		return fmt.Errorf("bd update expected outputs: %w", err)
	}

	clone := *b
	clone.ExpectedOutputs = append([]string(nil), criteria...)
	clone.AcceptanceCriteria = strings.Join(criteria, "\n")
	*b = clone
	return nil
}

func (e *LLMCriteriaEnricher) buildPrompt(ctx context.Context, b *bead.Bead) (string, error) {
	sections, err := e.contextSections(ctx, b)
	if err != nil {
		return "", err
	}
	if len(sections) == 0 {
		return "", nil
	}

	return "Generate acceptance criteria for the following work item:\n\n" + strings.Join(sections, "\n\n"), nil
}

func (e *LLMCriteriaEnricher) contextSections(ctx context.Context, b *bead.Bead) ([]string, error) {
	var sections []string
	if specID := bead.FindSpecLabel(b.Labels); specID != "" {
		doc, ok, err := e.loader.LoadSpec(ctx, specID)
		if err != nil {
			return nil, err
		}
		if ok && doc != nil {
			if formatted := formatDocument("Spec", doc); formatted != "" {
				sections = append(sections, formatted)
			}
		}
		plan, ok, err := e.loader.LoadPlan(ctx, specID)
		if err != nil {
			return nil, err
		}
		if ok && plan != nil {
			if formatted := formatDocument("Plan", plan); formatted != "" {
				sections = append(sections, formatted)
			}
		}
	}

	if len(sections) == 0 {
		var fallback []string
		if trimmed := strings.TrimSpace(b.Title); trimmed != "" {
			fallback = append(fallback, "Title: "+trimmed)
		}
		if trimmed := strings.TrimSpace(b.Description); trimmed != "" {
			fallback = append(fallback, "Description: "+trimmed)
		}
		if len(fallback) > 0 {
			sections = append(sections, strings.Join(fallback, "\n"))
		}
	}

	return sections, nil
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
