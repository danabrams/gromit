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

func TestQueuePosition_ComputesStableFIFORankForReadyEntry(t *testing.T) {
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
				Branch:    "branch-4",
				State:     StateDraft,
				FifoSeq:   4,
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

	// Test position for entry with FifoSeq=1 (should be 1st ready)
	entry1 := &queue.Entries[1]
	pos1 := QueuePosition(queue, entry1)
	if pos1 != 1 {
		t.Errorf("expected position=1 for FifoSeq=1, got %d", pos1)
	}

	// Test position for entry with FifoSeq=2 (should be 2nd ready)
	entry2 := &queue.Entries[3]
	pos2 := QueuePosition(queue, entry2)
	if pos2 != 2 {
		t.Errorf("expected position=2 for FifoSeq=2, got %d", pos2)
	}

	// Test position for entry with FifoSeq=3 (should be 3rd ready)
	entry3 := &queue.Entries[0]
	pos3 := QueuePosition(queue, entry3)
	if pos3 != 3 {
		t.Errorf("expected position=3 for FifoSeq=3, got %d", pos3)
	}
}
