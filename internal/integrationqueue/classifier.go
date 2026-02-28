package integrationqueue

import "strings"

// ClassifyLane inspects changed file paths and returns the appropriate execution lane.
// Returns SafeLane for metadata-only paths (.gromit/**, docs, specs)
// Returns CodeLane for all source/test/config/build changes.
func ClassifyLane(changedFiles []string) Lane {
	if len(changedFiles) == 0 {
		return SafeLane
	}

	for _, file := range changedFiles {
		if !isMetadataPath(file) {
			return CodeLane
		}
	}

	return SafeLane
}

func isMetadataPath(file string) bool {
	// .gromit/** paths are metadata
	if strings.HasPrefix(file, ".gromit/") {
		return true
	}

	// docs/** paths are metadata
	if strings.HasPrefix(file, "docs/") {
		return true
	}

	// specs/** paths are metadata
	if strings.HasPrefix(file, "specs/") {
		return true
	}

	return false
}
