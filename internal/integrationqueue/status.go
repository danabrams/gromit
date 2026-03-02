package integrationqueue

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

const displayLimit = 10

// ProjectStatus computes summary counts and ordered entries from a snapshot.
func ProjectStatus(snapshot *Snapshot) *Status {
	if snapshot == nil {
		return nil
	}

	status := &Status{
		QueueLength: len(snapshot.Entries),
	}

	allEntries := make([]*Entry, 0, len(snapshot.Entries))
	for i := range snapshot.Entries {
		entry := &snapshot.Entries[i]
		allEntries = append(allEntries, entry)

		switch entry.State {
		case StateReady:
			status.ReadyCount++
		case StateIntegrating:
			status.IntegratingCount++
		case StateMerged:
			status.MergedCount++
		default:
			if _, ok := blockedDisplayStates[entry.State]; ok {
				status.BlockedCount++
			}
		}
	}

	ordered := orderEntriesForDisplay(allEntries)
	entries := make([]StatusEntry, 0, len(ordered))
	readyPosition := 1

	for _, entry := range ordered {
		statusEntry := StatusEntry{Entry: entry}
		if entry.State == StateReady {
			statusEntry.ReadyPosition = readyPosition
			readyPosition++
		}
		entries = append(entries, statusEntry)
	}

	if len(entries) > displayLimit {
		entries = entries[:displayLimit]
	}

	status.Entries = entries
	return status
}
