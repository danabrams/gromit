package runner

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func (r *Runner) makeCoverageValidateFn() methodology.CoverageValidateFn {
	return func(ctx context.Context, testCode string, criterion coverage.Criterion) (*coverage.ValidationResponse, error) {
		if r.renderer == nil {
			return nil, fmt.Errorf("renderer not configured")
		}
		if r.router == nil {
			return nil, fmt.Errorf("router not configured")
		}
		promptText, err := r.renderer.RenderCoverageValidation(&prompt.CoverageValidationContext{
			TestCode:        testCode,
			CriterionNumber: criterion.Number,
			CriterionText:   criterion.Text,
		})
		if err != nil {
			return nil, fmt.Errorf("rendering coverage validation prompt: %w", err)
		}

		p, modelName := r.router.Select("build", provider.TierLow)
		if p == nil {
			return nil, fmt.Errorf("no providers available for coverage validation")
		}
		result, err := p.Run(ctx, promptText, provider.TierLow)
		if err != nil {
			return nil, fmt.Errorf("coverage validation provider invocation failed (provider=%s model=%s): %w", p.Name(), modelName, err)
		}
		if result == nil {
			return nil, fmt.Errorf("coverage validation returned nil result")
		}
		if !result.Success {
			return nil, fmt.Errorf(
				"coverage validation failed (provider=%s model=%s exit_code=%d): %s",
				p.Name(),
				modelName,
				result.ExitCode,
				runtypes.TruncateOutput(result.Output),
			)
		}

		resp, err := coverage.ParseValidationResponse(result.Output)
		if err != nil {
			return nil, fmt.Errorf("parsing coverage validation response: %w", err)
		}
		return resp, nil
	}
}

func (r *Runner) capturePromptDiagnostics(result *runtypes.IterationResult) {
	if r == nil || r.renderer == nil || result == nil {
		return
	}
	result.PromptDiagnostics = r.renderer.LastDiagnostics()
}

func reconcilePromptDiagnostics(diag *prompt.PromptDiagnostics, inputTokens int) {
	if inputTokens > 0 && diag != nil {
		diag.Reconcile(inputTokens)
	}
}
