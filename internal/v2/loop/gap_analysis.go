package loop

import "github.com/danabrams/gromit/internal/bead"

// FlagChangedBeads returns the subset of beads whose committed files overlap
// with the changed files. This is the core of resume gap analysis: given a
// file-level diff and a bead-to-file mapping from commit history, it identifies
// which completed beads need selective revalidation.
func FlagChangedBeads(beads []*bead.Bead, changedFiles []string, beadFiles map[string][]string) []*bead.Bead {
	changed := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = true
	}

	var flagged []*bead.Bead
	for _, b := range beads {
		if b == nil {
			continue
		}
		for _, f := range beadFiles[b.ID] {
			if changed[f] {
				flagged = append(flagged, b)
				break
			}
		}
	}
	return flagged
}
