package readinessadapterllm

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/readiness"
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

type trackingRouter struct {
	phase string
}

func (t *trackingRouter) Select(phase, tier string) (provider.Provider, string) {
	t.phase = phase
	return nil, "model"
}

func TestReadinessAdapterWithLLM_AssessShortCircuitsMissingCriteria(t *testing.T) {
	renderer := &dummyPromptRenderer{}
	router := &trackingRouter{}
	adapter := NewReadinessAdapterWithLLM(renderer, router)
	if adapter == nil {
		t.Fatal("expected adapter instance")
	}

	ctx := context.Background()
	b := &bead.Bead{ID: "missing-criteria"}
	assessment, err := adapter.Assess(ctx, b)
	if err != nil {
		t.Fatalf("Assess returned error: %v", err)
	}
	if assessment.Status != readiness.StatusNotReady {
		t.Fatalf("status = %q, want %q", assessment.Status, readiness.StatusNotReady)
	}
	if assessment.Reason != "criteria_count" {
		t.Fatalf("reason = %q, want %q", assessment.Reason, "criteria_count")
	}
	if router.phase != "" {
		t.Fatalf("router.Select called despite missing criteria: phase=%q", router.phase)
	}
}
