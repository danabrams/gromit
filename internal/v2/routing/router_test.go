package routing_test

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/routing"
)

type stubProvider struct {
	name string
}

func (s *stubProvider) Invoke(_ context.Context, _ llmtypes.LLMInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return &llmtypes.LLMInvokeResponse{}, nil
}

func (s *stubProvider) StreamInvoke(_ context.Context, _ llmtypes.LLMStreamInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return &llmtypes.LLMInvokeResponse{}, nil
}

func TestRouterSelectReturnsErrorWhenNoProviders(t *testing.T) {
	r := routing.NewRouter(routing.RouterConfig{})
	_, _, err := r.Select("build", "low")
	if err == nil {
		t.Fatal("expected error when no providers configured, got nil")
	}
}

func TestRouterSelectUsesPhasePreference(t *testing.T) {
	providerA := &stubProvider{name: "providerA"}
	providerB := &stubProvider{name: "providerB"}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"providerA": providerA,
			"providerB": providerB,
		},
		PhasePreferences: map[string]string{
			"build": "providerB",
		},
	})

	got, _, err := r.Select("build", "low")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != llmtypes.LLMProvider(providerB) {
		t.Fatalf("expected providerB selected for build phase, got %v", got)
	}
}
