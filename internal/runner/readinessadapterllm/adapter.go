package readinessadapterllm

import (
	"context"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/readiness"
)

type readinessPromptRenderer interface {
	RenderReadiness(ctx *prompt.ReadinessContext) (string, error)
}

type readinessRouter interface {
	Select(phase, tier string) (provider.Provider, string)
}

type readinessAdapterWithLLM struct {
	renderer readinessPromptRenderer
	router   readinessRouter
}

func NewReadinessAdapterWithLLM(renderer readinessPromptRenderer, router readinessRouter) *readinessAdapterWithLLM {
	if renderer == nil || router == nil {
		return nil
	}
	return &readinessAdapterWithLLM{
		renderer: renderer,
		router:   router,
	}
}

func (a *readinessAdapterWithLLM) Assess(ctx context.Context, b *bead.Bead) (readiness.Assessment, error) {
	if assessment, blocked := checkCriteriaCount(b); blocked {
		return assessment, nil
	}

	if assessment, blocked := checkCriteriaMissing(b); blocked {
		return assessment, nil
	}

	if a.renderer == nil {
		return readiness.Assessment{Status: readiness.StatusNotReady}, nil
	}

	renderCtx := &prompt.ReadinessContext{Bead: b}
	promptText, err := a.renderer.RenderReadiness(renderCtx)
	if err != nil {
		return readiness.Assessment{Status: readiness.StatusNotReady}, nil
	}

	if a.router == nil {
		return readiness.Assessment{Status: readiness.StatusNotReady}, nil
	}
	provider, _ := a.router.Select("readiness", "medium")
	if provider == nil {
		return readiness.Assessment{Status: readiness.StatusNotReady}, nil
	}

	_ = promptText
	return readiness.Assessment{Status: readiness.StatusNotReady}, nil
}

func checkCriteriaMissing(b *bead.Bead) (readiness.Assessment, bool) {
	if b == nil {
		return readiness.Assessment{
			Status: readiness.StatusNotReady,
			Reason: prepare.ReasonCriteriaMissing,
		}, true
	}

	for _, output := range b.ExpectedOutputs {
		if strings.TrimSpace(output) != "" {
			return readiness.Assessment{}, false
		}
	}

	if strings.TrimSpace(b.AcceptanceCriteria) != "" {
		return readiness.Assessment{}, false
	}

	return readiness.Assessment{
		Status: readiness.StatusNotReady,
		Reason: prepare.ReasonCriteriaMissing,
	}, true
}

func checkCriteriaCount(b *bead.Bead) (readiness.Assessment, bool) {
	if b == nil {
		return readiness.Assessment{}, false
	}

	count := len(effectiveCriteria(b))
	if count == 0 || count > 3 {
		return readiness.Assessment{
			Status: readiness.StatusNotReady,
			Reason: "criteria_count",
		}, true
	}
	return readiness.Assessment{}, false
}

func effectiveCriteria(b *bead.Bead) []string {
	if b == nil {
		return nil
	}

	outputs := sanitizeOutputs(b.ExpectedOutputs)
	if len(outputs) > 0 {
		return outputs
	}

	return parseAcceptanceCriteria(b.AcceptanceCriteria)
}

func sanitizeOutputs(outputs []string) []string {
	cleaned := make([]string, 0, len(outputs))
	for _, output := range outputs {
		trimmed := strings.TrimSpace(output)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return cleaned
}

func parseAcceptanceCriteria(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		cleaned := strings.TrimSpace(line)
		if cleaned == "" {
			continue
		}
		trimmed = append(trimmed, cleaned)
	}
	return trimmed
}
