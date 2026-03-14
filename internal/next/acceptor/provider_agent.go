package acceptor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// validStatuses is the set of allowed CriterionResult.Status values.
var validStatuses = map[string]bool{
	StatusPass:    true,
	StatusFail:    true,
	StatusUnclear: true,
}

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
	if result == nil {
		return CriterionResult{}, fmt.Errorf("acceptor: provider returned nil result")
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
		return CriterionResult{}, &ParseError{Msg: "parsing criterion result: " + err.Error()}
	}
	if cr.Criterion == "" {
		return CriterionResult{}, &ParseError{Msg: "missing required field: criterion"}
	}
	if cr.Status == "" {
		return CriterionResult{}, &ParseError{Msg: "missing required field: status"}
	}
	if !validStatuses[cr.Status] {
		return CriterionResult{}, &ParseError{Msg: fmt.Sprintf("invalid status %q: must be pass, fail, or unclear", cr.Status)}
	}
	if cr.Rationale == "" {
		return CriterionResult{}, &ParseError{Msg: "missing required field: rationale"}
	}
	cr.NormalizeNilFields()
	return cr, nil
}
