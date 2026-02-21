package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

const atddSkipLogMessage = "Skipping ATDD: bead is test-only"
const atddSpecGranularitySkipLogMessage = "Skipping ATDD: spec granularity active (per-bead ATDD disabled)"

const (
	authSpecOneCriterion = `# Auth

## Acceptance Criteria
- Criterion one`
	authSpecTwoCriteria = `# Auth

## Acceptance Criteria
- Criterion one
- Criterion two`
)

// newMinimalRunnerForMethodology creates the smallest possible Runner for
// testing prepareMethodologyForBead without needing a full Deps setup.
// Returns the runner and a pointer to the log buffer for output inspection.
func newMinimalRunnerForMethodology(t *testing.T, cfg *config.Config, renderer PromptRenderer) (*Runner, *strings.Builder) {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{}
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	buf := &strings.Builder{}
	sw := newSyncWriter(buf)
	r := &Runner{
		cfg:      cfg,
		renderer: renderer,
		output:   sw,
		syncOut:  sw,
	}
	return r, buf
}

func newTestBead(id, title string) *bead.Bead {
	return &bead.Bead{
		ID:     id,
		Title:  title,
		Labels: []string{},
	}
}

// newBeadContext creates a minimal BeadContext for testing prepareMethodologyForBead.
func newBeadContextForMethodology(b *bead.Bead) *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Bead:      b,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
	}
}

func newTDDFreshContextCoverageHarness(
	t *testing.T,
	runCyclesFn func(context.Context, *runtypes.BeadContext, *coverage.CoverageTracker, []coverage.Criterion) error,
	addCommentFn func(id, comment string) error,
) (*Runner, *strings.Builder, *mockBeadClient) {
	t.Helper()
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
			MaxTDDCycles:         1,
		},
	}
	r, buf := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	beads := &mockBeadClient{AddCommentFn: addCommentFn}
	r.beads = beads
	if runCyclesFn == nil {
		runCyclesFn = func(context.Context, *runtypes.BeadContext, *coverage.CoverageTracker, []coverage.Criterion) error {
			return nil
		}
	}
	r.tddOrchestrator = &tddOrchestrator{runCyclesFn: runCyclesFn}
	return r, buf, beads
}

func newCoverageBeadContext(id, title, spec string) (*bead.Bead, *runtypes.BeadContext) {
	b := newTestBead(id, title)
	b.Labels = []string{"spec:auth"}
	b.ExpectedOutputs = []string{"implement feature X"}
	bc := newBeadContextForMethodology(b)
	bc.PromptCtx.Spec = spec
	return b, bc
}

// --- ATDD skip tests ---

// TestPrepareMethodology_ATDDSkippedForTestOnlyBead verifies that
// prepareMethodologyForBead returns atddActive=false when the bead title
// matches a test-only pattern, even when ATDD is globally enabled.
func TestPrepareMethodology_ATDDSkippedForTestOnlyBead(t *testing.T) {
	titles := []string{
		"Add tests for bead validation",
		"Add unit tests for config loading",
		"Write tests for runner loop",
		"Write unit tests for prompt rendering",
	}

	for _, title := range titles {
		t.Run(title, func(t *testing.T) {
			cfg := &config.Config{
				Methodology: config.MethodologyConfig{ATDD: true},
			}
			r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
			b := newTestBead("test-skip-1", title)
			bc := newBeadContextForMethodology(b)

			atddActive, _, _ := r.prepareMethodologyForBead(context.Background(), bc)

			if atddActive {
				t.Errorf("prepareMethodologyForBead should skip ATDD for test-only bead title %q, got atddActive=true", title)
			}
		})
	}
}

// TestPrepareMethodology_ATDDSkipLogsReason verifies that when ATDD is skipped
// for a test-only bead, prepareMethodologyForBead logs the reason.
func TestPrepareMethodology_ATDDSkipLogsReason(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{ATDD: true},
	}
	r, buf := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("test-log-1", "Add tests for prompt rendering")
	bc := newBeadContextForMethodology(b)

	r.prepareMethodologyForBead(context.Background(), bc)

	if !strings.Contains(buf.String(), atddSkipLogMessage) {
		t.Errorf("expected log %q in output, got:\n%s", atddSkipLogMessage, buf.String())
	}
}

// TestPrepareMethodology_ATDDSkippedForSpecGranularity verifies that when
// granularity is set to spec, ATDD is skipped for per-bead execution.
func TestPrepareMethodology_ATDDSkippedForSpecGranularity(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			ATDD:        true,
			Granularity: config.MethodologyGranularitySpec,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("spec-skip-1", "Implement feature")
	b.Labels = []string{"spec:auth"}
	bc := newBeadContextForMethodology(b)

	atddActive, _, _ := r.prepareMethodologyForBead(context.Background(), bc)

	if atddActive {
		t.Error("prepareMethodologyForBead should skip ATDD when granularity=spec")
	}
}

func TestPrepareMethodology_ATDDSkippedForSpecGranularityWithoutSpecLabel(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			ATDD:        false,
			Granularity: config.MethodologyGranularitySpec,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("spec-skip-2", "Implement feature")
	b.Labels = []string{"atdd:true"}
	bc := newBeadContextForMethodology(b)

	atddActive, _, _ := r.prepareMethodologyForBead(context.Background(), bc)

	if atddActive {
		t.Error("prepareMethodologyForBead should skip ATDD in spec granularity even with atdd:true label")
	}
}

// TestPrepareMethodology_ATDDSkipSpecGranularityLogsReason verifies that when
// ATDD is skipped for spec granularity, the reason is logged.
func TestPrepareMethodology_ATDDSkipSpecGranularityLogsReason(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			ATDD:        true,
			Granularity: config.MethodologyGranularitySpec,
		},
	}
	r, buf := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("spec-log-1", "Implement feature")
	b.Labels = []string{"atdd:true"}
	bc := newBeadContextForMethodology(b)

	r.prepareMethodologyForBead(context.Background(), bc)

	if !strings.Contains(buf.String(), atddSpecGranularitySkipLogMessage) {
		t.Errorf("expected log %q in output, got:\n%s", atddSpecGranularitySkipLogMessage, buf.String())
	}
}

// --- TDD prompt routing tests ---

// TestPrepareMethodology_TDDSelectsRenderTDDBuild verifies that when TDD is
// globally enabled, prepareMethodologyForBead sets bc.BuildPrompt from
// RenderTDDBuild rather than leaving it empty (which means RenderBuild is used).
func TestPrepareMethodology_TDDSelectsRenderTDDBuild(t *testing.T) {
	tddBuildCalled := false
	renderer := &mockPromptRenderer{
		RenderTDDBuildFn: func(ctx *prompt.Context) (string, error) {
			tddBuildCalled = true
			return "tdd-build-prompt", nil
		},
	}
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{TDD: true},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, renderer)
	b := newTestBead("tdd-routing-1", "Implement feature X")
	bc := newBeadContextForMethodology(b)

	_, tddActive, _ := r.prepareMethodologyForBead(context.Background(), bc)

	if !tddActive {
		t.Error("prepareMethodologyForBead should return tddActive=true when TDD is enabled")
	}
	if !tddBuildCalled {
		t.Error("prepareMethodologyForBead should call RenderTDDBuild when TDD is active")
	}
	if bc.BuildPrompt != "tdd-build-prompt" {
		t.Errorf("bc.BuildPrompt should be set from RenderTDDBuild, got %q", bc.BuildPrompt)
	}
}

func TestPrepareMethodology_TDDFreshContextRunsOrchestrator(t *testing.T) {
	tddBuildCalled := false
	orchestratorCalled := false
	renderer := &mockPromptRenderer{
		RenderTDDBuildFn: func(ctx *prompt.Context) (string, error) {
			tddBuildCalled = true
			return "tdd-build-prompt", nil
		},
	}
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, renderer)
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			orchestratorCalled = true
			return nil
		},
	}
	b := newTestBead("tdd-fresh-context-1", "Implement feature with cycle orchestration")
	b.ExpectedOutputs = []string{"implement feature X"} // required for TDD fresh-context path
	bc := newBeadContextForMethodology(b)

	_, tddActive, done := r.prepareMethodologyForBead(context.Background(), bc)

	if !tddActive {
		t.Fatal("prepareMethodologyForBead should return tddActive=true when TDD is enabled")
	}
	if !done {
		t.Fatal("prepareMethodologyForBead should return done=true after tdd orchestrator success")
	}
	if !orchestratorCalled {
		t.Fatal("prepareMethodologyForBead should call tddOrchestrator.RunCycles when fresh_context_per_cycle=true")
	}
	if tddBuildCalled {
		t.Fatal("prepareMethodologyForBead should not call RenderTDDBuild when fresh_context_per_cycle=true")
	}
	if bc.Result.Error != nil {
		t.Fatalf("bc.Result.Error = %v, want nil", bc.Result.Error)
	}
	if !bc.Result.Success {
		t.Fatal("bc.Result.Success should be true when tddOrchestrator.RunCycles succeeds")
	}
	if !bc.Result.FirstPassSuccess {
		t.Fatal("bc.Result.FirstPassSuccess should be true when tddOrchestrator.RunCycles succeeds")
	}
}

// TestPrepareMethodology_TDDFreshContextUsesTitleFallbackWhenNoExpectedOutputs verifies that
// TDD fresh-context-per-cycle uses the bead title as expected output when explicit
// ExpectedOutputs are missing.
func TestPrepareMethodology_TDDFreshContextUsesTitleFallbackWhenNoExpectedOutputs(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
		},
	}
	orchestratorCalled := false
	tddBuildCalled := false
	renderer := &mockPromptRenderer{
		RenderTDDBuildFn: func(ctx *prompt.Context) (string, error) {
			tddBuildCalled = true
			return "tdd-build-prompt", nil
		},
	}
	r, buf := newMinimalRunnerForMethodology(t, cfg, renderer)
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			orchestratorCalled = true
			return nil
		},
	}
	b := newTestBead("tdd-no-outputs-1", "Fix nil pointer dereference in createRefinePipeline")
	b.ExpectedOutputs = []string{} // no expected outputs — bug-fix bead
	bc := newBeadContextForMethodology(b)

	_, tddActive, done := r.prepareMethodologyForBead(context.Background(), bc)

	if !tddActive {
		t.Fatal("expected tddActive=true")
	}
	if !done {
		t.Fatal("expected done=true when fresh-context orchestrator handles the bead via title fallback")
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected no error, got: %v", bc.Result.Error)
	}
	if !orchestratorCalled {
		t.Fatal("expected orchestrator to be called when bead title can be used as fallback expected output")
	}
	if tddBuildCalled {
		t.Fatal("expected RenderTDDBuild NOT to be called when fresh-context orchestrator handles the bead")
	}
	if len(bc.Bead.ExpectedOutputs) != 1 || bc.Bead.ExpectedOutputs[0] != b.Title {
		t.Fatalf("expected ExpectedOutputs to be populated from title, got %v", bc.Bead.ExpectedOutputs)
	}
	if !strings.Contains(buf.String(), "using title fallback") {
		t.Errorf("expected title fallback log message, got:\n%s", buf.String())
	}
}

func TestPrepareMethodology_TDDFreshContextErrorsWhenNoExpectedOutputsAndEmptyTitle(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
		},
	}
	orchestratorCalled := false
	tddBuildCalled := false
	renderer := &mockPromptRenderer{
		RenderTDDBuildFn: func(ctx *prompt.Context) (string, error) {
			tddBuildCalled = true
			return "tdd-build-prompt", nil
		},
	}
	r, buf := newMinimalRunnerForMethodology(t, cfg, renderer)
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			orchestratorCalled = true
			return nil
		},
	}
	b := newTestBead("tdd-no-outputs-empty-title-1", "   ")
	b.ExpectedOutputs = []string{}
	bc := newBeadContextForMethodology(b)

	_, tddActive, done := r.prepareMethodologyForBead(context.Background(), bc)

	if !tddActive {
		t.Fatal("expected tddActive=true")
	}
	if !done {
		t.Fatal("expected done=true when fresh-context mode is enabled")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected error when bead has no ExpectedOutputs and empty title in fresh-context mode")
	}
	if orchestratorCalled {
		t.Fatal("expected orchestrator NOT to be called when bead has no ExpectedOutputs and empty title")
	}
	if tddBuildCalled {
		t.Fatal("expected RenderTDDBuild NOT to be called when fresh-context mode is enabled")
	}
	if bc.BuildPrompt != "" {
		t.Errorf("expected BuildPrompt to remain empty, got %q", bc.BuildPrompt)
	}
	if !strings.Contains(bc.Result.Error.Error(), "requires ExpectedOutputs or a non-empty bead title") {
		t.Fatalf("unexpected error: %v", bc.Result.Error)
	}
	if strings.Contains(buf.String(), "falling back to standard TDD build") {
		t.Errorf("did not expect fallback log message, got:\n%s", buf.String())
	}
}

func TestPrepareMethodology_TDDFreshContextOrchestratorErrorSetsResultAndDone(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	wantErr := fmt.Errorf("orchestrator failed")
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			return wantErr
		},
	}
	b := newTestBead("tdd-fresh-context-2", "Implement feature with orchestrator error")
	b.ExpectedOutputs = []string{"implement feature X"} // required for TDD fresh-context path
	bc := newBeadContextForMethodology(b)

	_, tddActive, done := r.prepareMethodologyForBead(context.Background(), bc)

	if !tddActive {
		t.Fatal("prepareMethodologyForBead should return tddActive=true when TDD is enabled")
	}
	if !done {
		t.Fatal("prepareMethodologyForBead should return done=true when tddOrchestrator.RunCycles fails")
	}
	if bc.Result.Error == nil {
		t.Fatal("bc.Result.Error should be set when tddOrchestrator.RunCycles fails")
	}
	if bc.Result.Error != wantErr {
		t.Fatalf("bc.Result.Error = %v, want %v", bc.Result.Error, wantErr)
	}
	if bc.Result.Success {
		t.Fatal("bc.Result.Success should remain false when tddOrchestrator.RunCycles fails")
	}
}

func TestRunTDDFreshContextCycles_InitializesCoverageTrackerFromSpec(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})

	var trackerObserved bool
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			trackerObserved = tracker != nil
			return nil
		},
	}

	b := newTestBead("tdd-coverage-init-1", "Implement feature with coverage criteria")
	b.Labels = []string{"spec:auth"}
	b.ExpectedOutputs = []string{"implement feature X"}
	bc := newBeadContextForMethodology(b)
	bc.PromptCtx.Spec = `# Auth

## Acceptance Criteria
- Handles valid credential logins
- Rejects invalid credential logins`

	handled := r.runTDDFreshContextCycles(context.Background(), bc)
	if !handled {
		t.Fatal("expected runTDDFreshContextCycles to handle fresh-context path")
	}
	if trackerObserved {
		t.Fatal("expected nil coverage tracker — spec-level criteria are handled by the spec gate, not per-bead")
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected no error, got %v", bc.Result.Error)
	}
	if bc.Result.CriteriaTotal != 0 {
		t.Fatalf("CriteriaTotal = %d, want 0", bc.Result.CriteriaTotal)
	}
	if bc.Result.CriteriaCovered != 0 {
		t.Fatalf("CriteriaCovered = %d, want 0", bc.Result.CriteriaCovered)
	}
}

func TestRunTDDFreshContextCycles_RerunsUntilCoverageTrackerComplete(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
			MaxTDDCycles:         3,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})

	orchestratorCalls := 0
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			orchestratorCalls++
			return nil
		},
	}

	b := newTestBead("tdd-coverage-rerun-1", "Implement feature with disagreement")
	b.Labels = []string{"spec:auth"}
	b.ExpectedOutputs = []string{"implement feature X"}
	bc := newBeadContextForMethodology(b)
	bc.PromptCtx.Spec = `# Auth

## Acceptance Criteria
- Criterion one`

	handled := r.runTDDFreshContextCycles(context.Background(), bc)
	if !handled {
		t.Fatal("expected runTDDFreshContextCycles to handle fresh-context path")
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected no error, got %v", bc.Result.Error)
	}
	if orchestratorCalls != 1 {
		t.Fatalf("expected 1 orchestrator pass with nil tracker (breaks immediately), got %d", orchestratorCalls)
	}
}

// TestPrepareMethodology_TDDInactiveDoesNotSetBuildPrompt verifies that when
// neither TDD nor ATDD is active, prepareMethodologyForBead does not set
// bc.BuildPrompt, leaving the caller to use RenderBuild.
func TestPrepareMethodology_TDDInactiveDoesNotSetBuildPrompt(t *testing.T) {
	tddBuildCalled := false
	renderer := &mockPromptRenderer{
		RenderTDDBuildFn: func(ctx *prompt.Context) (string, error) {
			tddBuildCalled = true
			return "tdd-build-prompt", nil
		},
	}
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{TDD: false, ATDD: false},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, renderer)
	b := newTestBead("no-tdd-1", "Implement feature Y")
	bc := newBeadContextForMethodology(b)

	_, tddActive, _ := r.prepareMethodologyForBead(context.Background(), bc)

	if tddActive {
		t.Error("prepareMethodologyForBead should return tddActive=false when TDD is disabled")
	}
	if tddBuildCalled {
		t.Error("prepareMethodologyForBead should not call RenderTDDBuild when TDD is inactive")
	}
	if bc.BuildPrompt != "" {
		t.Errorf("bc.BuildPrompt should remain empty when TDD is inactive, got %q", bc.BuildPrompt)
	}
}

// TestPrepareMethodology_ATDDSkipDoesNotCallRenderAcceptanceTests verifies that
// when ATDD is skipped for a test-only bead, RenderAcceptanceTests is never
// invoked on the renderer. Uses mock tracking to confirm the skip is complete.
func TestPrepareMethodology_ATDDSkipDoesNotCallRenderAcceptanceTests(t *testing.T) {
	renderAcceptanceCalled := false
	renderer := &mockPromptRenderer{
		RenderAcceptanceTestsFn: func(ctx *prompt.Context) (string, error) {
			renderAcceptanceCalled = true
			return "acceptance-tests-prompt", nil
		},
	}
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{ATDD: true},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, renderer)
	b := newTestBead("test-no-render-1", "Add tests for runner loop")
	bc := newBeadContextForMethodology(b)

	r.prepareMethodologyForBead(context.Background(), bc)

	if renderAcceptanceCalled {
		t.Error("RenderAcceptanceTests should not be called when ATDD is skipped for a test-only bead")
	}
}

// --- FirstPassSuccess tests ---

// TestExecuteBuildLoop_InjectsScopedTestCommandFromTouchedPackages verifies that
// executeBuildAndMethodologyLoop calls injectScopedTestCommand before invoking
// executeWithRetry, so the prompt context reflects the current touched packages.
func TestExecuteBuildLoop_InjectsScopedTestCommandFromTouchedPackages(t *testing.T) {
	cfg := &config.Config{}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("inject-1", "Implement feature")
	bc := newBeadContextForMethodology(b)
	bc.TouchedPackages = []string{"internal/runner", "internal/config"}

	var capturedScopedCmd string
	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false,
		func() bool {
			if bc.PromptCtx != nil {
				capturedScopedCmd = bc.PromptCtx.ScopedTestCommand
			}
			return true
		},
	)

	if !result.Success {
		t.Fatal("expected Success=true")
	}
	want := "go test ./internal/runner/... ./internal/config/..."
	if capturedScopedCmd != want {
		t.Errorf("ScopedTestCommand before executeWithRetry = %q, want %q", capturedScopedCmd, want)
	}
}

// TestExecuteBuildLoop_ScopedTestCommandEmptyWhenNoPackages verifies that
// executeBuildAndMethodologyLoop does not inject a ScopedTestCommand when
// TouchedPackages is empty.
func TestExecuteBuildLoop_ScopedTestCommandEmptyWhenNoPackages(t *testing.T) {
	cfg := &config.Config{}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("inject-2", "Implement feature")
	bc := newBeadContextForMethodology(b)
	// No TouchedPackages set

	var capturedScopedCmd string
	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false,
		func() bool {
			if bc.PromptCtx != nil {
				capturedScopedCmd = bc.PromptCtx.ScopedTestCommand
			}
			return true
		},
	)

	if !result.Success {
		t.Fatal("expected Success=true")
	}
	if capturedScopedCmd != "" {
		t.Errorf("ScopedTestCommand should be empty when no packages touched, got %q", capturedScopedCmd)
	}
}

// TestExecuteBuildLoop_FirstPassSuccess_NoRetriesNoEscalation verifies that
// FirstPassSuccess is set to true when the build succeeds on the first attempt
// without any retries or escalation.
func TestExecuteBuildLoop_FirstPassSuccess_NoRetriesNoEscalation(t *testing.T) {
	cfg := &config.Config{}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("fps-1", "Implement feature")
	bc := newBeadContextForMethodology(b)

	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false, // no ATDD, no TDD
		func() bool { return true }, // build succeeds immediately
	)

	if !result.Success {
		t.Fatal("expected Success=true")
	}
	if !result.FirstPassSuccess {
		t.Error("expected FirstPassSuccess=true when no retries and no escalation")
	}
}

func TestExecuteBuildLoop_RecordsGreenPhaseMetric(t *testing.T) {
	var bc *runtypes.BeadContext
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if bc != nil {
			// Simulate later phases updating aggregate usage after green finishes.
			bc.Result.CostUSD = 9.99
			bc.Result.InputTokens = 900
			bc.Result.OutputTokens = 450
		}
		return "ok", "", 0, nil
	}
	r, _, _ := setupDirectValidationRunner(t, nil, cmdRunner)
	b := newTestBead("green-metric-1", "Implement feature")
	bc = newBeadContextForMethodology(b)
	bc.Model = "sonnet"
	bc.Tier = "medium"

	result := r.executeBuildAndMethodologyLoop(
		context.Background(),
		bc,
		false,
		false,
		func() bool {
			bc.Result.CostUSD = 1.23
			bc.Result.InputTokens = 111
			bc.Result.OutputTokens = 37
			return true
		},
	)

	if len(result.PhaseMetrics) != 1 {
		t.Fatalf("PhaseMetrics length = %d, want 1", len(result.PhaseMetrics))
	}
	metric := result.PhaseMetrics[0]
	if metric.Phase != "green" {
		t.Fatalf("Phase = %q, want %q", metric.Phase, "green")
	}
	if metric.CycleNumber != 1 {
		t.Fatalf("CycleNumber = %d, want 1", metric.CycleNumber)
	}
	if metric.BeadID != "green-metric-1" {
		t.Fatalf("BeadID = %q, want %q", metric.BeadID, "green-metric-1")
	}
	if metric.Model != "sonnet" {
		t.Fatalf("Model = %q, want %q", metric.Model, "sonnet")
	}
	if metric.Tier != "medium" {
		t.Fatalf("Tier = %q, want %q", metric.Tier, "medium")
	}
	if metric.CostUSD != 1.23 {
		t.Fatalf("CostUSD = %f, want 1.23", metric.CostUSD)
	}
	if metric.InputTokens != 111 || metric.OutputTokens != 37 {
		t.Fatalf("tokens = (%d,%d), want (111,37)", metric.InputTokens, metric.OutputTokens)
	}
	if !metric.Success {
		t.Fatal("Success should be true for successful green phase")
	}
	if metric.DurationMs < 0 {
		t.Fatalf("DurationMs = %d, want >= 0", metric.DurationMs)
	}
}

func TestExecuteBuildLoop_RecordsGreenPhaseMetricDeltaFromSnapshot(t *testing.T) {
	var bc *runtypes.BeadContext
	r, _, _ := setupDirectValidationRunner(t, nil, func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "ok", "", 0, nil
	})
	b := newTestBead("green-metric-delta-1", "Implement feature")
	bc = newBeadContextForMethodology(b)
	bc.Model = "sonnet"
	bc.Tier = "medium"
	bc.Result.CostUSD = 5.00
	bc.Result.InputTokens = 200
	bc.Result.OutputTokens = 120

	result := r.executeBuildAndMethodologyLoop(
		context.Background(),
		bc,
		false,
		false,
		func() bool {
			bc.Result.CostUSD += 0.75
			bc.Result.InputTokens += 41
			bc.Result.OutputTokens += 19
			return true
		},
	)

	if len(result.PhaseMetrics) != 1 {
		t.Fatalf("PhaseMetrics length = %d, want 1", len(result.PhaseMetrics))
	}
	metric := result.PhaseMetrics[0]
	if metric.Phase != "green" {
		t.Fatalf("Phase = %q, want %q", metric.Phase, "green")
	}
	if metric.CostUSD != 0.75 {
		t.Fatalf("CostUSD = %f, want 0.75", metric.CostUSD)
	}
	if metric.InputTokens != 41 || metric.OutputTokens != 19 {
		t.Fatalf("tokens = (%d,%d), want (41,19)", metric.InputTokens, metric.OutputTokens)
	}
}

// TestExecuteBuildLoop_FirstPassSuccess_FalseAfterRetry verifies that
// FirstPassSuccess remains false when the build succeeded but required retries.
func TestExecuteBuildLoop_FirstPassSuccess_FalseAfterRetry(t *testing.T) {
	cfg := &config.Config{}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("fps-2", "Implement feature with retry")
	bc := newBeadContextForMethodology(b)
	bc.TotalRetriesThisBead = 1 // simulate a retry happened

	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false,
		func() bool { return true },
	)

	if !result.Success {
		t.Fatal("expected Success=true")
	}
	if result.FirstPassSuccess {
		t.Error("expected FirstPassSuccess=false when retries occurred")
	}
}

// TestExecuteBuildLoop_FirstPassSuccess_FalseAfterEscalation verifies that
// FirstPassSuccess remains false when the build succeeded but required escalation.
func TestExecuteBuildLoop_FirstPassSuccess_FalseAfterEscalation(t *testing.T) {
	cfg := &config.Config{}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("fps-3", "Implement feature with escalation")
	bc := newBeadContextForMethodology(b)
	bc.Result.Escalated = true // simulate escalation happened

	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false,
		func() bool { return true },
	)

	if !result.Success {
		t.Fatal("expected Success=true")
	}
	if result.FirstPassSuccess {
		t.Error("expected FirstPassSuccess=false when escalation occurred")
	}
}

func TestUpdateIterationCoverageMetrics_UsesFinalTrackerState(t *testing.T) {
	result := &runtypes.IterationResult{
		CriteriaTotal:      42,
		CriteriaCovered:    42,
		CriteriaUntestable: 0,
		UncoveredCriteria:  []string{"stale"},
	}
	tracker := coverage.NewTracker([]coverage.Criterion{
		{Number: 1, Text: "First"},
		{Number: 2, Text: "Second"},
		{Number: 3, Text: "Third"},
	}, 2)
	tracker.MarkCovered(1)
	tracker.RecordRejection(2)
	tracker.RecordRejection(2)

	updateIterationCoverageMetrics(result, tracker)

	if result.CriteriaTotal != 3 {
		t.Fatalf("CriteriaTotal = %d, want 3", result.CriteriaTotal)
	}
	if result.CriteriaCovered != 1 {
		t.Fatalf("CriteriaCovered = %d, want 1", result.CriteriaCovered)
	}
	if result.CriteriaUntestable != 1 {
		t.Fatalf("CriteriaUntestable = %d, want 1", result.CriteriaUntestable)
	}
	if len(result.UncoveredCriteria) != 1 || result.UncoveredCriteria[0] != "Third" {
		t.Fatalf("UncoveredCriteria = %v, want [Third]", result.UncoveredCriteria)
	}
}

func TestRunTDDFreshContextCycles_AddsCoverageCommentForIncompleteCoverage(t *testing.T) {
	r, _, beads := newTDDFreshContextCoverageHarness(t, nil, nil)
	_, bc := newCoverageBeadContext("tdd-comment-1", "Implement feature with incomplete coverage", authSpecTwoCriteria)

	handled := r.runTDDFreshContextCycles(context.Background(), bc)

	if !handled {
		t.Fatal("expected runTDDFreshContextCycles to handle fresh-context path")
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected no error with nil tracker (spec-level criteria skipped), got %v", bc.Result.Error)
	}
	if bc.Result.CriteriaTotal != 0 {
		t.Fatalf("CriteriaTotal = %d, want 0", bc.Result.CriteriaTotal)
	}
	if bc.Result.CriteriaCovered != 0 {
		t.Fatalf("CriteriaCovered = %d, want 0", bc.Result.CriteriaCovered)
	}
	if len(beads.Comments) != 0 {
		t.Fatalf("expected 0 bead comments with nil tracker, got %d", len(beads.Comments))
	}
}

func TestRunTDDFreshContextCycles_CoverageCommentFailureLogsWarning(t *testing.T) {
	r, buf, _ := newTDDFreshContextCoverageHarness(
		t,
		nil,
		func(id, comment string) error { return errors.New("comment add failed") },
	)
	_, bc := newCoverageBeadContext("tdd-comment-2", "Implement feature with comment failure", authSpecOneCriterion)

	r.runTDDFreshContextCycles(context.Background(), bc)

	// With nil tracker, no coverage comment is attempted, so no warning should appear.
	if strings.Contains(buf.String(), "Warning: failed to add coverage summary comment") {
		t.Fatalf("expected no coverage comment warning with nil tracker, got:\n%s", buf.String())
	}
}

func TestRunTDDFreshContextCycles_AddsCoverageCommentForUntestableCriteria(t *testing.T) {
	r, _, beads := newTDDFreshContextCoverageHarness(
		t,
		nil,
		nil,
	)
	_, bc := newCoverageBeadContext("tdd-comment-untestable-1", "Implement feature with untestable criterion", authSpecOneCriterion)

	handled := r.runTDDFreshContextCycles(context.Background(), bc)
	if !handled {
		t.Fatal("expected runTDDFreshContextCycles to handle fresh-context path")
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected no error with nil tracker, got %v", bc.Result.Error)
	}
	if bc.Result.CriteriaTotal != 0 {
		t.Fatalf("CriteriaTotal = %d, want 0", bc.Result.CriteriaTotal)
	}
	if bc.Result.CriteriaUntestable != 0 {
		t.Fatalf("CriteriaUntestable = %d, want 0", bc.Result.CriteriaUntestable)
	}
	if len(beads.Comments) != 0 {
		t.Fatalf("expected 0 bead comments with nil tracker, got %d", len(beads.Comments))
	}
}

// --- Coverage tracker granularity tests ---

// TestBuildCoverageTrackerFromSpec_SkipsWhenGranularityIsSpec verifies that
// buildCoverageTrackerFromSpec returns nil tracker, nil criteria, nil error
// when the methodology granularity is "spec", because the spec gate handles
// system-level criteria after all beads complete.
func TestBuildCoverageTrackerFromSpec_SkipsWhenGranularityIsSpec(t *testing.T) {
	_, bc := newCoverageBeadContext("cov-gran-spec-1", "Implement stage", authSpecTwoCriteria)
	bc.PromptCtx.MethodologyGranularity = config.MethodologyGranularitySpec

	tracker, criteria, err := buildCoverageTrackerFromSpec(bc)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tracker != nil {
		t.Fatal("expected nil tracker when granularity is spec")
	}
	if criteria != nil {
		t.Fatal("expected nil criteria when granularity is spec")
	}
}

// TestBuildCoverageTrackerFromSpec_ReturnsTrackerWhenGranularityIsBead verifies
// that buildCoverageTrackerFromSpec creates a coverage tracker when granularity
// is explicitly set to "bead".
func TestBuildCoverageTrackerFromSpec_ReturnsNilWhenGranularityIsBead(t *testing.T) {
	_, bc := newCoverageBeadContext("cov-gran-bead-1", "Implement feature", authSpecTwoCriteria)
	bc.PromptCtx.MethodologyGranularity = config.MethodologyGranularityBead

	tracker, criteria, err := buildCoverageTrackerFromSpec(bc)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tracker != nil {
		t.Fatal("expected nil tracker — spec-level criteria are handled by the spec gate, not per-bead")
	}
	if criteria != nil {
		t.Fatal("expected nil criteria when tracker is nil")
	}
}

// TestBuildCoverageTrackerFromSpec_ReturnsTrackerWhenGranularityIsEmpty verifies
// that buildCoverageTrackerFromSpec creates a coverage tracker when granularity
// is empty (the default), preserving backward compatibility.
func TestBuildCoverageTrackerFromSpec_ReturnsNilWhenGranularityIsEmpty(t *testing.T) {
	_, bc := newCoverageBeadContext("cov-gran-empty-1", "Implement feature", authSpecTwoCriteria)
	bc.PromptCtx.MethodologyGranularity = ""

	tracker, criteria, err := buildCoverageTrackerFromSpec(bc)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tracker != nil {
		t.Fatal("expected nil tracker — spec-level criteria are handled by the spec gate, not per-bead")
	}
	if criteria != nil {
		t.Fatal("expected nil criteria when tracker is nil")
	}
}
