//go:build acceptance

package main

import (
	"strings"
	"testing"
)

func runExploreHelp(t *testing.T) string {
	t.Helper()

	stdout, stderr, exitCode := runGromit(t, "explore", "--help")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", exitCode, stderr)
	}

	if strings.Contains(stderr, "unknown flag") {
		t.Fatalf("unexpected flag error: %s", stderr)
	}

	return stdout
}

func TestExploreHelpIncludesCodexAgentExample(t *testing.T) {
	// Expected failure: exploreCodexHelpExample constant does not exist yet and explore help lacks codex selection example
	stdout := runExploreHelp(t)
	expectedExample := exploreCodexHelpExample

	if !strings.Contains(stdout, expectedExample) {
		t.Fatalf("expected explore help to include codex example %q, got: %s", expectedExample, stdout)
	}
}

func TestExploreHelpDocumentsAgentSelectionBehavior(t *testing.T) {
	// Expected failure: exploreAgentSelectionHelpSentence constant does not exist yet and help lacks agent selection guidance
	stdout := runExploreHelp(t)
	expectedSentence := exploreAgentSelectionHelpSentence

	if !strings.Contains(stdout, expectedSentence) {
		t.Fatalf("expected explore help to include agent selection guidance %q, got: %s", expectedSentence, stdout)
	}
}
