package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssertCodexHarnessAcceptanceBuildTagsApplied(t *testing.T) {
	AssertCodexHarnessAcceptanceBuildTagsApplied(t)
}

func AssertCodexHarnessAcceptanceBuildTagsApplied(t *testing.T) {
	t.Helper()

	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	expectations := []struct {
		relPath string
		tag     string
	}{
		{
			relPath: filepath.Join("test", "contracts", "codex_harness_test.go"),
			tag:     "contract",
		},
		{
			relPath: filepath.Join("test", "e2e", "codex_harness_test.go"),
			tag:     "e2e",
		},
	}

	for _, expect := range expectations {
		fullPath := filepath.Join(projectRoot, expect.relPath)
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", expect.relPath, err)
		}
		if info.IsDir() {
			t.Fatalf("expected %s to be a file, found directory", expect.relPath)
		}
		if !hasBuildTag(t, fullPath, expect.tag) {
			t.Errorf("%s must include //go:build %s", expect.relPath, expect.tag)
		}
	}

	removedArtifacts := []string{
		filepath.Join("test", "contracts", "codex_harness_acceptance_test.go"),
		filepath.Join("test", "e2e", "codex_harness_acceptance_test.go"),
	}

	for _, relPath := range removedArtifacts {
		fullPath := filepath.Join(projectRoot, relPath)
		if _, err := os.Stat(fullPath); err == nil {
			t.Errorf("%s should be removed; coverage now lives in codex_harness_test.go under contract/e2e build tags", relPath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", relPath, err)
		}
	}
}
