package runner

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/methodology"
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

func TestRunATDDPreBuildPhases_UsesMethodologyPolicyPhaseTimeout(t *testing.T) {
	r, _ := newRunnerWithMocks(t, &config.Config{}, Deps{
		Renderer: &mockPromptRenderer{
			RenderATDDBuildFn: func(ctx *prompt.Context) (string, error) {
				return "atdd build", nil
			},
		},
	})

	phaseTimeoutSeconds := 17
	r.methodologyPolicy = &mockMethodologyPolicy{
		PhaseTimeoutFn: func(phase string, beadTimeoutSec int) int {
			if phase != "red" {
				t.Fatalf("unexpected phase: %s", phase)
			}
			return phaseTimeoutSeconds
		},
	}

	var deadline time.Time
	var deadlineSet bool
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) error {
		deadline, deadlineSet = ctx.Deadline()
		return nil
	}

	renderFn := func(ctx *prompt.Context) (string, error) {
		return "acceptance prompt", nil
	}
	exec := methodology.NewExecutor(&config.Config{
		Escalation: config.EscalationConfig{MaxRetriesPerModel: 0},
	}, &bytes.Buffer{}, renderFn, invokeFn, nil)
	r.methodologyExec = exec

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "b1"},
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	ok := r.runATDDPreBuildPhases(context.Background(), bc)
	if !ok {
		t.Fatal("expected runATDDPreBuildPhases to succeed")
	}
	if !deadlineSet {
		t.Fatal("expected acceptance invoke to receive deadline context")
	}
	untilDeadline := time.Until(deadline)
	if untilDeadline < 12*time.Second || untilDeadline > 22*time.Second {
		t.Fatalf("deadline unexpected: %s remaining, want ~%ds", untilDeadline.Round(time.Second), phaseTimeoutSeconds)
	}
}

func TestRunRefactorAndPostChecks_UsesMethodologyPolicyMinRefactorBudget(t *testing.T) {
	r, _, _ := setupDirectValidationRunner(t, nil, nil)

	refactorInvoked := false
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, error) {
		refactorInvoked = true
		return &claude.Result{Success: true}, nil
	})
	r.cfg.Refactor.MinFilesChanged = 0
	r.cfg.Validation.Enabled = false

	r.methodologyPolicy = &mockMethodologyPolicy{
		MinRefactorBudgetFn: func() time.Duration {
			return 5 * time.Second
		},
	}

	beadCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bc := &runtypes.BeadContext{
		Tier:          provider.TierMedium,
		StartCommit:   testStartCommit,
		ParentCtx:     context.Background(),
		BeadTimeout:   300 * time.Second,
		BeadStartTime: time.Now(),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	r.runRefactorAndPostChecks(beadCtx, bc, false)

	if !refactorInvoked {
		t.Fatal("expected refactor phase to run with reduced min refactor budget")
	}
}
