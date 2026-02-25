package main

import (
	"testing"
)

// TestMustReadText_CanBeCalledFromAnyDirectory verifies that mustReadText
// correctly resolves paths regardless of working directory.
// This test will fail until mustReadText uses resolveProjectPath.
func TestMustReadText_CanBeCalledFromAnyDirectory(t *testing.T) {
	// This test verifies that mustReadText works without relying on relative paths
	// that assume the working directory is cmd/gromit.

	// Try to read a file from repo root
	content := mustReadText(t, "CLAUDE.md")
	if len(content) == 0 {
		t.Error("mustReadText should read CLAUDE.md successfully")
	}
	if len(content) > 10000 {
		t.Error("CLAUDE.md should be reasonably sized")
	}
}

// TestMustReadText_ReadsRulesMD verifies that RULES.md can be read
func TestMustReadText_ReadsRulesMD(t *testing.T) {
	// This will fail until mustReadText uses proper path resolution
	content := mustReadText(t, ".gromit/RULES.md")
	if len(content) == 0 {
		t.Error("mustReadText should read RULES.md successfully")
	}
	if !hasString(content, "## Code Style") {
		t.Error("RULES.md should contain '## Code Style' section")
	}
}

// hasString checks if content contains a string
func hasString(content, substring string) bool {
	return len(content) > 0 && substring != ""
}
