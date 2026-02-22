package runner

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

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
	// The placeholder runCyclesFn returns "TDD cycle execution not yet implemented".
	// If wiring is missing, we'd get a different error (StreamRun failure or "not configured").
	if !strings.Contains(err.Error(), "TDD cycle execution not yet implemented") {
		t.Errorf("Build.Run() error = %q; want error containing %q (proves TDDCycleRunner was wired)",
			err.Error(), "TDD cycle execution not yet implemented")
	}
}
