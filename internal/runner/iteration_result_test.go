package runner

import (
	"context"
	"testing"
)

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

// TestIterationResult_HasValidationDurationMsField verifies that IterationResult
// struct has a ValidationDurationMs field for tracking validation duration.
func TestIterationResult_HasValidationDurationMsField(t *testing.T) {
	result := &IterationResult{
		BeadID:               "test-1",
		Model:                "sonnet",
		ValidationDurationMs: 420,
	}

	if result.ValidationDurationMs != 420 {
		t.Errorf("expected ValidationDurationMs=420, got %d", result.ValidationDurationMs)
	}
}

func TestIterationResult_HasProviderAndFailureCategoryFields(t *testing.T) {
	result := &IterationResult{
		BeadID:          "test-1",
		Model:           "sonnet",
		Provider:        "codex",
		FailureCategory: "rate_limited",
	}

	if result.Provider != "codex" {
		t.Errorf("expected Provider=%q, got %q", "codex", result.Provider)
	}
	if result.FailureCategory != "rate_limited" {
		t.Errorf("expected FailureCategory=%q, got %q", "rate_limited", result.FailureCategory)
	}
}

func TestIterationResult_HasFallbackAndValidationTimeoutFields(t *testing.T) {
	result := &IterationResult{}

	recordFallbackAttempt(result)
	recordFallbackOutcome(result, true)
	recordFallbackAttempt(result)
	recordFallbackOutcome(result, false)
	recordValidationTimeout(result, context.DeadlineExceeded)
	recordValidationTimeout(result, context.Canceled)
	recordValidationTimeout(result, nil)

	if result.FallbackAttempts != 2 {
		t.Errorf("expected FallbackAttempts=2, got %d", result.FallbackAttempts)
	}
	if result.FallbackSuccesses != 1 {
		t.Errorf("expected FallbackSuccesses=1, got %d", result.FallbackSuccesses)
	}
	if result.FallbackFailures != 1 {
		t.Errorf("expected FallbackFailures=1, got %d", result.FallbackFailures)
	}
	if result.ValidationTimeouts != 2 {
		t.Errorf("expected ValidationTimeouts=2, got %d", result.ValidationTimeouts)
	}
}
