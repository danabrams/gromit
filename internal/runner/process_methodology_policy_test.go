package runner

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/policy"
	"github.com/danabrams/gromit/internal/runner/reviewpkg"
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
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		refactorInvoked = true
		return &claude.Result{Success: true}, nil, nil
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

	r.runRefactorAndPostChecks(beadCtx, bc, false, 1)

	if !refactorInvoked {
		t.Fatal("expected refactor phase to run with reduced min refactor budget")
	}
}

func TestRunRefactorAndPostChecks_UsesMethodologyPolicyMinRevalidationBudget(t *testing.T) {
	commandCalls := 0
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		commandCalls++
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)

	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		return &claude.Result{Success: true}, nil, nil
	})
	r.cfg.Refactor.MinFilesChanged = 0
	r.cfg.Validation.Enabled = true

	r.methodologyPolicy = &mockMethodologyPolicy{
		MinRefactorBudgetFn: func() time.Duration {
			return 0
		},
		MinRevalidationBudgetFn: func() time.Duration {
			return 0
		},
	}

	bc := &runtypes.BeadContext{
		Tier:          provider.TierMedium,
		StartCommit:   testStartCommit,
		ParentCtx:     context.Background(),
		BeadTimeout:   20 * time.Second,
		BeadStartTime: time.Now().Add(-10 * time.Second),
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}
	r.runRefactorAndPostChecks(context.Background(), bc, false, 1)

	if commandCalls == 0 {
		t.Fatal("expected re-validation to run with zero min revalidation budget")
	}
}

func TestExecuteBuildAndMethodologyLoop_UsesMethodologyPolicyDeferral(t *testing.T) {
	learnFromSuccess := false
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"true"},
		},
		Review: config.ReviewConfig{
			Enabled: true,
		},
		Loop: config.LoopConfig{
			LearnFromSuccess: &learnFromSuccess,
		},
	}

	mockClaude := &mockClaudeClient{}
	r, _ := newRunnerWithMocks(t, cfg, Deps{
		Router:   newMockRouterFromClaudeClient(mockClaude),
		Renderer: &mockPromptRenderer{},
	})
	mockClaude.RunFn = func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
		return &claude.Result{
			Success: true,
			Output:  `{"passed":true,"fixes_applied":[],"beads_to_create":[],"backlog_items":[],"summary":"ok"}`,
		}, nil
	}
	r.reviewer = reviewpkg.NewReviewer(
		cfg,
		r.router,
		r.beads,
		r.renderer,
		func(startCommit string) (string, error) {
			return "diff --git a/a.go b/a.go\n+line", nil
		},
		r.logger,
	)
	r.reviewer.SetLogFn(r.log)
	r.reviewer.SetValidateFn(func(ctx context.Context, commands []string, workDir string) (bool, error) {
		return true, nil
	})
	r.methodologyPolicy = &mockMethodologyPolicy{
		ShouldDeferPostSuccessFn: func(atddActive, tddActive bool) bool {
			return false
		},
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:    "bead-1",
			Title: "Test bead",
		},
		StartCommit: "abc123",
		Model:       "sonnet",
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	executeWithRetry := func() bool { return true }

	r.executeBuildAndMethodologyLoop(context.Background(), bc, false, false, executeWithRetry)

	if len(mockClaude.RunCalls) != 0 {
		t.Fatal("expected post-success review to be deferred by policy")
	}
}

func TestRunRefactorAndPostChecks_UsesMethodologyPolicyPhaseTimeout(t *testing.T) {
	r, _, _ := setupDirectValidationRunner(t, nil, nil)

	var refactorDeadline time.Time
	var refactorDeadlineSet bool
	setTestRefactorDeps(r, func(ctx context.Context, prompt string, tier string) (*claude.Result, *logger.StreamStats, error) {
		refactorDeadline, refactorDeadlineSet = ctx.Deadline()
		return &claude.Result{Success: true}, nil, nil
	})
	r.cfg.Refactor.MinFilesChanged = 0
	r.cfg.Methodology.PhaseTimeouts = config.MethodologyPhaseTimeout{
		RefactorSeconds: 120,
	}

	r.methodologyPolicy = &mockMethodologyPolicy{
		PhaseTimeoutFn: func(phase string, beadTimeoutSec int) int {
			if phase != "refactor" {
				t.Fatalf("unexpected phase: %s", phase)
			}
			return 10
		},
		MinRefactorBudgetFn: func() time.Duration {
			return 0
		},
	}

	bc := &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		StartCommit: testStartCommit,
		ParentCtx:   context.Background(),
		BeadTimeout: 300 * time.Second,
		PromptCtx: &prompt.Context{
			WorkDir: t.TempDir(),
		},
		Result: &IterationResult{},
	}

	r.runRefactorAndPostChecks(context.Background(), bc, false, 1)

	if !refactorDeadlineSet {
		t.Fatal("expected refactor invoke to receive deadline context")
	}
	untilDeadline := time.Until(refactorDeadline)
	if untilDeadline < 7*time.Second || untilDeadline > 14*time.Second {
		t.Fatalf("refactor deadline unexpected: %s remaining, want ~10s", untilDeadline.Round(time.Second))
	}
}
