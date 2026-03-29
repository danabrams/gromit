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

// FlagChangedBeads returns beads that need revalidation when the diff file lists
// changed files. The initial implementation returns all beads when any files
// are reported as changed.
func FlagChangedBeads(beads []*bead.Bead, changedFiles []string, beadFiles map[string][]string) []*bead.Bead {
	if len(changedFiles) == 0 {
		return nil
	}
	return beads
}
