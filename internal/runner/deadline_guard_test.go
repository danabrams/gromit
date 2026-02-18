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

// TestDeadlineGuard_InsufficientTimeRemaining verifies that when the context has
// a future deadline but less time remains than needed, the guard skips the phase
// and includes both remaining and needed durations in the result.
func TestDeadlineGuard_InsufficientTimeRemaining(t *testing.T) {
	needed := 60 * time.Second
	// Provide only 10 seconds, less than the 60 needed.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	guard := checkDeadlineGuard(ctx, needed)

	if !guard.Skip {
		t.Error("expected Skip=true when remaining time is less than needed")
	}
	if guard.SkipReason == "" {
		t.Error("expected non-empty SkipReason")
	}
	if guard.Remaining <= 0 {
		t.Errorf("expected Remaining > 0 for a future deadline, got %v", guard.Remaining)
	}
	if guard.Needed != needed {
		t.Errorf("Needed = %v, want %v", guard.Needed, needed)
	}
}

// TestDeadlineGuard_SufficientTimeRemaining verifies that when enough time remains
// before the deadline, the guard allows the phase to run (Skip=false) and reports
// both remaining and needed durations.
func TestDeadlineGuard_SufficientTimeRemaining(t *testing.T) {
	needed := 60 * time.Second
	// Provide 5 minutes, well above the 60 seconds needed.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Minute))
	defer cancel()

	guard := checkDeadlineGuard(ctx, needed)

	if guard.Skip {
		t.Errorf("expected Skip=false when sufficient time remains, got reason=%q", guard.SkipReason)
	}
	if guard.Remaining <= 0 {
		t.Errorf("expected Remaining > 0, got %v", guard.Remaining)
	}
	if guard.Needed != needed {
		t.Errorf("Needed = %v, want %v", guard.Needed, needed)
	}
}
