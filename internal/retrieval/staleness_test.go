package retrieval

import (
	"testing"
	"time"
)

func TestStalenessDetectorMaxAge(t *testing.T) {
	now := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
	detector := NewStalenessDetector(func() time.Time { return now })

	metadata := IndexMetadata{LastUpdated: now.Add(-48 * time.Hour)}
	stale, reason := detector.Check(metadata, StalenessPolicyMaxAge, 1)
	if !stale {
		t.Fatalf("expected stale, got clean")
	}
	if reason == "" {
		t.Fatalf("expected reason for staleness")
	}

	metadata.LastUpdated = now
	stale, _ = detector.Check(metadata, StalenessPolicyMaxAge, 1)
	if stale {
		t.Fatalf("expected fresh state")
	}
}
