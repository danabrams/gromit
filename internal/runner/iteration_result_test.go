package runner

import "testing"

// TestIterationResult_HasRateLimitRecoveryMsField verifies that IterationResult
// struct has a RateLimitRecoveryMs field for tracking rate limit recovery time.
func TestIterationResult_HasRateLimitRecoveryMsField(t *testing.T) {
	result := &IterationResult{
		BeadID:              "test-1",
		Model:               "sonnet",
		RateLimitRecoveryMs: 250,
	}

	if result.RateLimitRecoveryMs != 250 {
		t.Errorf("expected RateLimitRecoveryMs=250, got %d", result.RateLimitRecoveryMs)
	}
}
