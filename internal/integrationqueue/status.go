package integrationqueue

import "sort"

// Status captures summary data for an integration queue snapshot.
type Status struct {
	QueueLength      int
	ReadyCount       int
	IntegratingCount int
	BlockedCount     int
	MergedCount      int
	Entries          []StatusEntry
}

// StatusEntry represents an ordered entry in the projected queue view.
type StatusEntry struct {
	Entry         *Entry
	ReadyPosition int
}

var blockedStates = map[State]struct{}{
	StateConflict:      {},
	StateFailedGates:   {},
	StateLaneViolation: {},
}

// ProjectStatus computes summary counts and ordered entries from a snapshot.
func ProjectStatus(snapshot *Snapshot) *Status {
	if snapshot == nil {
		return nil
	}

	status := &Status{
		QueueLength: len(snapshot.Entries),
	}

	var integratingEntries []*Entry
	var readyEntries []*Entry
	var blockedEntries []*Entry

	for i := range snapshot.Entries {
		entry := &snapshot.Entries[i]
		switch entry.State {
		case StateReady:
			readyEntries = append(readyEntries, entry)
			status.ReadyCount++
		case StateIntegrating:
			integratingEntries = append(integratingEntries, entry)
			status.IntegratingCount++
		case StateMerged:
			status.MergedCount++
		default:
			if _, ok := blockedStates[entry.State]; ok {
				blockedEntries = append(blockedEntries, entry)
				status.BlockedCount++
			}
		}
	}

	sort.SliceStable(integratingEntries, func(i, j int) bool {
		return integratingEntries[i].FifoSeq < integratingEntries[j].FifoSeq
	})

	sort.SliceStable(readyEntries, func(i, j int) bool {
		return readyEntries[i].FifoSeq < readyEntries[j].FifoSeq
	})

	sort.SliceStable(blockedEntries, func(i, j int) bool {
		if blockedEntries[i].UpdatedAt.Equal(blockedEntries[j].UpdatedAt) {
			return blockedEntries[i].FifoSeq > blockedEntries[j].FifoSeq
		}
		return blockedEntries[i].UpdatedAt.After(blockedEntries[j].UpdatedAt)
	})

	entries := make([]StatusEntry, 0, len(integratingEntries)+len(readyEntries)+len(blockedEntries))
	for _, entry := range integratingEntries {
		entries = append(entries, StatusEntry{Entry: entry})
	}
	for idx, entry := range readyEntries {
		entries = append(entries, StatusEntry{Entry: entry, ReadyPosition: idx + 1})
	}
	for _, entry := range blockedEntries {
		entries = append(entries, StatusEntry{Entry: entry})
	}
	status.Entries = entries
	return status
}
