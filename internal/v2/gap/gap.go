package gap

import (
	"os"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
)

// ReadChangedFiles returns the list of files recorded in the gap analysis diff file.
func ReadChangedFiles(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var changed []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		changed = append(changed, line)
	}
	return changed, nil
}

// FlagChangedBeads returns beads whose files overlap the changed files recorded
// in the diff file.
func FlagChangedBeads(beads []*bead.Bead, changedFiles []string, beadFiles map[string][]string) []*bead.Bead {
	changed := make(map[string]struct{}, len(changedFiles))
	for _, f := range changedFiles {
		if f == "" {
			continue
		}
		changed[f] = struct{}{}
	}

	var flagged []*bead.Bead
	for _, b := range beads {
		if b == nil {
			continue
		}
		for _, f := range beadFiles[b.ID] {
			if _, ok := changed[f]; ok {
				flagged = append(flagged, b)
				break
			}
		}
	}
	return flagged
}
