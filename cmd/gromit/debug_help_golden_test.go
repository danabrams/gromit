package main

import (
	"strings"
	"testing"
)

func TestDebugHelpMatchesGolden(t *testing.T) {
	t.Parallel()
	assertHelpMatchesGolden(t, "cmd/gromit/testdata/golden/debug.help.txt", debugCmd.Long)
}

func TestDebugHelpMentionsDebugJobs(t *testing.T) {
	t.Parallel()
	text := strings.ToLower(debugCmd.Long)
	for _, job := range []string{"diagnose", "fix", "learn"} {
		if !strings.Contains(text, job) {
			t.Fatalf("debug help missing mention of %s job", job)
		}
	}
}
