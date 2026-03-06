package v2

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/stage"
)

func TestRemediationRunnerRunSpecIDRequired(t *testing.T) {
	t.Parallel()

	runner := NewRemediationRunner(RemediationRunnerConfig{})
	err := runner.Run(context.Background(), "")
	if !errors.Is(err, ErrSpecIDRequired) {
		t.Fatalf("expected ErrSpecIDRequired, got %v", err)
	}
}

func TestRemediationRunnerRunAcceptStageRequired(t *testing.T) {
	t.Parallel()

	runner := NewRemediationRunner(RemediationRunnerConfig{})
	err := runner.Run(context.Background(), "spec-id")
	if !errors.Is(err, ErrAcceptStageRequired) {
		t.Fatalf("expected ErrAcceptStageRequired, got %v", err)
	}
}

func TestRemediationRunnerRunBeadRunnerRequired(t *testing.T) {
	t.Parallel()

	accept := &stubStage{
		name: "accept",
		result: &stage.Result{
			Decision: stage.DecisionFail,
		},
	}
	decompose := &stubStage{
		name: "decompose",
		result: &stage.Result{
			Artifacts: &stage.DecomposeArtifacts{
				Beads: []*bead.Bead{{ID: "bead-1"}},
			},
		},
	}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    accept,
		DecomposeStage: decompose,
		GenerationCap:  1,
	})

	err := runner.Run(context.Background(), "spec-id")
	if !errors.Is(err, ErrBeadRunnerRequired) {
		t.Fatalf("expected ErrBeadRunnerRequired, got %v", err)
	}
}

func TestRemediationRunnerRunDecomposeStageRequired(t *testing.T) {
	t.Parallel()

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage: &stubStage{
			name: "accept",
			result: &stage.Result{
				Decision: stage.DecisionFail,
			},
		},
		GenerationCap: 1,
	})

	err := runner.Run(context.Background(), "spec-id")
	if !errors.Is(err, ErrDecomposeStageRequired) {
		t.Fatalf("expected ErrDecomposeStageRequired, got %v", err)
	}
}

func TestRemediationRunnerRunUnexpectedDecomposeArtifacts(t *testing.T) {
	t.Parallel()

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage: &stubStage{
			name: "accept",
			result: &stage.Result{
				Decision: stage.DecisionFail,
			},
		},
		DecomposeStage: &stubStage{
			name:   "decompose",
			result: &stage.Result{Artifacts: "invalid"},
		},
		GenerationCap: 1,
	})

	err := runner.Run(context.Background(), "spec-id")
	if !errors.Is(err, ErrUnexpectedDecomposeArtifacts) {
		t.Fatalf("expected ErrUnexpectedDecomposeArtifacts, got %v", err)
	}
}

type stubStage struct {
	name   string
	result *stage.Result
	err    error
}

func (s *stubStage) Name() string {
	return s.name
}

func (s *stubStage) Run(_ context.Context, _ *stage.Request) (*stage.Result, error) {
	return s.result, s.err
}
