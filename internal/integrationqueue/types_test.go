package integrationqueue

import (
	"testing"
	"time"
)

func TestEntryOrderingMetadata(t *testing.T) {
	created := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	entry := Entry{
		Branch:        "gromit/ordering",
		SessionID:     "session",
		OriginCommand: "ready",
		State:         "ready",
		Lane:          "code_lane",
		BaseRef:       "main",
		HeadSHA:       "abc123",
		FifoSeq:       7,
		CreatedAt:     created,
		UpdatedAt:     updated,
	}

	meta := entry.Ordering()
	if meta.Sequence != entry.FifoSeq {
		t.Fatalf("sequence = %d, want %d", meta.Sequence, entry.FifoSeq)
	}
	if !meta.CreatedAt.Equal(entry.CreatedAt) {
		t.Fatalf("created_at = %v, want %v", meta.CreatedAt, entry.CreatedAt)
	}
	if !meta.UpdatedAt.Equal(entry.UpdatedAt) {
		t.Fatalf("updated_at = %v, want %v", meta.UpdatedAt, entry.UpdatedAt)
	}
}
