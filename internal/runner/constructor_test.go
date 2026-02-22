package runner

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
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
// This test currently fails because the method is a placeholder that returns nil
// without invoking the analyzer.
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

	err := a.ExtractFailureLearning(context.Background(), "bead-1", "Implement feature")
	if err != nil {
		t.Fatalf("ExtractFailureLearning returned unexpected error: %v", err)
	}
	if !called {
		t.Error("analyzer.Analyze was not called; want failure learning extraction to invoke the analyzer")
	}
}
