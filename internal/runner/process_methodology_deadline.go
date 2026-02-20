package runner

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// deadlineGuard holds the result of a deadline check for an optional phase.
type deadlineGuard struct {
	Skip       bool
	Remaining  time.Duration
	Needed     time.Duration
	SkipReason string
}

const (
	skipReasonDeadlineExpired           = "deadline_expired"
	skipReasonInsufficientTimeRemaining = "insufficient_time_remaining"
)

// checkDeadlineGuard inspects ctx.Deadline() and determines whether enough time
// remains to run a phase that requires the given needed duration. If the context
// has no deadline, the phase is allowed to run (Skip=false). If the deadline has
// passed or insufficient time remains, Skip=true with a reason set.
func checkDeadlineGuard(ctx context.Context, needed time.Duration) deadlineGuard {
	deadline, ok := ctx.Deadline()
	if !ok {
		return deadlineGuard{Skip: false, Needed: needed}
	}
	return checkRemainingGuard(time.Until(deadline), needed)
}

func checkRemainingGuard(remaining time.Duration, needed time.Duration) deadlineGuard {
	if remaining <= 0 {
		return deadlineGuard{
			Skip:       true,
			Remaining:  remaining,
			Needed:     needed,
			SkipReason: skipReasonDeadlineExpired,
		}
	}
	if remaining < needed {
		return deadlineGuard{
			Skip:       true,
			Remaining:  remaining,
			Needed:     needed,
			SkipReason: skipReasonInsufficientTimeRemaining,
		}
	}
	return deadlineGuard{Skip: false, Remaining: remaining, Needed: needed}
}

func beadRemaining(bc *runtypes.BeadContext) (remaining time.Duration, elapsed time.Duration, ok bool) {
	if bc == nil || bc.BeadTimeout <= 0 || bc.BeadStartTime.IsZero() {
		return 0, 0, false
	}
	elapsed = time.Since(bc.BeadStartTime)
	return bc.BeadTimeout - elapsed, elapsed, true
}
