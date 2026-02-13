package logger

import (
	"testing"
	"time"
)

// TestRateLimitRecoveryMs_MeasuresTimeBetweenRateLimitHitAndNextEvent verifies that
// RateLimitRecoveryMs captures the duration from when RecordRateLimitHit() is called
// until the next RecordEvent() call.
func TestRateLimitRecoveryMs_MeasuresTimeBetweenRateLimitHitAndNextEvent(t *testing.T) {
	stats, _ := NewStreamStats()

	// Record a rate limit hit
	stats.RecordRateLimitHit()

	// Wait a known duration before the next event
	time.Sleep(100 * time.Millisecond)

	// Record the next event (this should compute recovery time)
	stats.RecordEvent()

	// Verify recovery time was captured via DiagnosticSnapshot
	_, _, _, _, _, recoveryMs := stats.DiagnosticSnapshot()
	if recoveryMs == 0 {
		t.Error("expected recovery time to be set after rate limit hit followed by event")
	}

	if recoveryMs < 100 {
		t.Errorf("expected recovery time >= 100ms, got %d ms", recoveryMs)
	}
}

// TestRateLimitRecoveryMs_ZeroWhenNoRateLimitOccurs verifies that RateLimitRecoveryMs
// remains zero when events occur without any rate limit hits.
func TestRateLimitRecoveryMs_ZeroWhenNoRateLimitOccurs(t *testing.T) {
	stats, _ := NewStreamStats()

	// Record events without any rate limit hits
	stats.RecordEvent()
	time.Sleep(50 * time.Millisecond)
	stats.RecordEvent()

	// Verify recovery time is not set (still zero)
	_, _, _, _, _, recoveryMs := stats.DiagnosticSnapshot()
	if recoveryMs != 0 {
		t.Errorf("expected recovery time to be 0 when no rate limit hit, got %d ms", recoveryMs)
	}
}

// TestDiagnosticSnapshot_ReturnsRecoveryTime verifies that DiagnosticSnapshot()
// returns the rate limit recovery time as part of its diagnostic data.
func TestDiagnosticSnapshot_ReturnsRecoveryTime(t *testing.T) {
	stats, _ := NewStreamStats()

	// Record rate limit hit followed by event
	stats.RecordRateLimitHit()
	time.Sleep(75 * time.Millisecond)
	stats.RecordEvent()

	// Call DiagnosticSnapshot and verify it returns recovery time
	stallCount, stallTier, _, toolCalls, rateLimitHits, recoveryMs := stats.DiagnosticSnapshot()

	// Verify all expected values
	if stallCount != 0 {
		t.Errorf("expected stallCount=0, got %d", stallCount)
	}
	if stallTier != "" {
		t.Errorf("expected empty stallTier, got %q", stallTier)
	}
	if toolCalls != 0 {
		t.Errorf("expected toolCalls=0, got %d", toolCalls)
	}
	if rateLimitHits != 1 {
		t.Errorf("expected rateLimitHits=1, got %d", rateLimitHits)
	}

	// Verify the new recovery time return value
	if recoveryMs == 0 {
		t.Error("expected recovery time to be set after rate limit recovery")
	}
	if recoveryMs < 75 {
		t.Errorf("expected recovery time >= 75ms, got %d ms", recoveryMs)
	}
}

// TestDiagnosticSnapshot_RecoveryTimeZeroWhenNoRateLimit verifies that
// DiagnosticSnapshot() returns 0 for recovery time when no rate limit occurred.
func TestDiagnosticSnapshot_RecoveryTimeZeroWhenNoRateLimit(t *testing.T) {
	stats, _ := NewStreamStats()

	// Record activity without rate limits
	stats.RecordEvent()
	stats.RecordToolCall("Read", "/foo.go")

	_, _, _, _, rateLimitHits, recoveryMs := stats.DiagnosticSnapshot()

	if recoveryMs != 0 {
		t.Errorf("expected recovery time=0 when no rate limit, got %d ms", recoveryMs)
	}
	if rateLimitHits != 0 {
		t.Errorf("expected rateLimitHits=0, got %d", rateLimitHits)
	}
}

// TestMultipleRateLimitHits_RecordMostRecentRecoveryTime verifies that when
// multiple rate limit hits occur, only the most recent recovery time is retained.
func TestMultipleRateLimitHits_RecordMostRecentRecoveryTime(t *testing.T) {
	stats, _ := NewStreamStats()

	// First rate limit hit and recovery
	stats.RecordRateLimitHit()
	time.Sleep(50 * time.Millisecond)
	stats.RecordEvent()

	_, _, _, _, _, firstRecovery := stats.DiagnosticSnapshot()
	if firstRecovery < 50 {
		t.Errorf("expected first recovery >= 50ms, got %d ms", firstRecovery)
	}

	// Second rate limit hit with different recovery time
	stats.RecordRateLimitHit()
	time.Sleep(150 * time.Millisecond)
	stats.RecordEvent()

	_, _, _, _, _, secondRecovery := stats.DiagnosticSnapshot()
	if secondRecovery < 150 {
		t.Errorf("expected second recovery >= 150ms, got %d ms", secondRecovery)
	}

	// Verify the most recent recovery time is retained, not the first one
	if secondRecovery == firstRecovery {
		t.Error("expected recovery time to be updated to most recent, still shows first recovery")
	}

	// Verify it's the second recovery time
	if secondRecovery < firstRecovery {
		t.Errorf("expected second recovery (%d ms) > first recovery (%d ms)", secondRecovery, firstRecovery)
	}
}

// TestMultipleRateLimitHits_WithoutInterveningEvent verifies that multiple
// rate limit hits without intervening events only measure recovery from the
// most recent hit.
func TestMultipleRateLimitHits_WithoutInterveningEvent(t *testing.T) {
	stats, _ := NewStreamStats()

	// Multiple rate limit hits in quick succession
	// Use wider intervals so the gap between "from first hit" (~200ms)
	// and "from last hit" (~50ms) is large enough to tolerate scheduler jitter
	// when the full test suite runs under load.
	stats.RecordRateLimitHit()
	time.Sleep(75 * time.Millisecond)
	stats.RecordRateLimitHit()
	time.Sleep(75 * time.Millisecond)
	stats.RecordRateLimitHit()

	// Now record an event after 50ms from the last hit
	time.Sleep(50 * time.Millisecond)
	stats.RecordEvent()

	// Recovery should be measured from the last (third) rate limit hit
	// which was ~50ms before the event, not from the first hit (~200ms).
	// Allow generous upper bound (150ms) to handle scheduler delays,
	// while still distinguishing from the first-hit measurement (~200ms+).
	_, _, _, _, _, recoveryMs := stats.DiagnosticSnapshot()
	if recoveryMs > 150 {
		t.Errorf("expected recovery time ~50ms (from last hit), got %d ms", recoveryMs)
	}
}

// TestRateLimitRecoveryMs_NilSafe verifies nil safety for DiagnosticSnapshot.
func TestRateLimitRecoveryMs_NilSafe(t *testing.T) {
	var stats *StreamStats

	// Should not panic
	_, _, _, _, _, recoveryMs := stats.DiagnosticSnapshot()
	if recoveryMs != 0 {
		t.Errorf("expected 0 for nil stats, got %d", recoveryMs)
	}
}

// TestRecordEvent_OnlyComputesRecoveryAfterRateLimit verifies that RecordEvent()
// only computes and stores recovery time when it follows a rate limit hit.
func TestRecordEvent_OnlyComputesRecoveryAfterRateLimit(t *testing.T) {
	stats, _ := NewStreamStats()

	// Record events without rate limit - should not compute recovery
	stats.RecordEvent()
	time.Sleep(30 * time.Millisecond)
	stats.RecordEvent()

	_, _, _, _, _, recoveryMs := stats.DiagnosticSnapshot()
	if recoveryMs != 0 {
		t.Errorf("expected no recovery time without rate limit, got %d ms", recoveryMs)
	}

	// Now record a rate limit and subsequent event
	stats.RecordRateLimitHit()
	time.Sleep(60 * time.Millisecond)
	stats.RecordEvent()

	_, _, _, _, _, recoveryMs = stats.DiagnosticSnapshot()
	if recoveryMs == 0 {
		t.Error("expected recovery time after rate limit + event")
	}
	if recoveryMs < 60 {
		t.Errorf("expected recovery >= 60ms, got %d ms", recoveryMs)
	}

	// Get the recovery time after first rate limit
	previousRecovery := recoveryMs

	// Subsequent events without new rate limits should not change recovery time
	time.Sleep(40 * time.Millisecond)
	stats.RecordEvent()

	_, _, _, _, _, recoveryMs = stats.DiagnosticSnapshot()
	if recoveryMs != previousRecovery {
		t.Errorf("expected recovery time unchanged (%d ms) when no new rate limit, got %d ms",
			previousRecovery, recoveryMs)
	}
}

// TestParseAndLogEvent_ComputesRecoveryTimeOnEvent verifies that
// ParseAndLogEvent() properly triggers recovery time computation when an
// event follows a rate limit error.
func TestParseAndLogEvent_ComputesRecoveryTimeOnEvent(t *testing.T) {
	stats, _ := NewStreamStats()

	// Parse a rate limit error event
	rateLimitLine := []byte(`{"type":"error","subtype":"overloaded"}`)
	ParseAndLogEvent(nil, stats, rateLimitLine)

	_, _, _, _, rateLimitHits, _ := stats.DiagnosticSnapshot()
	if rateLimitHits != 1 {
		t.Errorf("expected rateLimitHits=1, got %d", rateLimitHits)
	}

	// Wait and parse a subsequent normal event
	time.Sleep(90 * time.Millisecond)
	normalLine := []byte(`{"type":"system","subtype":"init"}`)
	ParseAndLogEvent(nil, stats, normalLine)

	// Verify recovery time was computed
	_, _, _, _, rateLimitHits, recoveryMs := stats.DiagnosticSnapshot()
	if recoveryMs == 0 {
		t.Error("expected recovery time to be computed after rate limit followed by event")
	}
	if recoveryMs < 90 {
		t.Errorf("expected recovery time >= 90ms, got %d ms", recoveryMs)
	}
	if rateLimitHits != 1 {
		t.Errorf("expected rateLimitHits=1 from DiagnosticSnapshot, got %d", rateLimitHits)
	}
}
