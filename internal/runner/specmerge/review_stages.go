package specmerge

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/review"
)

const stageSpecConformance = "spec_conformance"

type ReviewStageDependencies struct {
	Router   Router
	Renderer ReviewPromptRenderer
}

type Router interface {
	Select(phase, tier string) (provider.Provider, string)
}

type ReviewPromptRenderer interface {
	RenderReview(ctx *prompt.ReviewContext) (string, error)
	LoadRulesForPhase(phase string) (string, error)
	LoadSpec(name string) (string, error)
}

// RunStage2SpecConformance runs the spec-conformance review stage on the full diff.
func RunStage2SpecConformance(ctx context.Context, deps ReviewStageDependencies, specName, diff, tier string) (*review.ReviewResult, *provider.Result, error) {
	if deps.Router == nil {
		return nil, nil, fmt.Errorf("router is nil")
	}
	if deps.Renderer == nil {
		return nil, nil, fmt.Errorf("renderer is nil")
	}

	specContent, err := deps.Renderer.LoadSpec(specName)
	if err != nil {
		return nil, nil, fmt.Errorf("load spec %q: %w", specName, err)
	}

	rules, err := deps.Renderer.LoadRulesForPhase(stageSpecConformance)
	if err != nil {
		return nil, nil, fmt.Errorf("load rules for phase %q: %w", stageSpecConformance, err)
	}

	reviewCtx := &prompt.ReviewContext{
		Spec:  specContent,
		Diff:  diff,
		Rules: rules,
	}

	promptText, err := deps.Renderer.RenderReview(reviewCtx)
	if err != nil {
		return nil, nil, fmt.Errorf("render review prompt: %w", err)
	}

	provider, _ := deps.Router.Select(stageSpecConformance, tier)
	if provider == nil {
		return nil, nil, fmt.Errorf("no provider available for phase %q", stageSpecConformance)
	}

	result, err := provider.Run(ctx, promptText, tier)
	if err != nil {
		return nil, nil, fmt.Errorf("provider run: %w", err)
	}

	reviewResult, err := review.ParseReviewResult(result.Output)
	if err != nil {
		return nil, result, fmt.Errorf("parse review result: %w", err)
	}

	return reviewResult, result, nil
}
