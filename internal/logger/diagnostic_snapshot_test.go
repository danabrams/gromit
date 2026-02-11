package logger

import (
	"testing"
	"time"
)

// TestDiagnosticSnapshot_ReturnsRateLimitRecoveryMs verifies that
// DiagnosticSnapshot returns rate limit recovery time as the 6th return value.
func TestDiagnosticSnapshot_ReturnsRateLimitRecoveryMs(t *testing.T) {
	stats, _ := NewStreamStats()

	// Record rate limit hit followed by event after delay
	stats.RecordRateLimitHit()
	time.Sleep(50 * time.Millisecond)
	stats.RecordEvent()

	stallCount, stallTier, ttfe, toolCalls, rateLimitHits, recoveryMs := stats.DiagnosticSnapshot()

	if stallCount != 0 {
		t.Errorf("expected stallCount=0, got %d", stallCount)
	}
	if stallTier != "" {
		t.Errorf("expected stallTier='', got %q", stallTier)
	}
	if ttfe == 0 {
		t.Error("expected non-zero time to first event")
	}
	if toolCalls != 0 {
		t.Errorf("expected toolCalls=0, got %d", toolCalls)
	}
	if rateLimitHits != 1 {
		t.Errorf("expected rateLimitHits=1, got %d", rateLimitHits)
	}

	// Verify recovery time is non-zero and reasonable
	if recoveryMs == 0 {
		t.Error("expected non-zero recovery time after rate limit")
	}
	if recoveryMs < 40 {
		t.Errorf("expected recoveryMs >= 40ms (50ms sleep), got %d ms", recoveryMs)
	}
}

// TestDiagnosticSnapshot_ZeroRecoveryWhenNoRateLimit verifies that
// recovery time is zero when no rate limit events occur.
func TestDiagnosticSnapshot_ZeroRecoveryWhenNoRateLimit(t *testing.T) {
	stats, _ := NewStreamStats()

	// Record activity without rate limits
	stats.RecordEvent()
	stats.RecordToolCall("Read", "/foo.go")

	_, _, _, _, _, recoveryMs := stats.DiagnosticSnapshot()

	if recoveryMs != 0 {
		t.Errorf("expected recoveryMs=0 when no rate limit, got %d ms", recoveryMs)
	}
}
