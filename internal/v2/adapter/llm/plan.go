package llm

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// PlanLLMAdapter wraps an LLMProvider to implement the adapter.LLMAdapter interface
// for plan generation.
type PlanLLMAdapter struct {
	provider LLMProvider
	specsDir string
}

// NewPlanLLMAdapter creates a PlanLLMAdapter backed by the given LLMProvider
// and specs directory.
func NewPlanLLMAdapter(provider LLMProvider, specsDir string) (*PlanLLMAdapter, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider is required")
	}
	return &PlanLLMAdapter{
		provider: provider,
		specsDir: filepath.Clean(specsDir),
	}, nil
}

// Invoke delegates to the underlying LLMProvider.
func (a *PlanLLMAdapter) Invoke(ctx context.Context, req LLMInvokeRequest) (*LLMInvokeResponse, error) {
	return a.provider.Invoke(ctx, req)
}

// StreamInvoke delegates to the underlying LLMProvider.
func (a *PlanLLMAdapter) StreamInvoke(ctx context.Context, req LLMStreamInvokeRequest) (*LLMStreamInvokeResponse, error) {
	return a.provider.StreamInvoke(ctx, req)
}

// GeneratePlan invokes the LLM to produce a plan for the given spec.
func (a *PlanLLMAdapter) GeneratePlan(ctx context.Context, specID string) (string, error) {
	specID = strings.TrimSpace(specID)
	if specID == "" {
		return "", fmt.Errorf("specID is required")
	}

	prompt := fmt.Sprintf("Generate a plan for spec: %s (specs dir: %s)", specID, a.specsDir)
	resp, err := a.provider.Invoke(ctx, InvokeRequest{
		Prompt: prompt,
		Model:  "sonnet",
	})
	if err != nil {
		return "", fmt.Errorf("generating plan for %s: %w", specID, err)
	}
	return resp.Output, nil
}
