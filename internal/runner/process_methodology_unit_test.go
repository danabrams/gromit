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
const atddSpecGranularitySkipLogMessage = "Skipping ATDD: spec granularity active for spec:"

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
// granularity is set to spec and a bead has a spec label, ATDD is skipped.
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
		t.Error("prepareMethodologyForBead should skip ATDD when granularity=spec and bead has spec label")
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
	b.Labels = []string{"spec:auth"}
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

func TestPrepareMethodology_TDDFreshContextFallsBackWhenNoExpectedOutputsAndEmptyTitle(t *testing.T) {
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
	if done {
		t.Fatal("expected done=false so caller falls through to normal build loop")
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected no error, got: %v", bc.Result.Error)
	}
	if orchestratorCalled {
		t.Fatal("expected orchestrator NOT to be called when bead has no ExpectedOutputs and empty title")
	}
	if !tddBuildCalled {
		t.Fatal("expected RenderTDDBuild to be called as fallback")
	}
	if bc.BuildPrompt != "tdd-build-prompt" {
		t.Errorf("expected BuildPrompt from RenderTDDBuild fallback, got %q", bc.BuildPrompt)
	}
	if !strings.Contains(buf.String(), "falling back to standard TDD build") {
		t.Errorf("expected fallback log message, got:\n%s", buf.String())
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

	var gotCriteria []coverage.Criterion
	var trackerObserved bool
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			trackerObserved = tracker != nil
			gotCriteria = append([]coverage.Criterion(nil), criteria...)
			for _, criterion := range criteria {
				tracker.MarkCovered(criterion.Number)
			}
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
	if !trackerObserved {
		t.Fatal("expected a coverage tracker to be initialized when spec criteria are present")
	}
	if len(gotCriteria) != 2 {
		t.Fatalf("expected 2 parsed coverage criteria, got %d", len(gotCriteria))
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected no error, got %v", bc.Result.Error)
	}
	if bc.Result.CriteriaTotal != 2 {
		t.Fatalf("CriteriaTotal = %d, want 2", bc.Result.CriteriaTotal)
	}
	if bc.Result.CriteriaCovered != 2 {
		t.Fatalf("CriteriaCovered = %d, want 2", bc.Result.CriteriaCovered)
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
			if orchestratorCalls == 1 {
				// Simulate Claude self-reporting completion while tracker still has unchecked criteria.
				return nil
			}
			if tracker != nil && len(criteria) > 0 {
				tracker.MarkCovered(criteria[0].Number)
			}
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
	if orchestratorCalls != 2 {
		t.Fatalf("expected 2 orchestrator passes when tracker remains incomplete, got %d", orchestratorCalls)
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
	cfg := &config.Config{}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("green-metric-1", "Implement feature")
	bc := newBeadContextForMethodology(b)
	bc.Model = "sonnet"
	bc.Tier = "medium"

	result := r.executeBuildAndMethodologyLoop(
		context.Background(),
		bc,
		false,
		false,
		func() bool {
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
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
			MaxTDDCycles:         1,
		},
	}
	r, buf := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	beads := &mockBeadClient{}
	r.beads = beads
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			return nil
		},
	}

	b := newTestBead("tdd-comment-1", "Implement feature with incomplete coverage")
	b.Labels = []string{"spec:auth"}
	b.ExpectedOutputs = []string{"implement feature X"}
	bc := newBeadContextForMethodology(b)
	bc.PromptCtx.Spec = `# Auth

## Acceptance Criteria
- Criterion one
- Criterion two`

	handled := r.runTDDFreshContextCycles(context.Background(), bc)

	if !handled {
		t.Fatal("expected runTDDFreshContextCycles to handle fresh-context path")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected error when coverage remains incomplete after max cycles")
	}
	if bc.Result.CriteriaTotal != 2 {
		t.Fatalf("CriteriaTotal = %d, want 2", bc.Result.CriteriaTotal)
	}
	if bc.Result.CriteriaCovered != 0 {
		t.Fatalf("CriteriaCovered = %d, want 0", bc.Result.CriteriaCovered)
	}
	if bc.Result.CriteriaUntestable != 0 {
		t.Fatalf("CriteriaUntestable = %d, want 0", bc.Result.CriteriaUntestable)
	}
	if len(bc.Result.UncoveredCriteria) != 2 {
		t.Fatalf("UncoveredCriteria len = %d, want 2", len(bc.Result.UncoveredCriteria))
	}
	if len(beads.Comments) != 1 {
		t.Fatalf("expected 1 bead comment, got %d", len(beads.Comments))
	}
	if beads.Comments[0].ID != b.ID {
		t.Fatalf("comment bead id = %q, want %q", beads.Comments[0].ID, b.ID)
	}
	if !strings.Contains(beads.Comments[0].Comment, "Coverage: 0/2 criteria covered") {
		t.Fatalf("comment %q missing coverage summary", beads.Comments[0].Comment)
	}
	if !strings.Contains(buf.String(), "TDD coverage summary for bead tdd-comment-1") {
		t.Fatalf("expected coverage summary log, got:\n%s", buf.String())
	}
}

func TestRunTDDFreshContextCycles_CoverageCommentFailureLogsWarning(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
			MaxTDDCycles:         1,
		},
	}
	r, buf := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	beads := &mockBeadClient{
		AddCommentFn: func(id, comment string) error {
			return errors.New("comment add failed")
		},
	}
	r.beads = beads
	r.tddOrchestrator = &tddOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			return nil
		},
	}

	b := newTestBead("tdd-comment-2", "Implement feature with comment failure")
	b.Labels = []string{"spec:auth"}
	b.ExpectedOutputs = []string{"implement feature X"}
	bc := newBeadContextForMethodology(b)
	bc.PromptCtx.Spec = `# Auth

## Acceptance Criteria
- Criterion one`

	r.runTDDFreshContextCycles(context.Background(), bc)

	if !strings.Contains(buf.String(), "Warning: failed to add coverage summary comment: comment add failed") {
		t.Fatalf("expected warning log when AddComment fails, got:\n%s", buf.String())
	}
}
