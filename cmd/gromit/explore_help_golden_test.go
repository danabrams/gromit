package main

import "testing"

func TestExploreHelpMatchesGolden(t *testing.T) {
	t.Parallel()
	assertHelpMatchesGolden(t, "cmd/gromit/testdata/golden/explore.help.txt", exploreCmd.Long)
}
