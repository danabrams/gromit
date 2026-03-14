package planner

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// Compile-time interface check.
var _ Agent = (*ProviderPlanAgent)(nil)

// ProviderPlanAgent satisfies the planner.Agent interface by delegating to an
// llmadapter.Invoker. The tier parameter is ignored because the tier is baked
// into the LLMAdapter at construction time.
type ProviderPlanAgent struct {
	invoker llmadapter.Invoker
}

// NewProviderPlanAgent creates a ProviderPlanAgent that delegates to the given invoker.
func NewProviderPlanAgent(invoker llmadapter.Invoker) *ProviderPlanAgent {
	return &ProviderPlanAgent{invoker: invoker}
}

// Invoke delegates to the underlying invoker, mapping provider.Result to AgentResult.
// The tier parameter is intentionally ignored.
func (a *ProviderPlanAgent) Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error) {
	result, err := a.invoker.Invoke(ctx, prompt)
	// Build partial result for observability even on error
	var ar AgentResult
	if result != nil {
		ar = AgentResult{
			Output:    result.Output,
			TokensIn:  result.InputTokens,
			TokensOut: result.OutputTokens,
			Cost:      result.CostUSD,
			Model:     result.Model,
			Duration:  result.Duration.Milliseconds(),
		}
	}
	if err != nil {
		return ar, err
	}
	if result == nil {
		return AgentResult{}, fmt.Errorf("planner: provider returned nil result")
	}
	return ar, nil
}
