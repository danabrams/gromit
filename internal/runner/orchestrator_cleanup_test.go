package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// cleanupTrackingCheckout tracks calls to RevertAndReturnToBase.
type cleanupTrackingCheckout struct {
	RevertAndReturnToBaseFn  func(ctx context.Context) error
	CreateOrCheckoutErr      error
	revertAndReturnBaseCalls int
	checkoutCalls            []string
}

func (c *cleanupTrackingCheckout) CreateOrCheckoutSpecBranch(ctx context.Context, specBranchName string) error {
	c.checkoutCalls = append(c.checkoutCalls, specBranchName)
	return c.CreateOrCheckoutErr
}

func (c *cleanupTrackingCheckout) RevertAndReturnToBase(ctx context.Context) error {
	c.revertAndReturnBaseCalls++
	if c.RevertAndReturnToBaseFn != nil {
		return c.RevertAndReturnToBaseFn(ctx)
	}
	return nil
}

// TestOrchestrator_BuildFailure_CallsRevertAndReturnToBase verifies that after
// a build failure, RevertAndReturnToBase is called on GitCheckout.
func TestOrchestrator_BuildFailure_CallsRevertAndReturnToBase(t *testing.T) {
	t.Parallel()

	checkout := &cleanupTrackingCheckout{}
	beadCount := 0

	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Build: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{}, errors.New("build exploded")
		}},
		Validate:     &fakeStage{},
		Epilogue:     &fakeStage{},
		BranchRouter: &mockBranchRouter{},
		GitCheckout:  checkout,
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{ID: "cleanup-build", Title: "Build Fail", Labels: []string{"spec:auth"}}, nil
			}
			return nil, nil
		},
		Config: &config.Config{},
		Output: io.Discard,
	}

	o := NewOrchestrator(cfg)
	_ = o.Run(context.Background(), 1, time.Time{}, make(chan struct{}))

	if checkout.revertAndReturnBaseCalls == 0 {
		t.Fatal("RevertAndReturnToBase was not called after build failure")
	}
}

// TestOrchestrator_ValidationFailure_CallsRevertAndReturnToBase verifies that
// after a validation failure, RevertAndReturnToBase is called on GitCheckout.
func TestOrchestrator_ValidationFailure_CallsRevertAndReturnToBase(t *testing.T) {
	t.Parallel()

	checkout := &cleanupTrackingCheckout{}
	beadCount := 0

	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Build: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Validate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{
				Decision:           pipeline.Block,
				ValidationFailures: []string{"test failed"},
			}, nil
		}},
		Epilogue:     &fakeStage{},
		BranchRouter: &mockBranchRouter{},
		GitCheckout:  checkout,
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{ID: "cleanup-validate", Title: "Validate Fail", Labels: []string{"spec:auth"}}, nil
			}
			return nil, nil
		},
		Config: &config.Config{},
		Output: io.Discard,
	}

	o := NewOrchestrator(cfg)
	_ = o.Run(context.Background(), 1, time.Time{}, make(chan struct{}))

	if checkout.revertAndReturnBaseCalls == 0 {
		t.Fatal("RevertAndReturnToBase was not called after validation failure")
	}
}

// TestOrchestrator_BranchCheckoutFailure_CallsRevertAndReturnToBase verifies
// that non-dirty checkout failures still trigger cleanup before continuing.
func TestOrchestrator_BranchCheckoutFailure_CallsRevertAndReturnToBase(t *testing.T) {
	t.Parallel()

	checkout := &cleanupTrackingCheckout{
		CreateOrCheckoutErr: errors.New("checkout failed"),
	}
	beadCount := 0

	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Build:        &fakeStage{},
		Validate:     &fakeStage{},
		Epilogue:     &fakeStage{},
		BranchRouter: &mockBranchRouter{},
		GitCheckout:  checkout,
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{ID: "cleanup-checkout-fail", Title: "Checkout Fail", Labels: []string{"spec:auth"}}, nil
			}
			return nil, nil
		},
		Config: &config.Config{},
		Output: io.Discard,
	}

	o := NewOrchestrator(cfg)
	_ = o.Run(context.Background(), 1, time.Time{}, make(chan struct{}))

	if checkout.revertAndReturnBaseCalls == 0 {
		t.Fatal("RevertAndReturnToBase was not called after non-dirty checkout failure")
	}
}

// TestOrchestrator_CleanupFailure_LogsWarningAndContinues verifies that when
// RevertAndReturnToBase returns an error, it is logged as a warning but does
// not stop the run. The next iteration proceeds normally.
func TestOrchestrator_CleanupFailure_LogsWarningAndContinues(t *testing.T) {
	t.Parallel()

	checkout := &cleanupTrackingCheckout{
		RevertAndReturnToBaseFn: func(ctx context.Context) error {
			return errors.New("cleanup kaboom")
		},
	}
	buf := &bytes.Buffer{}
	beadCount := 0
	buildCallCount := 0

	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Build: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			buildCallCount++
			if buildCallCount == 1 {
				return pipeline.Output{}, errors.New("build exploded")
			}
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Validate:     &fakeStage{},
		Epilogue:     &fakeStage{},
		BranchRouter: &mockBranchRouter{},
		GitCheckout:  checkout,
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			switch beadCount {
			case 1:
				return &bead.Bead{ID: "fail-cleanup-1", Title: "Fail Cleanup 1", Labels: []string{"spec:auth"}}, nil
			case 2:
				return &bead.Bead{ID: "succeed-2", Title: "Succeed 2", Labels: []string{"spec:auth"}}, nil
			default:
				return nil, nil
			}
		},
		Config: &config.Config{},
		Output: buf,
	}

	o := NewOrchestrator(cfg)
	_ = o.Run(context.Background(), 0, time.Time{}, make(chan struct{}))

	// The cleanup failure should be logged as a warning.
	output := buf.String()
	if !strings.Contains(output, "cleanup kaboom") {
		t.Fatalf("expected warning about cleanup failure in output, got: %q", output)
	}

	// The second bead should still have been processed (build called twice).
	if buildCallCount < 2 {
		t.Fatalf("expected build to be called at least 2 times (second bead processed), got %d", buildCallCount)
	}
}

// TestOrchestrator_NilGitCheckout_SkipsCleanup verifies that when GitCheckout
// is nil, cleanup is not called and no panic occurs.
func TestOrchestrator_NilGitCheckout_SkipsCleanup(t *testing.T) {
	t.Parallel()

	beadCount := 0
	cfg := OrchestratorConfig{
		Gate: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		Build: &fakeStage{runFn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			return pipeline.Output{}, errors.New("build exploded")
		}},
		Validate: &fakeStage{},
		Epilogue: &fakeStage{},
		// No BranchRouter or GitCheckout set.
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			beadCount++
			if beadCount == 1 {
				return &bead.Bead{ID: "nil-checkout", Title: "Nil Checkout"}, nil
			}
			return nil, nil
		},
		Config: &config.Config{},
		Output: io.Discard,
	}

	o := NewOrchestrator(cfg)
	// Should not panic even though GitCheckout is nil.
	_ = o.Run(context.Background(), 1, time.Time{}, make(chan struct{}))
}
