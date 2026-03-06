package v2

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/stage"
)

func TestRemediationRunnerRunMissingSpecID(t *testing.T) {
	runner := runnerForSpecIDValidation()
	if err := runner.Run(context.Background(), ""); !errors.Is(err, ErrSpecIDRequired) {
		t.Fatalf("expected ErrSpecIDRequired, got %v", err)
	}
}

func TestRemediationRunnerRunMissingAcceptStage(t *testing.T) {
	runner := runnerForAcceptStageValidation()
	if err := runner.Run(context.Background(), "spec"); !errors.Is(err, ErrAcceptStageRequired) {
		t.Fatalf("expected ErrAcceptStageRequired, got %v", err)
	}
}

func TestRemediationRunnerRunMissingBeadRunner(t *testing.T) {
	accept := &unitTestStage{
		name: "accept",
		result: &stage.Result{
			Decision: stage.DecisionFail,
		},
	}
	decompose := &unitTestStage{
		name: "decompose",
		result: &stage.Result{
			Artifacts: &stage.DecomposeArtifacts{
				Beads: []*bead.Bead{{ID: "bead-1"}},
			},
		},
	}

	runner := runnerForMissingBeadRunner(accept, decompose)
	if err := runner.Run(context.Background(), "spec"); !errors.Is(err, ErrBeadRunnerRequired) {
		t.Fatalf("expected ErrBeadRunnerRequired, got %v", err)
	}
}

func TestRemediationRunnerRunMissingDecomposeStage(t *testing.T) {
	accept := &unitTestStage{
		name: "accept",
		result: &stage.Result{Decision: stage.DecisionFail},
	}

	runner := runnerForMissingDecomposeStage(accept)
	if err := runner.Run(context.Background(), "spec"); !errors.Is(err, ErrDecomposeStageRequired) {
		t.Fatalf("expected ErrDecomposeStageRequired, got %v", err)
	}
}

func runnerForSpecIDValidation() *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{})
}

func runnerForAcceptStageValidation() *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{AcceptStage: nil})
}

func runnerForMissingBeadRunner(accept, decompose stage.Stage) *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    accept,
		DecomposeStage: decompose,
		GenerationCap:  1,
	})
}

type unitTestStage struct {
	name   string
	result *stage.Result
	err    error
}

func (s *unitTestStage) Name() string {
	return s.name
}

func (s *unitTestStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}
