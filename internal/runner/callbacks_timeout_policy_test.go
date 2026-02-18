package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/runner/policy"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

type mockEscalationPolicy struct {
	SelectInitialTierFn  func(priority int, labels []string) string
	SelectModelFn        func(priority int, labels []string) string
	NextTierFn           func(currentTier string) string
	MaxRetriesPerModelFn func() int
	MaxRetriesPerBeadFn  func() int
	ClassifyTimeoutFn    func(ctxErr, parentErr error, stallFired bool) policy.TimeoutClassification
}

func (m *mockEscalationPolicy) SelectInitialTier(priority int, labels []string) string {
	if m.SelectInitialTierFn != nil {
		return m.SelectInitialTierFn(priority, labels)
	}
	return ""
}

func (m *mockEscalationPolicy) SelectModel(priority int, labels []string) string {
	if m.SelectModelFn != nil {
		return m.SelectModelFn(priority, labels)
	}
	return ""
}

func (m *mockEscalationPolicy) NextTier(currentTier string) string {
	if m.NextTierFn != nil {
		return m.NextTierFn(currentTier)
	}
	return ""
}

func (m *mockEscalationPolicy) MaxRetriesPerModel() int {
	if m.MaxRetriesPerModelFn != nil {
		return m.MaxRetriesPerModelFn()
	}
	return 0
}

func (m *mockEscalationPolicy) MaxRetriesPerBead() int {
	if m.MaxRetriesPerBeadFn != nil {
		return m.MaxRetriesPerBeadFn()
	}
	return 0
}

func (m *mockEscalationPolicy) ClassifyTimeout(ctxErr, parentErr error, stallFired bool) policy.TimeoutClassification {
	if m.ClassifyTimeoutFn != nil {
		return m.ClassifyTimeoutFn(ctxErr, parentErr, stallFired)
	}
	return policy.TimeoutClassification{}
}

func TestHandleInvokeError_UsesEscalationPolicyClassification(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	cancel()

	var gotCtxErr error
	var gotParentErr error
	var gotStall bool
	mockPolicy := &mockEscalationPolicy{
		ClassifyTimeoutFn: func(ctxErr, parentErr error, stallFired bool) policy.TimeoutClassification {
			gotCtxErr = ctxErr
			gotParentErr = parentErr
			gotStall = stallFired
			return policy.TimeoutClassification{TimeoutType: "bead"}
		},
	}

	r := &Runner{
		escalationPolicy: mockPolicy,
		renderer:         &mockRenderer{},
	}
	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-1", Title: "Timeout Bead"},
		Model:       "test-model",
		ParentCtx:   context.Background(),
		BeadTimeout: 30 * time.Second,
		Result:      &IterationResult{},
	}

	_, err := r.handleInvokeError(ctx, bc, nil, context.DeadlineExceeded)
	if err == nil || !strings.Contains(err.Error(), "bead timeout") {
		t.Fatalf("expected bead timeout error, got %v", err)
	}
	if gotCtxErr == nil || !errors.Is(gotCtxErr, context.DeadlineExceeded) {
		t.Fatal("expected ClassifyTimeout to receive ctx error")
	}
	if gotParentErr != nil {
		t.Fatalf("expected parent error to be nil, got %v", gotParentErr)
	}
	if gotStall {
		t.Fatal("expected stall flag to be false")
	}
	if bc.Result.TimeoutType != "bead" {
		t.Fatalf("TimeoutType = %q, want %q", bc.Result.TimeoutType, "bead")
	}
}
