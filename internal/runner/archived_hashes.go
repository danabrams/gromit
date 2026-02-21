package runner

import "sort"

func (r *Runner) syncArchivedHashesFromState(st *runLoopState) {
	if r == nil || st == nil || st.sf == nil || r.renderer == nil {
		return
	}

	lf := r.renderer.GetLearningsFile()
	if lf == nil {
		return
	}

	lf.SetArchivedHashes(sortedHashList(st.sf.GetArchivedHashes()))
}

func (r *Runner) persistArchivedHashesToState(st *runLoopState) {
	if r == nil || st == nil || st.sf == nil || r.renderer == nil {
		return
	}

	lf := r.renderer.GetLearningsFile()
	if lf == nil {
		return
	}

	existing := st.sf.GetArchivedHashes()
	newHashes := diffHashSet(lf.GetArchivedHashes(), existing)
	if len(newHashes) == 0 {
		return
	}

	st.sf.AddArchivedHashes(newHashes)
	if err := st.sf.Save(); err != nil {
		r.log("Warning: could not save state with archived hashes: %v", err)
	}
}

func sortedHashList(hashSet map[string]bool) []string {
	if len(hashSet) == 0 {
		return []string{}
	}

	hashes := make([]string, 0, len(hashSet))
	for hash := range hashSet {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)
	return hashes
}

func diffHashSet(candidate, existing map[string]bool) []string {
	if len(candidate) == 0 {
		return []string{}
	}

	diff := make([]string, 0, len(candidate))
	for hash := range candidate {
		if !existing[hash] {
			diff = append(diff, hash)
		}
	}
	sort.Strings(diff)
	return diff
}
