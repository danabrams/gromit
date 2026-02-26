//go:build !windows

package claude_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/policy"
)

// RED: test for bead timeout classification
func TestClaudeClient_BeadTimeoutClassification(t *testing.T) {
	// A bead timeout occurs when the invocation context deadline expires
	// but the parent context is not canceled.
	//
	// Scenario:
	// 1. Parent context remains valid (no deadline, no cancellation)
	// 2. Invocation context has a short deadline that expires
	// 3. The error from the invocation is context.DeadlineExceeded
	// 4. Parent context error is nil
	//
	// This simulates a bead-level timeout where the bead's time budget
	// is exhausted before the runner's overall time budget.

	// Create a context that's already expired to simulate bead timeout
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	// Simulate the error that would occur from the expired context
	ctxErr := ctx.Err()
	if ctxErr == nil {
		t.Fatal("expected context error from expired deadline, got nil")
	}

	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", ctxErr)
	}

	// Classify using the policy
	cfg := &config.Config{}
	classification := policy.NewConfigEscalationPolicy(cfg).ClassifyTimeout(ctxErr, nil, false)

	// Assert the timeout is classified as "bead"
	if classification.TimeoutType != "bead" {
		t.Fatalf("ClassifyTimeout() TimeoutType = %q, want %q", classification.TimeoutType, "bead")
	}
	if classification.ParentCanceled {
		t.Fatal("ClassifyTimeout() ParentCanceled = true, want false")
	}
}
