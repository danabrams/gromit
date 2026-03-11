package enrich

import (
	"testing"
	"time"
)

func TestStaleness_FreshFact(t *testing.T) {
	f := InferredFact{CreatedAt: time.Now()}
	if IsExpired(f, 30) {
		t.Error("fact created now should not be expired with 30-day window")
	}
}

func TestStaleness_ExpiredFact(t *testing.T) {
	f := InferredFact{CreatedAt: time.Now().Add(-45 * 24 * time.Hour)}
	if !IsExpired(f, 30) {
		t.Error("fact created 45 days ago should be expired with 30-day window")
	}
}

func TestStaleness_FilterExpired(t *testing.T) {
	facts := []InferredFact{
		{FactID: "fresh", CreatedAt: time.Now()},
		{FactID: "stale", CreatedAt: time.Now().Add(-60 * 24 * time.Hour)},
	}
	filtered := FilterExpired(facts, 30)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 non-expired fact, got %d", len(filtered))
	}
	if filtered[0].FactID != "fresh" {
		t.Errorf("expected fresh fact, got %q", filtered[0].FactID)
	}
}

func TestStaleness_ZeroExpiryDaysUsesDefault(t *testing.T) {
	// A fact created 15 days ago should NOT be expired when expiryDays=0,
	// because 0 defaults to 30 days.
	f := InferredFact{CreatedAt: time.Now().Add(-15 * 24 * time.Hour)}
	if IsExpired(f, 0) {
		t.Error("fact created 15 days ago should not be expired when expiryDays=0 (defaults to 30)")
	}
}

func TestStaleness_ObservedFactsFreshness(t *testing.T) {
	warning := CheckObservedFreshness("abc123", "def456")
	if warning == "" {
		t.Error("expected staleness warning for SHA mismatch")
	}
	warning = CheckObservedFreshness("abc123", "abc123")
	if warning != "" {
		t.Error("expected no warning for matching SHA")
	}
}
