package integrationqueue

import (
	"testing"
)

func TestClassifyLane_SafeLaneForMetadataOnlyPaths(t *testing.T) {
	files := []string{
		".gromit/config.yaml",
		".gromit/state.json",
		"docs/README.md",
		"docs/api.md",
		"specs/design.md",
	}

	lane := ClassifyLane(files)
	if lane != SafeLane {
		t.Fatalf("ClassifyLane(%v) = %s, want %s", files, lane, SafeLane)
	}
}
