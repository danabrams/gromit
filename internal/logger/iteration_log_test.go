package logger

import "testing"

// TestIterationLog_HasRateLimitRecoveryMsField verifies that IterationLog
// struct has a RateLimitRecoveryMs field for logging rate limit recovery time.
func TestIterationLog_HasRateLimitRecoveryMsField(t *testing.T) {
	log := &IterationLog{
		BeadID:              "test-1",
		Model:               "sonnet",
		RateLimitHits:       2,
		RateLimitRecoveryMs: 180,
	}

	if log.RateLimitRecoveryMs != 180 {
		t.Errorf("expected RateLimitRecoveryMs=180, got %d", log.RateLimitRecoveryMs)
	}
}
