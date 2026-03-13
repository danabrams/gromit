package acceptor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// Compile-time interface check.
var _ AcceptAgent = (*ProviderAcceptAgent)(nil)

// ProviderAcceptAgent satisfies AcceptAgent by delegating to an llmadapter.Invoker.
type ProviderAcceptAgent struct {
	invoker llmadapter.Invoker
}

// NewProviderAcceptAgent creates a ProviderAcceptAgent backed by the given invoker.
func NewProviderAcceptAgent(invoker llmadapter.Invoker) *ProviderAcceptAgent {
	return &ProviderAcceptAgent{invoker: invoker}
}

// EvaluateCriterion sends the prompt to the LLM and parses a CriterionResult from the response.
func (a *ProviderAcceptAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	result, err := a.invoker.Invoke(ctx, prompt)
	if err != nil {
		return CriterionResult{}, err
	}
	return ParseCriterionResult(result.Output)
}

// ParseCriterionResult extracts JSON from raw LLM output, unmarshals it into
// a CriterionResult, and normalizes nil fields. It is exported so callers can
// reuse the parsing logic independently.
func ParseCriterionResult(output string) (CriterionResult, error) {
	extracted := llmadapter.ExtractJSON(output)
	var cr CriterionResult
	if err := json.Unmarshal([]byte(extracted), &cr); err != nil {
		return CriterionResult{}, fmt.Errorf("parsing criterion result: %w", err)
	}
	cr.NormalizeNilFields()
	return cr, nil
}
