package playbook

// MergedPlaybook loads entries from both global and local directories,
// merges them with local-wins semantics, and returns the merged slice.
//
// Merge behavior:
// - Entries with matching IDs: local entry wins
// - Local entry with Status="superseded": masks the global entry
// - Otherwise: global entry is used for unique IDs
func MergedPlaybook(globalDir, localDir string) ([]Entry, error) {
	// Load global entries
	globalStore := Store{Dir: globalDir}
	globalEntries, err := globalStore.Load()
	if err != nil {
		return nil, err
	}

	// Load local entries
	localStore := Store{Dir: localDir}
	localEntries, err := localStore.Load()
	if err != nil {
		return nil, err
	}

	// Build a map of local entries by ID for quick lookup
	localByID := make(map[string]Entry)
	for _, entry := range localEntries {
		localByID[entry.ID] = entry
	}

	// Build a map of local superseded IDs (entries with Status="superseded")
	supersededGlobalIDs := make(map[string]bool)
	for _, entry := range localEntries {
		if entry.Status == "superseded" {
			supersededGlobalIDs[entry.ID] = true
		}
	}

	// Merge: start with global entries, override with local where present
	merged := []Entry{}

	// Add global entries, unless masked by local superseded entry
	for _, entry := range globalEntries {
		if supersededGlobalIDs[entry.ID] {
			// Skip global entry if local has Status="superseded" for same ID
			continue
		}
		if localEntry, exists := localByID[entry.ID]; exists {
			// Local wins for matching IDs
			merged = append(merged, localEntry)
		} else {
			// No local override, use global
			merged = append(merged, entry)
		}
	}

	// Add local entries that don't exist in global (new local entries)
	globalIDs := make(map[string]bool)
	for _, entry := range globalEntries {
		globalIDs[entry.ID] = true
	}

	for _, localEntry := range localEntries {
		if !globalIDs[localEntry.ID] {
			// This local entry is new (not in global)
			merged = append(merged, localEntry)
		}
	}

	return merged, nil
}
