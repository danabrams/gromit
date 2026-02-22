package runner

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
)

// TestDecomposerAdapter_DecomposeSucceeds verifies that decomposerAdapter.Decompose
// returns nil when asked to decompose an oversized bead, indicating the decomposition
// workflow ran (creating child beads) rather than erroring with "not yet implemented".
func TestDecomposerAdapter_DecomposeSucceeds(t *testing.T) {
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	client.RunFn = func(args ...string) (string, error) {
		if len(args) > 0 && args[0] == "create" {
			return `{"id":"child-1","title":"part 1","status":"open"}`, nil
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client}
	b := &bead.Bead{
		ID:              "over-1",
		Title:           "oversized bead",
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
	}

	if err := adapter.Decompose(context.Background(), b); err != nil {
		t.Fatalf("Decompose returned error: %v; want nil for a decomposable oversized bead", err)
	}
}

// stubFailureAnalyzer is a test double for FailureAnalyzer.
type stubFailureAnalyzer struct {
	fn func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error)
}

func (s *stubFailureAnalyzer) Analyze(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
	if s.fn != nil {
		return s.fn(ctx, b, output)
	}
	return nil, nil
}

// TestFailureLearnerAdapter_CallsAnalyzer verifies that failureLearnerAdapter
// calls the analyzer when ExtractFailureLearning is invoked on the failure path.
func TestFailureLearnerAdapter_CallsAnalyzer(t *testing.T) {
	called := false
	stub := &stubFailureAnalyzer{
		fn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			called = true
			return nil, nil
		},
	}
	a := &failureLearnerAdapter{
		analyzer: stub,
	}

	err := a.ExtractFailureLearning(context.Background(), "bead-1", "Implement feature", "FAIL: TestFoo")
	if err != nil {
		t.Fatalf("ExtractFailureLearning returned unexpected error: %v", err)
	}
	if !called {
		t.Error("analyzer.Analyze was not called; want failure learning extraction to invoke the analyzer")
	}
}

// TestBuildTDDCycleRunner_ReturnsTDDPipelineAdapter verifies that buildTDDCycleRunner
// returns a non-nil *TDDPipelineAdapter so that the Build stage can delegate TDD
// cycles to the runner-backed orchestrator.
func TestBuildTDDCycleRunner_ReturnsTDDPipelineAdapter(t *testing.T) {
	cfg := &config.Config{}
	result := buildTDDCycleRunner(cfg, nil, nil, io.Discard)
	if result == nil {
		t.Fatal("buildTDDCycleRunner returned nil TDDCycleRunner")
	}
	if _, ok := result.(*TDDPipelineAdapter); !ok {
		t.Fatalf("buildTDDCycleRunner returned %T, want *TDDPipelineAdapter", result)
	}
}

// TestBuildTDDCycleRunner_RunnerHasConfiguredOrchestrator verifies that the adapter
// returned by buildTDDCycleRunner has a non-nil tddOrchestrator with a configured
// runCyclesFn, so TDD cycle invocations will execute rather than error with "not configured".
func TestBuildTDDCycleRunner_RunnerHasConfiguredOrchestrator(t *testing.T) {
	cfg := &config.Config{}
	result := buildTDDCycleRunner(cfg, nil, nil, io.Discard)
	adapter, ok := result.(*TDDPipelineAdapter)
	if !ok {
		t.Fatalf("expected *TDDPipelineAdapter, got %T", result)
	}
	if adapter.runner.tddOrchestrator == nil {
		t.Fatal("runner.tddOrchestrator is nil; want configured orchestrator")
	}
	if adapter.runner.tddOrchestrator.runCyclesFn == nil {
		t.Fatal("runner.tddOrchestrator.runCyclesFn is nil; want configured run cycles function")
	}
}

// TestOptionalTDDCycleRunner_ReturnsNilWhenFreshContextDisabled verifies that
// optionalTDDCycleRunner returns nil when FreshContextPerCycle is false, so the
// Build stage falls back to single-invocation StreamRun.
func TestOptionalTDDCycleRunner_ReturnsNilWhenFreshContextDisabled(t *testing.T) {
	cfg := &config.Config{}
	result := optionalTDDCycleRunner(cfg, nil, nil, io.Discard)
	if result != nil {
		t.Errorf("optionalTDDCycleRunner returned %T, want nil when FreshContextPerCycle is false", result)
	}
}

// TestOptionalTDDCycleRunner_ReturnsAdapterWhenFreshContextEnabled verifies that
// optionalTDDCycleRunner returns a non-nil TDDCycleRunner when FreshContextPerCycle
// is true, so the Build stage can delegate to per-cycle TDD orchestration.
func TestOptionalTDDCycleRunner_ReturnsAdapterWhenFreshContextEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Methodology.FreshContextPerCycle = true
	result := optionalTDDCycleRunner(cfg, nil, nil, io.Discard)
	if result == nil {
		t.Fatal("optionalTDDCycleRunner returned nil, want non-nil TDDCycleRunner when FreshContextPerCycle is true")
	}
}

// TestFailureLearnerAdapter_ForwardsFailureOutput verifies that the failureOutput
// string passed to ExtractFailureLearning reaches the analyzer.Analyze call.
func TestFailureLearnerAdapter_ForwardsFailureOutput(t *testing.T) {
	var receivedOutput string
	stub := &stubFailureAnalyzer{
		fn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			receivedOutput = output
			return nil, nil
		},
	}
	a := &failureLearnerAdapter{
		analyzer: stub,
	}

	err := a.ExtractFailureLearning(context.Background(), "bead-1", "Implement feature", "FAIL: TestFoo\nexpected 1 got 2")
	if err != nil {
		t.Fatalf("ExtractFailureLearning returned unexpected error: %v", err)
	}
	if receivedOutput != "FAIL: TestFoo\nexpected 1 got 2" {
		t.Errorf("analyzer received output %q, want %q", receivedOutput, "FAIL: TestFoo\nexpected 1 got 2")
	}
}

// TestNewRunnerImpl_BuildStageUsesTDDCycleRunner_WhenFreshContextPerCycle verifies
// that newRunnerImpl wires the TDDCycleRunner into the Build stage when
// FreshContextPerCycle is true. The test exercises the Build stage with a TDD bead
// and checks that it delegates to the TDDCycleRunner (identified by the distinctive
// error from the placeholder runCyclesFn) instead of falling back to StreamRun.
func TestNewRunnerImpl_BuildStageUsesTDDCycleRunner_WhenFreshContextPerCycle(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	_ = os.MkdirAll(filepath.Join(gromitDir, "templates"), 0o755)
	_ = os.MkdirAll(filepath.Join(gromitDir, "specs"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "logs"), 0o755)

	cfg := &config.Config{}
	cfg.Paths.Templates = filepath.Join(gromitDir, "templates")
	cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
	cfg.Paths.Logs = filepath.Join(tmpDir, "logs")
	cfg.Methodology.TDD = true
	cfg.Methodology.FreshContextPerCycle = true

	orch, err := newRunnerImpl(cfg, io.Discard, nil)
	if err != nil {
		t.Fatalf("newRunnerImpl: %v", err)
	}

	// Extract the Build stage and run it with a TDD bead.
	buildStage := orch.cfg.Build
	tddBead := &bead.Bead{
		ID:     "test-bead-1",
		Title:  "Test feature",
		Labels: []string{"tdd:true"},
	}
	in := pipeline.Input{
		Bead:   tddBead,
		Config: cfg,
	}
	_, err = buildStage.Run(context.Background(), in)
	if err == nil {
		t.Fatal("Build.Run() returned nil error; want TDD cycle runner error proving delegation")
	}
	// The real runCyclesFn delegates to the CycleOrchestrator which attempts
	// red-phase rendering. The error proves Build delegated to TDDCycleRunner.
	if !strings.Contains(err.Error(), "TDD cycle runner") {
		t.Errorf("Build.Run() error = %q; want error containing %q (proves TDDCycleRunner was wired)",
			err.Error(), "TDD cycle runner")
	}
}

// TestNewRunnerImpl_StatusWriterComputesTimeBudgetFromDeadline verifies that the
// StatusWriter closure created by newRunnerImpl computes timeBudgetMinutes from the
// deadline parameter instead of hardcoding 0. When the deadline is 30 minutes away,
// the status.json should contain time_budget_minutes ≈ 30, not 0.
func TestNewRunnerImpl_StatusWriterComputesTimeBudgetFromDeadline(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	_ = os.MkdirAll(filepath.Join(gromitDir, "templates"), 0o755)
	_ = os.MkdirAll(filepath.Join(gromitDir, "specs"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "logs"), 0o755)

	cfg := &config.Config{}
	cfg.Paths.Templates = filepath.Join(gromitDir, "templates")
	cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
	cfg.Paths.Logs = filepath.Join(tmpDir, "logs")

	orch, err := newRunnerImpl(cfg, io.Discard, nil)
	if err != nil {
		t.Fatalf("newRunnerImpl: %v", err)
	}

	// Call the StatusWriter callback with a deadline 30 minutes from now.
	deadline := time.Now().Add(30 * time.Minute)
	orch.cfg.StatusWriter(1, "bead-1", "Test bead", deadline)

	// Read the status.json that was written.
	statusPath := filepath.Join(gromitDir, "status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("reading status.json: %v", err)
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("unmarshaling status.json: %v", err)
	}

	// The constructor currently hardcodes 0. After the fix, it should compute
	// timeBudgetMinutes from the deadline (~30 minutes from now).
	if status.TimeBudgetMinutes == 0 {
		t.Errorf("status.TimeBudgetMinutes = 0; want non-zero value computed from deadline (%v from now)",
			time.Until(deadline).Round(time.Minute))
	}
	if status.TimeBudgetMinutes < 29 || status.TimeBudgetMinutes > 31 {
		t.Errorf("status.TimeBudgetMinutes = %d; want approximately 30 (computed from deadline)",
			status.TimeBudgetMinutes)
	}
}

// TestNewRunnerImpl_GateStageHasDecomposerConfigured verifies that newRunnerImpl
// wires a Decomposer implementation into the Gate stage so that oversized beads
// can be auto-decomposed instead of blocked.
func TestNewRunnerImpl_GateStageHasDecomposerConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	_ = os.MkdirAll(filepath.Join(gromitDir, "templates"), 0o755)
	_ = os.MkdirAll(filepath.Join(gromitDir, "specs"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "logs"), 0o755)

	cfg := &config.Config{}
	cfg.Paths.Templates = filepath.Join(gromitDir, "templates")
	cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
	cfg.Paths.Logs = filepath.Join(tmpDir, "logs")

	orch, err := newRunnerImpl(cfg, io.Discard, nil)
	if err != nil {
		t.Fatalf("newRunnerImpl: %v", err)
	}

	// Type-assert the Gate to *prepare.Gate to verify it's configured with a Decomposer.
	gateStage, ok := orch.cfg.Gate.(*prepare.Gate)
	if !ok {
		t.Fatalf("Gate stage is %T, want *prepare.Gate", orch.cfg.Gate)
	}

	// The Gate should have a Decomposer configured so it can auto-decompose
	// oversized beads instead of blocking them.
	if !gateStage.HasDecomposer() {
		t.Fatal("Gate.HasDecomposer() returned false; want Decomposer wired in constructor for scope-triggered auto-decomposition")
	}
}
