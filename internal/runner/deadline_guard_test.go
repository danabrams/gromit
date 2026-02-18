package runner

import (
	"context"
	"testing"
	"time"
)

// TestDeadlineGuard_NoDeadline verifies that when the context has no deadline,
// the guard allows the phase to run (no forced skip).
func TestDeadlineGuard_NoDeadline(t *testing.T) {
	ctx := context.Background() // no deadline
	guard := checkDeadlineGuard(ctx, 60*time.Second)

	if guard.Skip {
		t.Errorf("expected Skip=false when context has no deadline, got reason=%q", guard.SkipReason)
	}
}

// TestDeadlineGuard_ExpiredDeadline verifies that when the context deadline has
// already passed, the guard skips the phase with a reason indicating expiry.
func TestDeadlineGuard_ExpiredDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	guard := checkDeadlineGuard(ctx, 60*time.Second)

	if !guard.Skip {
		t.Error("expected Skip=true when deadline has expired")
	}
	if guard.SkipReason == "" {
		t.Error("expected non-empty SkipReason when deadline has expired")
	}
	if guard.Needed != 60*time.Second {
		t.Errorf("Needed = %v, want %v", guard.Needed, 60*time.Second)
	}
}
