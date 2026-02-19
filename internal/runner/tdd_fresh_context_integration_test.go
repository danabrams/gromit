package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

func TestTDD_FreshContext_FullValidationAfterCycles(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
		},
		Validation: config.ValidationConfig{
			Enabled: true,
			Commands: []string{
				"go test ./...",
			},
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})

	validationCalls := 0
	r.validationRunner = validation.NewRunner(r.cfg, func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validationCalls++
		return "", "", 0, nil
	}, nil, nil)

	orchestratorCalled := false
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			orchestratorCalled = true
			return nil
		},
	}

	b := newTestBead("tdd-validation-1", "Implement feature with cycles")
	b.Labels = []string{"tdd:true"}
	b.ExpectedOutputs = []string{"implement feature"}
	bc := newBeadContextForMethodology(b)

	_, tddActive, done := r.prepareMethodologyForBead(context.Background(), bc)

	if !tddActive {
		t.Fatal("expected tddActive=true")
	}
	if !done {
		t.Fatal("expected done=true when fresh context is enabled")
	}
	if !orchestratorCalled {
		t.Fatal("expected tdd orchestrator to run")
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected no error, got %v", bc.Result.Error)
	}
	if !bc.Result.Success {
		t.Fatal("expected success after fresh-context cycles")
	}
	if validationCalls == 0 {
		t.Fatal("expected validation to run after successful fresh-context cycles")
	}
}

func TestTDD_FreshContext_DelegatesToOrchestrator(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})

	orchestratorCalled := false
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			orchestratorCalled = true
			return nil
		},
	}

	b := newTestBead("tdd-fresh-context-1", "Implement feature with fresh context")
	b.Labels = []string{"tdd:true"}
	b.ExpectedOutputs = []string{"implement feature"}
	bc := newBeadContextForMethodology(b)

	_, tddActive, done := r.prepareMethodologyForBead(context.Background(), bc)

	if !tddActive {
		t.Fatal("expected tddActive=true")
	}
	if !done {
		t.Fatal("expected done=true when orchestrator handles the cycles")
	}
	if !orchestratorCalled {
		t.Fatal("expected fresh-context TDD to call the orchestrator")
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected no error, got %v", bc.Result.Error)
	}
	if !bc.Result.Success {
		t.Fatal("expected success when orchestrator completes without error")
	}
}

func TestTDD_FreshContext_FallsBackOnOrchestratorError(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})

	wantErr := errors.New("orchestrator failed")
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			return wantErr
		},
	}

	b := newTestBead("tdd-fresh-context-2", "Implement feature with orchestrator failure")
	b.Labels = []string{"tdd:true"}
	b.ExpectedOutputs = []string{"implement feature"}
	bc := newBeadContextForMethodology(b)

	_, tddActive, done := r.prepareMethodologyForBead(context.Background(), bc)

	if !tddActive {
		t.Fatal("expected tddActive=true")
	}
	if !done {
		t.Fatal("expected done=true when orchestrator fails")
	}
	if !errors.Is(bc.Result.Error, wantErr) {
		t.Fatalf("expected orchestrator error %v, got %v", wantErr, bc.Result.Error)
	}
	if bc.Result.Success {
		t.Fatal("expected success=false when orchestrator returns an error")
	}
}

func TestTDD_FreshContext_FlagFalse_UsesSingleInvocation(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Methodology: config.MethodologyConfig{
				FreshContextPerCycle: false,
			},
		},
		cycleOrchestrator: &cycleOrchestrator{
			executeFn: func(ctx context.Context, bc *runtypes.BeadContext, atddActive bool, executeWithRetry func() bool) *IterationResult {
				t.Fatal("did not expect cycle orchestrator to execute when fresh_context_per_cycle=false")
				return nil
			},
		},
	}

	executeCalls := 0
	result := r.runMethodologyExecution(
		context.Background(),
		&runtypes.BeadContext{Result: &runtypes.IterationResult{}},
		false,
		true,
		func() bool {
			executeCalls++
			return false
		},
	)

	if executeCalls != 1 {
		t.Fatalf("expected exactly one build invocation, got %d", executeCalls)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestTDD_LabelOverride_TDDFalse_SkipsOrchestrator(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})

	orchestratorCalled := false
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			orchestratorCalled = true
			return nil
		},
	}

	b := newTestBead("tdd-label-override-1", "Implement feature with override")
	b.Labels = []string{"tdd:false"}
	bc := newBeadContextForMethodology(b)

	_, tddActive, done := r.prepareMethodologyForBead(context.Background(), bc)

	if tddActive {
		t.Fatal("expected tddActive=false when bead label has tdd:false")
	}
	if done {
		t.Fatal("expected done=false when TDD is inactive for this bead")
	}
	if orchestratorCalled {
		t.Fatal("expected orchestrator to be skipped when tdd:false label is present")
	}
}

func TestTDD_ConfigToggle_PreservesExistingBehavior(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  false,
			FreshContextPerCycle: true,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})

	orchestratorCalled := false
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			orchestratorCalled = true
			return nil
		},
	}

	b := newTestBead("tdd-config-toggle-1", "Implement feature without TDD")
	bc := newBeadContextForMethodology(b)

	_, tddActive, done := r.prepareMethodologyForBead(context.Background(), bc)

	if tddActive {
		t.Fatal("expected tddActive=false when methodology.tdd is disabled")
	}
	if done {
		t.Fatal("expected done=false when TDD is disabled in config")
	}
	if orchestratorCalled {
		t.Fatal("expected orchestrator not to run when methodology.tdd=false")
	}
}
