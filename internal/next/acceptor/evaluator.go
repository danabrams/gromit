package acceptor

import (
	"context"
	"fmt"
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
	agent AcceptAgent
}

// NewEvaluator creates a new Evaluator wrapping the given agent.
func NewEvaluator(agent AcceptAgent) *Evaluator {
	return &Evaluator{agent: agent}
}

// Evaluate runs each criterion through the agent and assembles the result.
func (e *Evaluator) Evaluate(ctx context.Context, input EvaluateInput) (AcceptanceResult, error) {
	results := make([]CriterionResult, 0, len(input.Criteria))
	allPass := true
	hasFailOrUnclear := false

	for _, criterion := range input.Criteria {
		prompt := buildEvalPrompt(criterion, input)
		cr, err := e.agent.EvaluateCriterion(ctx, prompt)
		if err != nil {
			return AcceptanceResult{}, fmt.Errorf("evaluating criterion %q: %w", criterion, err)
		}
		cr.Criterion = criterion
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

func buildEvalPrompt(criterion string, input EvaluateInput) string {
	prompt := fmt.Sprintf("Evaluate this acceptance criterion: %s\n", criterion)
	if input.DiffSummary != "" {
		prompt += fmt.Sprintf("\nDiff summary:\n%s\n", input.DiffSummary)
	}
	if input.TaskResults != "" {
		prompt += fmt.Sprintf("\nTask results:\n%s\n", input.TaskResults)
	}
	if input.ValidationResults != "" {
		prompt += fmt.Sprintf("\nValidation results:\n%s\n", input.ValidationResults)
	}
	if input.ReviewFindings != "" {
		prompt += fmt.Sprintf("\nReview findings:\n%s\n", input.ReviewFindings)
	}
	return prompt
}
