package v2

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/stage"
)

func TestRemediationRunnerRun_requiresSpecID(t *testing.T) {
	runner := newRunnerForSpecValidation()

	if err := runner.Run(context.Background(), ""); !errors.Is(err, ErrSpecIDRequired) {
		t.Fatalf("expected ErrSpecIDRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresAcceptStage(t *testing.T) {
	runner := newRunnerWithAcceptStage(nil)

	if err := runner.Run(context.Background(), "spec-id"); !errors.Is(err, ErrAcceptStageRequired) {
		t.Fatalf("expected ErrAcceptStageRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresBeadRunner(t *testing.T) {
	artifacts := &stage.DecomposeArtifacts{Beads: []*bead.Bead{}}
	runner := newRunnerForRemediationCycle(newDecisionFailStage(), newDecomposeStageReturning(artifacts), nil, 1)

	if err := runner.Run(context.Background(), "spec-id"); !errors.Is(err, ErrBeadRunnerRequired) {
		t.Fatalf("expected ErrBeadRunnerRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresDecomposeStage(t *testing.T) {
	runner := newRunnerForDecomposeFailure(newDecisionFailStage(), 1)

	if err := runner.Run(context.Background(), "spec-id"); !errors.Is(err, ErrDecomposeStageRequired) {
		t.Fatalf("expected ErrDecomposeStageRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresValidDecomposeArtifacts(t *testing.T) {
	runner := newRunnerForUnexpectedArtifacts(newDecisionFailStage(), newDecomposeStageReturning("unexpected"), 1)

	if err := runner.Run(context.Background(), "spec-id"); !errors.Is(err, ErrUnexpectedDecomposeArtifacts) {
		t.Fatalf("expected ErrUnexpectedDecomposeArtifacts, got %v", err)
	}
}

func newRunnerForSpecValidation() *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{})
}

func newRunnerWithAcceptStage(stage stage.Stage) *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{AcceptStage: stage})
}

func newRunnerForRemediationCycle(accept stage.Stage, decompose stage.Stage, beadRunner BeadRunner, generationCap int) *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    accept,
		DecomposeStage: decompose,
		BeadRunner:     beadRunner,
		GenerationCap:  generationCap,
	})
}

func newRunnerForDecomposeFailure(accept stage.Stage, generationCap int) *RemediationRunner {
	return newRunnerForRemediationCycle(accept, nil, &testBeadRunner{}, generationCap)
}

func newRunnerForUnexpectedArtifacts(accept stage.Stage, decompose stage.Stage, generationCap int) *RemediationRunner {
	return newRunnerForRemediationCycle(accept, decompose, &testBeadRunner{}, generationCap)
}

func newDecisionFailStage() stage.Stage {
	return &testStage{
		name: "decision-fail",
		run: func(ctx context.Context, _ *stage.Request) (*stage.StageResult, error) {
			return &stage.StageResult{Decision: stage.DecisionFail}, nil
		},
	}
}

func newDecomposeStageReturning(artifacts any) stage.Stage {
	return &testStage{
		name: "decompose",
		run: func(ctx context.Context, _ *stage.Request) (*stage.StageResult, error) {
			return &stage.StageResult{Artifacts: artifacts}, nil
		},
	}
}

type testStage struct {
	name string
	run  func(context.Context, *stage.Request) (*stage.StageResult, error)
}

func (s *testStage) Name() string {
	return s.name
}

func (s *testStage) Run(ctx context.Context, req *stage.Request) (*stage.StageResult, error) {
	if s.run == nil {
		return nil, nil
	}
	return s.run(ctx, req)
}

type testBeadRunner struct{}

func (testBeadRunner) Run(context.Context, []*bead.Bead) error {
	return nil
}
