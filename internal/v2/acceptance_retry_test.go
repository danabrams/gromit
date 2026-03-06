//go:build acceptance
// +build acceptance

package v2

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/loop"
	"github.com/danabrams/gromit/internal/v2/stage"
)

func TestRetryContextPopulatedOnFailure(t *testing.T) {
	t.Parallel()

	epilogue := newCapturingStage("epilogue")
	failStage := newFailingStage("validate")

	beadLoop, err := newRetryBeadLoop(failStage, epilogue)
	if err != nil {
		t.Fatalf("failed to build retry loop: %v", err)
	}

	err = beadLoop.Run(context.Background(), []*bead.Bead{{ID: "bead"}}, nil)
	if err == nil {
		t.Fatalf("expected loop to fail when stage errors")
	}

	if len(epilogue.requests) != 1 {
		t.Fatalf("epilogue should have been invoked once, got %d calls", len(epilogue.requests))
	}

	req := epilogue.requests[0]
	if req.RetryContext == nil {
		t.Fatalf("expected retry context to be populated")
	}
	if req.RetryContext.Attempt != 1 {
		t.Fatalf("retry attempt = %d, want 1", req.RetryContext.Attempt)
	}
	if len(req.RetryContext.PriorFailures) != 1 {
		t.Fatalf("expected one prior failure, got %d", len(req.RetryContext.PriorFailures))
	}
}

// Retry-with semantics and max retries exhaustion are covered by unit tests in
// internal/v2/loop/bead_loop_test.go:
//   - TestBeadLoopRetryWithRunsBuildBeforeRetryAndReportsAttempt
//   - TestBeadLoopValidateRetriesBuildOnFailure
//   - TestBeadLoopShortCircuitsToFailurePath

func newRetryBeadLoop(validate stage.Stage, epilogue stage.Stage) (*loop.BeadLoop, error) {
	return loop.NewBeadLoop(loop.BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    newNoopStage("build"),
		Validate: validate,
		Review:   newNoopStage("review"),
		Epilogue: epilogue,
	})
}

type failStage struct {
	name string
}

func newFailingStage(name string) *failStage {
	return &failStage{name: name}
}

func (f *failStage) Name() string { return f.name }

func (f *failStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	return &stage.Result{Decision: stage.DecisionFail}, nil
}

type capturingStage struct {
	name     string
	requests []*stage.Request
}

func newCapturingStage(name string) *capturingStage {
	return &capturingStage{name: name}
}

func (c *capturingStage) Name() string { return c.name }

func (c *capturingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	if req != nil {
		c.requests = append(c.requests, req)
	}
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

type noopStage struct {
	name string
}

func newNoopStage(name string) stage.Stage {
	return &noopStage{name: name}
}

func (n *noopStage) Name() string { return n.name }

func (n *noopStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}
