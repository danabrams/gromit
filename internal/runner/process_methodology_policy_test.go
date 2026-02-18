package runner

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/policy"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

type mockMethodologyPolicy struct {
	IsActiveFn               func(labels []string, methodology string) bool
	PhaseTimeoutFn           func(phase string, beadTimeoutSec int) int
	MinRefactorBudgetFn      func() time.Duration
	MinRevalidationBudgetFn  func() time.Duration
	ShouldDeferPostSuccessFn func(atddActive, tddActive bool) bool
}

var _ policy.MethodologyPolicy = (*mockMethodologyPolicy)(nil)

func (m *mockMethodologyPolicy) IsActive(labels []string, methodology string) bool {
	if m.IsActiveFn != nil {
		return m.IsActiveFn(labels, methodology)
	}
	return false
}

func (m *mockMethodologyPolicy) PhaseTimeout(phase string, beadTimeoutSec int) int {
	if m.PhaseTimeoutFn != nil {
		return m.PhaseTimeoutFn(phase, beadTimeoutSec)
	}
	return 0
}

func (m *mockMethodologyPolicy) MinRefactorBudget() time.Duration {
	if m.MinRefactorBudgetFn != nil {
		return m.MinRefactorBudgetFn()
	}
	return 0
}

func (m *mockMethodologyPolicy) MinRevalidationBudget() time.Duration {
	if m.MinRevalidationBudgetFn != nil {
		return m.MinRevalidationBudgetFn()
	}
	return 0
}

func (m *mockMethodologyPolicy) ShouldDeferPostSuccess(atddActive, tddActive bool) bool {
	if m.ShouldDeferPostSuccessFn != nil {
		return m.ShouldDeferPostSuccessFn(atddActive, tddActive)
	}
	return false
}

func TestPrepareMethodologyForBead_UsesMethodologyPolicyIsActive(t *testing.T) {
	r, _ := newRunnerWithMocks(t, &config.Config{}, Deps{
		Renderer: &mockPromptRenderer{
			RenderTDDBuildFn: func(ctx *prompt.Context) (string, error) {
				return "tdd prompt", nil
			},
		},
	})

	r.methodologyPolicy = &mockMethodologyPolicy{
		IsActiveFn: func(labels []string, methodology string) bool {
			return methodology == "tdd"
		},
	}

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{Labels: []string{}},
		PromptCtx: &prompt.Context{},
		Result:    &IterationResult{},
	}

	atddActive, tddActive, done := r.prepareMethodologyForBead(context.Background(), bc)

	if atddActive {
		t.Fatal("expected atddActive=false")
	}
	if !tddActive {
		t.Fatal("expected tddActive=true")
	}
	if done {
		t.Fatal("expected done=false")
	}
	if bc.BuildPrompt != "tdd prompt" {
		t.Fatalf("expected TDD build prompt, got %q", bc.BuildPrompt)
	}
}
