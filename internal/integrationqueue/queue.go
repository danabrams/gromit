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

	var integratingEntries []*Entry
	var readyEntries []*Entry
	var blockedEntries []*Entry

	blockedStateMap := map[State]struct{}{
		StateConflict:      {},
		StateFailedGates:   {},
		StateLaneViolation: {},
		StatePushFailure:   {},
	}

	for i := range queue.Entries {
		entry := &queue.Entries[i]
		switch entry.State {
		case StateIntegrating:
			integratingEntries = append(integratingEntries, entry)
		case StateReady:
			readyEntries = append(readyEntries, entry)
		default:
			if _, ok := blockedStateMap[entry.State]; ok {
				blockedEntries = append(blockedEntries, entry)
			}
		}
	}

	// Sort integrating by FifoSeq ascending
	sort.SliceStable(integratingEntries, func(i, j int) bool {
		return integratingEntries[i].FifoSeq < integratingEntries[j].FifoSeq
	})

	// Sort ready by FifoSeq ascending
	sort.SliceStable(readyEntries, func(i, j int) bool {
		return readyEntries[i].FifoSeq < readyEntries[j].FifoSeq
	})

	// Sort blocked by UpdatedAt descending, then FifoSeq descending
	sort.SliceStable(blockedEntries, func(i, j int) bool {
		if blockedEntries[i].UpdatedAt.Equal(blockedEntries[j].UpdatedAt) {
			return blockedEntries[i].FifoSeq > blockedEntries[j].FifoSeq
		}
		return blockedEntries[i].UpdatedAt.After(blockedEntries[j].UpdatedAt)
	})

	// Combine in order
	result := make([]*Entry, 0, len(integratingEntries)+len(readyEntries)+len(blockedEntries))
	result = append(result, integratingEntries...)
	result = append(result, readyEntries...)
	result = append(result, blockedEntries...)
	return result
}
