package integrationqueue

import (
	"testing"
	"time"
)

func TestOldestReady_ReturnsEntryWithSmallestFifoSeq(t *testing.T) {
	now := time.Now().UTC()
	queue := &Queue{
		Entries: []Entry{
			{
				Branch:    "branch-3",
				State:     StateReady,
				FifoSeq:   3,
				CreatedAt: now,
			},
			{
				Branch:    "branch-1",
				State:     StateReady,
				FifoSeq:   1,
				CreatedAt: now,
			},
			{
				Branch:    "branch-2",
				State:     StateReady,
				FifoSeq:   2,
				CreatedAt: now,
			},
		},
	}

	entry := OldestReady(queue)
	if entry == nil {
		t.Fatal("OldestReady returned nil")
	}
	if entry.FifoSeq != 1 {
		t.Errorf("expected FifoSeq=1, got %d", entry.FifoSeq)
	}
	if entry.Branch != "branch-1" {
		t.Errorf("expected branch=branch-1, got %s", entry.Branch)
	}
}
