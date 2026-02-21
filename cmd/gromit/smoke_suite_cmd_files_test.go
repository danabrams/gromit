package main

import (
	"path/filepath"
	"testing"
)

func TestCmdSmokeSuiteReclassified_AcceptanceFileSet(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(projectRoot, "cmd/gromit/*_acceptance_test.go"))
	if err != nil {
		t.Fatalf("glob cmd acceptance files: %v", err)
	}

	allowedFiles := map[string]bool{
		filepath.Join(projectRoot, "cmd/gromit/debug_agent_acceptance_test.go"):              true,
		filepath.Join(projectRoot, "cmd/gromit/explore_codex_help_acceptance_test.go"):       true,
		filepath.Join(projectRoot, "cmd/gromit/review_spec_validation_acceptance_test.go"):   true,
	}

	for _, abs := range matches {
		if !allowedFiles[abs] {
			t.Fatalf("unexpected cmd acceptance file in smoke suite: %s", abs)
		}
	}
}
