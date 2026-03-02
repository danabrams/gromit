package integrationqueue

import "sort"

var blockedDisplayStates = map[State]struct{}{
	StateConflict:      {},
	StateFailedGates:   {},
	StateLaneViolation: {},
	StatePushFailure:   {},
}

func orderEntriesForDisplay(entries []*Entry) []*Entry {
	var integratingEntries []*Entry
	var readyEntries []*Entry
	var blockedEntries []*Entry

	for _, entry := range entries {
		switch entry.State {
		case StateIntegrating:
			integratingEntries = append(integratingEntries, entry)
		case StateReady:
			readyEntries = append(readyEntries, entry)
		default:
			if _, ok := blockedDisplayStates[entry.State]; ok {
				blockedEntries = append(blockedEntries, entry)
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

	ordered := make([]*Entry, 0, len(integratingEntries)+len(readyEntries)+len(blockedEntries))
	ordered = append(ordered, integratingEntries...)
	ordered = append(ordered, readyEntries...)
	ordered = append(ordered, blockedEntries...)
	return ordered
}
