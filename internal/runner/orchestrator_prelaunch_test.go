package runner

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestOrchestrator_GateBlock_SetsPrelaunchFailurePhase verifies that when the
// Gate stage returns Block, the resulting IterationLog has FailurePhase set to
// "prelaunch" since no subprocess was launched.
func TestOrchestrator_GateBlock_SetsPrelaunchFailurePhase(t *testing.T) {
	t.Parallel()

	var capturedResult *logger.IterationLog

	beadCount := 0
	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Block, GateBlockReason: "test_block"}, nil
		}},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			if in.Result != nil {
				capturedResult = in.Result
			}
			return pipeline.Output{}, nil
		}},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{ID: "b-1", Title: "Test"}, nil
			}
			return nil, nil
		},
		Config: &config.Config{},
		Output: io.Discard,
	}

	o := NewOrchestrator(cfg)
	_ = o.Run(context.Background(), 1, time.Time{}, nil)

	if capturedResult == nil {
		t.Fatal("epilogue was not called with a Result")
	}
	if capturedResult.FailurePhase != failurephase.Prelaunch {
		t.Errorf("FailurePhase = %q, want %q", capturedResult.FailurePhase, failurephase.Prelaunch)
	}
}

// TestOrchestrator_BranchCheckoutFailure_SetsPrelaunchFailurePhase verifies that
// when branch checkout fails, the IterationLog has FailurePhase "prelaunch".
func TestOrchestrator_BranchCheckoutFailure_SetsPrelaunchFailurePhase(t *testing.T) {
	t.Parallel()

	var capturedResult *logger.IterationLog

	beadCount := 0
	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Build:    &fakeStage{},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			if in.Result != nil {
				capturedResult = in.Result
			}
			return pipeline.Output{}, nil
		}},
		BranchRouter: &mockBranchRouter{},
		GitCheckout: &failingGitCheckout{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{ID: "b-2", Title: "Test", Labels: []string{"spec:auth"}}, nil
			}
			return nil, nil
		},
		Config: &config.Config{},
		Output: io.Discard,
	}

	o := NewOrchestrator(cfg)
	_ = o.Run(context.Background(), 1, time.Time{}, nil)

	if capturedResult == nil {
		t.Fatal("epilogue was not called with a Result")
	}
	if capturedResult.FailurePhase != failurephase.Prelaunch {
		t.Errorf("FailurePhase = %q, want %q", capturedResult.FailurePhase, failurephase.Prelaunch)
	}
}

// failingGitCheckout always returns an error.
type failingGitCheckout struct{}

func (f *failingGitCheckout) CreateOrCheckoutSpecBranch(ctx context.Context, branch string) error {
	return context.DeadlineExceeded
}
