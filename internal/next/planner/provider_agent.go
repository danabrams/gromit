package planner

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// Compile-time interface check.
var _ Agent = (*ProviderPlanAgent)(nil)

// ProviderPlanAgent satisfies the planner.Agent interface by delegating to an
// llmadapter.Invoker. The tier is baked into the LLMAdapter at construction
// time; if a caller passes a different tier to Invoke, the mismatch is logged
// once via sync.Once.
type ProviderPlanAgent struct {
	invoker  llmadapter.Invoker
	adpTier  string
	warnOnce sync.Once
}

// NewProviderPlanAgent creates a ProviderPlanAgent that delegates to the given invoker.
// adapterTier is the tier configured on the underlying LLMAdapter so that
// tier-mismatch warnings can be emitted.
func NewProviderPlanAgent(invoker llmadapter.Invoker, adapterTier string) *ProviderPlanAgent {
	return &ProviderPlanAgent{invoker: invoker, adpTier: adapterTier}
}

// Invoke delegates to the underlying invoker, mapping provider.Result to AgentResult.
// If tier differs from the adapter's configured tier, a warning is logged once.
func (a *ProviderPlanAgent) Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error) {
	if tier != "" && tier != a.adpTier {
		a.warnOnce.Do(func() {
			log.Printf("planner: tier mismatch: caller requested %q but adapter is configured for %q", tier, a.adpTier)
		})
	}
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
