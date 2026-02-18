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
