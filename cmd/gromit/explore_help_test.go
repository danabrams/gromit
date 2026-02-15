package main

import (
	"strings"
	"testing"
)

func TestExploreHelpIncludesCodexExample(t *testing.T) {
	if !strings.Contains(exploreCmd.Long, exploreCodexHelpExample) {
		t.Fatalf("expected explore help to include codex example %q", exploreCodexHelpExample)
	}
}
