package runner

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
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
