package integrationqueue

import "sort"

// OldestReady returns the entry with the smallest fifo_seq among ready entries.
// Returns nil if no ready entries exist.
func OldestReady(queue *Queue) *Entry {
	if queue == nil {
		return nil
	}

	var oldest *Entry
	for i := range queue.Entries {
		entry := &queue.Entries[i]
		if entry.State != StateReady {
			continue
		}
		if oldest == nil || entry.FifoSeq < oldest.FifoSeq {
			oldest = entry
		}
	}
	return oldest
}

// QueuePosition computes the stable FIFO rank (1-based) for a ready entry.
// Returns 0 if the entry is not ready or not in the queue.
func QueuePosition(queue *Queue, entry *Entry) int {
	if queue == nil || entry == nil {
		return 0
	}

	// Collect all ready entries
	var readyEntries []*Entry
	for i := range queue.Entries {
		e := &queue.Entries[i]
		if e.State == StateReady {
			readyEntries = append(readyEntries, e)
		}
	}

	// Sort by FifoSeq
	sort.SliceStable(readyEntries, func(i, j int) bool {
		return readyEntries[i].FifoSeq < readyEntries[j].FifoSeq
	})

	// Find position of the given entry
	for idx, e := range readyEntries {
		if e == entry {
			return idx + 1
		}
	}

	return 0
}

// SortedForDisplay returns entries in deterministic order for status projection:
// 1. Integrating entries sorted by FifoSeq (ascending)
// 2. Ready entries sorted by FifoSeq (ascending)
// 3. Blocked entries sorted by UpdatedAt (descending), then FifoSeq (descending)
func SortedForDisplay(queue *Queue) []*Entry {
	if queue == nil {
		return nil
	}

	entries := make([]*Entry, 0, len(queue.Entries))
	for i := range queue.Entries {
		entries = append(entries, &queue.Entries[i])
	}
	return orderEntriesForDisplay(entries)
}
