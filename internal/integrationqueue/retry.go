package integrationqueue

// gateRetryTracker enforces lane-specific retry policies for scoped gate execution.
type gateRetryTracker struct {
	maxRetries  int
	usedRetries int
}

// newGateRetryTracker returns a tracker configured with the lane's retry budget.
func newGateRetryTracker(l Lane) gateRetryTracker {
	max := 0
	if l == CodeLane {
		max = 1
	}
	return gateRetryTracker{maxRetries: max}
}

// CanRetry reports whether another retry is allowed.
func (t gateRetryTracker) CanRetry() bool {
	return t.usedRetries < t.maxRetries
}

// RecordRetry registers a retry attempt and updates the entry's retry counter.
func (t *gateRetryTracker) RecordRetry(entry *Entry) {
	if !t.CanRetry() {
		return
	}
	t.usedRetries++
	entry.RetryCount = t.usedRetries
}
