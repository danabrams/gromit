package main

import "testing"

func TestDebugHelpMatchesGolden(t *testing.T) {
	t.Parallel()
	assertHelpMatchesGolden(t, "cmd/gromit/testdata/golden/debug.help.txt", debugCmd.Long)
}
