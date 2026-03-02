package readinessadapterllm

import (
    "testing"

    "github.com/danabrams/gromit/internal/prompt"
    "github.com/danabrams/gromit/internal/provider"
)

type dummyRouter struct {
    phase string
    tier  string
}

func (d *dummyRouter) Select(phase, tier string) (provider.Provider, string) {
    d.phase = phase
    d.tier = tier
    return nil, "dummy-model"
}

type dummyPromptRenderer struct{}

func (d *dummyPromptRenderer) RenderReadiness(ctx *prompt.ReadinessContext) (string, error) {
    return "ready", nil
}

func TestNewReadinessAdapterWithLLM_CreatesInstance(t *testing.T) {
    renderer := &dummyPromptRenderer{}
    router := &dummyRouter{}
    if adapter := NewReadinessAdapterWithLLM(renderer, router); adapter == nil {
        t.Fatal("expected adapter instance")
    }
}
