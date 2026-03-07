package epilogue

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestEpilogueStageSuccessClosesBeadAndEmitsCostEvent(t *testing.T) {
	t.Parallel()

	tracker := &fakeTracker{}
	stageInstance, err := New(&config.Config{}, tracker)
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	telemetry := &stagepkg.LLMCostSummary{
		Model:        "haiku",
		Duration:     2 * time.Second,
		InputTokens:  10,
		OutputTokens: 5,
		CostUSD:      0.42,
	}

	req := &stagepkg.Request{
		Bead:      stagepkg.BeadInfo{ID: "bead-123"},
		Model:     "haiku",
		Telemetry: telemetry,
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(tracker.closeCalls) != 1 || tracker.closeCalls[0] != "bead-123" {
		t.Fatalf("expected CloseBead on bead-123, got %v", tracker.closeCalls)
	}

	if res == nil {
		t.Fatal("expected result")
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want proceed", res.Decision)
	}
	if len(res.Events) != 0 {
		t.Fatalf("events count = %d, want 0", len(res.Events))
	}
}

func TestEpilogueStageFailureEmitsFailureEventWithoutClosing(t *testing.T) {
	t.Parallel()

	tracker := &fakeTracker{}
	stageInstance, err := New(&config.Config{}, tracker)
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	priorFailures := []string{"build failed", "validate timed out"}
	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "failure-bead"},
		RetryContext: &stagepkg.RetryContext{
			Attempt:       1,
			PriorFailures: append([]string(nil), priorFailures...),
		},
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(tracker.closeCalls) != 0 {
		t.Fatalf("close called unexpectedly: %v", tracker.closeCalls)
	}

	if len(res.Events) != 0 {
		t.Fatalf("events count = %d, want 0", len(res.Events))
	}
}

func TestIsFailurePath_NilRequest(t *testing.T) {
	t.Parallel()
	if isFailurePath(nil) {
		t.Fatal("nil request should not be a failure path")
	}
}

func TestIsFailurePath_NilRetryContext(t *testing.T) {
	t.Parallel()
	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "bead-1"},
	}
	if isFailurePath(req) {
		t.Fatal("nil RetryContext should not be a failure path")
	}
}

func TestIsFailurePath_EmptyPriorFailures(t *testing.T) {
	t.Parallel()
	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "bead-1"},
		RetryContext: &stagepkg.RetryContext{
			Attempt:       1,
			PriorFailures: []string{},
		},
	}
	if isFailurePath(req) {
		t.Fatal("RetryContext with empty PriorFailures should not be a failure path (successful retry)")
	}
}

func TestIsFailurePath_WithPriorFailures(t *testing.T) {
	t.Parallel()
	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "bead-1"},
		RetryContext: &stagepkg.RetryContext{
			Attempt:       2,
			PriorFailures: []string{"build failed"},
		},
	}
	if !isFailurePath(req) {
		t.Fatal("RetryContext with PriorFailures should be a failure path")
	}
}

type fakeTracker struct {
	closeCalls []string
}

func (f *fakeTracker) NextBead(ctx context.Context, req tasktracker.NextBeadRequest) (*tasktracker.NextBeadResponse, error) {
	return nil, nil
}

func (f *fakeTracker) ShowBead(ctx context.Context, beadID string) (*tasktracker.Bead, error) {
	return nil, nil
}

func (f *fakeTracker) CreateBead(ctx context.Context, req tasktracker.CreateBeadRequest) (*tasktracker.CreateBeadResponse, error) {
	return nil, nil
}

func (f *fakeTracker) CloseBead(ctx context.Context, req tasktracker.CloseBeadRequest) (*tasktracker.CloseBeadResponse, error) {
	f.closeCalls = append(f.closeCalls, req.BeadID)
	return &tasktracker.CloseBeadResponse{Closed: true}, nil
}

func (f *fakeTracker) QueryBeads(ctx context.Context, req tasktracker.QueryBeadsRequest) (*tasktracker.QueryBeadsResponse, error) {
	return &tasktracker.QueryBeadsResponse{}, nil
}
