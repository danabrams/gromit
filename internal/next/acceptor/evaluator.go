package acceptor

import (
	"context"
	"errors"
	"fmt"
	"log"
)

// AcceptAgent is the interface for LLM-based criterion evaluation.
type AcceptAgent interface {
	EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error)
}

// EvaluateInput holds all context needed for acceptance evaluation.
type EvaluateInput struct {
	Criteria          []string
	DiffSummary       string
	TaskResults       string
	ValidationResults string
	ReviewFindings    string
}

// Evaluator orchestrates per-criterion acceptance evaluation via an AcceptAgent.
type Evaluator struct {
	agent      AcceptAgent
	timeoutCfg TimeoutConfig
}

// NewEvaluator creates a new Evaluator wrapping the given agent.
func NewEvaluator(agent AcceptAgent, timeoutCfg TimeoutConfig) *Evaluator {
	return &Evaluator{agent: agent, timeoutCfg: timeoutCfg}
}

// Evaluate runs each criterion through the agent and assembles the result.
func (e *Evaluator) Evaluate(ctx context.Context, input EvaluateInput) (AcceptanceResult, error) {
	results := make([]CriterionResult, 0, len(input.Criteria))
	allPass := true
	hasFailOrUnclear := false
	diffSize := len(input.DiffSummary)

	for _, criterion := range input.Criteria {
		prompt, renderErr := RenderAcceptancePrompt(AcceptancePromptInput{
			Criterion:         criterion,
			DiffSummary:       input.DiffSummary,
			TaskResults:       input.TaskResults,
			ValidationResults: input.ValidationResults,
			ReviewFindings:    input.ReviewFindings,
		})
		if renderErr != nil {
			return AcceptanceResult{}, fmt.Errorf("rendering prompt for %q: %w", criterion, renderErr)
		}
		deadline := ComputeCriterionTimeout(e.timeoutCfg, diffSize, criterion)
		log.Printf("criterion_timeout_computed criterion=%q diff_bytes=%d timeout=%s", criterion, diffSize, deadline)
		criterionCtx, cancel := context.WithTimeout(ctx, deadline)
		cr, err := e.agent.EvaluateCriterion(criterionCtx, prompt)
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return AcceptanceResult{}, fmt.Errorf("evaluating criterion %q: deadline exceeded (timeout) (%w)", criterion, err)
			}
			return AcceptanceResult{}, fmt.Errorf("evaluating criterion %q: %w", criterion, err)
		}
		cr.Criterion = criterion

		// Validate status is one of the known values; invalid status triggers retry.
		switch cr.Status {
		case StatusPass, StatusFail, StatusUnclear:
			// valid
		default:
			return AcceptanceResult{}, fmt.Errorf("evaluating criterion %q: invalid status %q (expected pass/fail/unclear)", criterion, cr.Status)
		}

		// Validate that fail/unclear results include a rationale for downstream consumers.
		if cr.Status != StatusPass && cr.Rationale == "" {
			return AcceptanceResult{}, fmt.Errorf("evaluating criterion %q: missing rationale for status %q", criterion, cr.Status)
		}

		cr.NormalizeNilFields()
		results = append(results, cr)

		if cr.Status != StatusPass {
			allPass = false
			hasFailOrUnclear = true
		}
	}

	return AcceptanceResult{
		Results:          results,
		AllPass:          allPass,
		HasFailOrUnclear: hasFailOrUnclear,
	}, nil
}
