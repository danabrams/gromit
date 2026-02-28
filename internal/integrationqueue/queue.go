package integrationqueue

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
