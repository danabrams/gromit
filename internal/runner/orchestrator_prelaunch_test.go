package runner

import (
	"context"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/procutil"
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

// TestOrchestrator_ProviderBinaryMissing_SetsPrelaunchFailure verifies that when
// no configured provider binary exists on PATH, the iteration is recorded as a
// prelaunch failure with GateBlockReason "provider_binary_missing".
func TestOrchestrator_ProviderBinaryMissing_SetsPrelaunchFailure(t *testing.T) {
	t.Parallel()

	// Inject a LookPath that always fails.
	origFn := orchestratorLookPathFn
	orchestratorLookPathFn = func(file string) (string, error) {
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}
	t.Cleanup(func() { orchestratorLookPathFn = origFn })

	var capturedResult *logger.IterationLog
	buildCalled := false

	beadCount := 0
	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Build: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			buildCalled = true
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
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
				return &bead.Bead{ID: "b-4", Title: "Binary Test"}, nil
			}
			return nil, nil
		},
		Config: &config.Config{
			Providers: map[string]config.ProviderDef{
				"claude": {Binary: "nonexistent-claude-binary-xyz"},
			},
		},
		Output: io.Discard,
	}

	o := NewOrchestrator(cfg)
	_ = o.Run(context.Background(), 1, time.Time{}, nil)

	if buildCalled {
		t.Error("Build stage was called; want it skipped when provider binary is missing")
	}
	if capturedResult == nil {
		t.Fatal("epilogue was not called with a Result")
	}
	if capturedResult.FailurePhase != failurephase.Prelaunch {
		t.Errorf("FailurePhase = %q, want %q", capturedResult.FailurePhase, failurephase.Prelaunch)
	}
	if capturedResult.GateBlockReason != "provider_binary_missing" {
		t.Errorf("GateBlockReason = %q, want %q", capturedResult.GateBlockReason, "provider_binary_missing")
	}
}

// TestOrchestrator_ProviderBinaryExists_ProceedsTouild verifies that when the
// provider binary exists on PATH, the Build stage runs normally.
func TestOrchestrator_ProviderBinaryExists_ProceedsToBuild(t *testing.T) {
	t.Parallel()

	// Inject a LookPath that always succeeds.
	origFn := orchestratorLookPathFn
	orchestratorLookPathFn = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}
	t.Cleanup(func() { orchestratorLookPathFn = origFn })

	buildCalled := false

	beadCount := 0
	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Build: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			buildCalled = true
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{ID: "b-5", Title: "Binary Exists Test"}, nil
			}
			return nil, nil
		},
		Config: &config.Config{
			Providers: map[string]config.ProviderDef{
				"claude": {Binary: "some-binary"},
			},
		},
		Output: io.Discard,
	}

	o := NewOrchestrator(cfg)
	_ = o.Run(context.Background(), 1, time.Time{}, nil)

	if !buildCalled {
		t.Error("Build stage was not called; want it to proceed when provider binary exists")
	}
}

// TestOrchestrator_ProcessCapacityExhausted_SetsPrelaunchFailure verifies that
// when WaitForProcessCapacity returns an error before Build, the iteration is
// recorded as a prelaunch failure with GateBlockReason "process_capacity_exhausted".
func TestOrchestrator_ProcessCapacityExhausted_SetsPrelaunchFailure(t *testing.T) {
	t.Parallel()

	// Inject a failing WaitForProcessCapacity function.
	origFn := orchestratorWaitForProcessCapacityFn
	orchestratorWaitForProcessCapacityFn = func(ctx context.Context, maxWait time.Duration) error {
		return &procutil.ProcessCapacityError{Current: 950, Max: 1000, Waited: 3 * time.Second}
	}
	t.Cleanup(func() { orchestratorWaitForProcessCapacityFn = origFn })

	var capturedResult *logger.IterationLog
	buildCalled := false

	beadCount := 0
	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Build: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			buildCalled = true
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
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
				return &bead.Bead{ID: "b-3", Title: "Capacity Test"}, nil
			}
			return nil, nil
		},
		Config: &config.Config{},
		Output: io.Discard,
	}

	o := NewOrchestrator(cfg)
	_ = o.Run(context.Background(), 1, time.Time{}, nil)

	if buildCalled {
		t.Error("Build stage was called; want it skipped when process capacity is exhausted")
	}
	if capturedResult == nil {
		t.Fatal("epilogue was not called with a Result")
	}
	if capturedResult.FailurePhase != failurephase.Prelaunch {
		t.Errorf("FailurePhase = %q, want %q", capturedResult.FailurePhase, failurephase.Prelaunch)
	}
	if capturedResult.GateBlockReason != "process_capacity_exhausted" {
		t.Errorf("GateBlockReason = %q, want %q", capturedResult.GateBlockReason, "process_capacity_exhausted")
	}
}
