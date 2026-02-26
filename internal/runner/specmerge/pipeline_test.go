package specmerge_test

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/runner/specmerge"
	"github.com/danabrams/gromit/internal/specgate"
)

type fakeBeadCreator struct {
	createFn func(ctx context.Context, title, description, priority string, labels []string) (string, error)
}

var _ specgate.BeadCreator = (*fakeBeadCreator)(nil)

func (f *fakeBeadCreator) Create(ctx context.Context, title, description, priority string, labels []string) (string, error) {
	if f.createFn == nil {
		return "", nil
	}
	return f.createFn(ctx, title, description, priority, labels)
}

func TestHandleStageFailure_CreateFixBeads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	failures := []specgate.CriterionResult{
		{Criterion: "Test quality", Passed: false, Evidence: "missing tests"},
	}

	var createdBeadIDs []string
	creator := &fakeBeadCreator{
		createFn: func(_ context.Context, title, description, priority string, labels []string) (string, error) {
			createdBeadIDs = append(createdBeadIDs, "bead-1")
			return "bead-1", nil
		},
	}

	deps := specmerge.FixBeadDependencies{
		BeadCreator: creator,
	}

	opts := specmerge.HandleStageFailureOptions{
		SpecName:      "test-spec",
		Failures:      failures,
		Priority:      "P1",
		AttemptCount:  0,
		RetryCap:      3,
	}

	err := specmerge.HandleStageFailure(ctx, deps, opts)
	if err != nil {
		t.Fatalf("HandleStageFailure returned error: %v", err)
	}

	if len(createdBeadIDs) != 1 {
		t.Fatalf("HandleStageFailure created %d beads, want 1", len(createdBeadIDs))
	}
}

func TestCheckRetryCapExceeded_AtCapOrBeyond(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		attemptCount int
		retryCap     int
		want         bool
	}{
		{
			name:         "attempt equals cap",
			attemptCount: 3,
			retryCap:     3,
			want:         true,
		},
		{
			name:         "attempt exceeds cap",
			attemptCount: 4,
			retryCap:     3,
			want:         true,
		},
		{
			name:         "attempt below cap",
			attemptCount: 2,
			retryCap:     3,
			want:         false,
		},
		{
			name:         "zero attempts at zero cap",
			attemptCount: 0,
			retryCap:     0,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := specmerge.CheckRetryCapExceeded(tt.attemptCount, tt.retryCap)
			if err != nil {
				t.Fatalf("CheckRetryCapExceeded returned error: %v", err)
			}
			if result != tt.want {
				t.Fatalf("CheckRetryCapExceeded(%d, %d) = %v, want %v", tt.attemptCount, tt.retryCap, result, tt.want)
			}
		})
	}
}

func TestEmitRetryCapReachedAlert_ReturnsAlert(t *testing.T) {
	t.Parallel()

	specName := "test-spec"
	retryCap := 3

	alert := specmerge.EmitRetryCapReachedAlert(specName, retryCap)

	if alert == "" {
		t.Fatalf("EmitRetryCapReachedAlert returned empty string, want non-empty alert message")
	}

	if !contains(alert, specName) {
		t.Fatalf("alert = %q, want to contain spec name %q", alert, specName)
	}

	if !contains(alert, "3") {
		t.Fatalf("alert = %q, want to contain retry cap 3", alert)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
