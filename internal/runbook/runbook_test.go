package runbook

import (
	"fmt"
	"testing"
	"time"
)

func TestNewEntrySetsIDAndTimestamp(t *testing.T) {
	now := time.Date(2026, time.February, 18, 12, 0, 0, 0, time.FixedZone("test", -5*60*60))
	beadID := "beads-abc"

	entry := NewEntry(beadID, now)

	wantID := fmt.Sprintf("rb-%d-%s", now.Unix(), beadID)
	if entry.ID != wantID {
		t.Fatalf("expected id %q, got %q", wantID, entry.ID)
	}
	if !entry.Timestamp.Equal(now.UTC()) {
		t.Fatalf("expected timestamp %s, got %s", now.UTC(), entry.Timestamp)
	}
}
