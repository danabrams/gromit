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

	reclassifiedExpectations := []struct {
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

	acceptanceExpectations := []struct {
		relPath string
		tag     string
	}{
		{
			relPath: filepath.Join("test", "contracts", "codex_harness_acceptance_test.go"),
			tag:     "acceptance",
		},
		{
			relPath: filepath.Join("test", "e2e", "codex_harness_acceptance_test.go"),
			tag:     "acceptance",
		},
	}

	for i := range reclassifiedExpectations {
		reclassified := reclassifiedExpectations[i]
		acceptance := acceptanceExpectations[i]
		reclassifiedPath := filepath.Join(projectRoot, reclassified.relPath)
		acceptancePath := filepath.Join(projectRoot, acceptance.relPath)
		_, reclassifiedErr := os.Stat(reclassifiedPath)
		_, acceptanceErr := os.Stat(acceptancePath)

		switch {
		case reclassifiedErr == nil:
			if !hasBuildTag(t, reclassifiedPath, reclassified.tag) {
				t.Errorf("%s must include //go:build %s", reclassified.relPath, reclassified.tag)
			}
			if acceptanceErr == nil && !hasBuildTag(t, acceptancePath, acceptance.tag) {
				t.Errorf("%s must include //go:build %s when present", acceptance.relPath, acceptance.tag)
			}
		case acceptanceErr == nil:
			if !hasBuildTag(t, acceptancePath, acceptance.tag) {
				t.Errorf("%s must include //go:build %s", acceptance.relPath, acceptance.tag)
			}
		case os.IsNotExist(reclassifiedErr) && os.IsNotExist(acceptanceErr):
			t.Errorf("expected either %s or %s to exist", reclassified.relPath, acceptance.relPath)
		default:
			if reclassifiedErr != nil && !os.IsNotExist(reclassifiedErr) {
				t.Fatalf("stat %s: %v", reclassified.relPath, reclassifiedErr)
			}
			if acceptanceErr != nil && !os.IsNotExist(acceptanceErr) {
				t.Fatalf("stat %s: %v", acceptance.relPath, acceptanceErr)
			}
		}
	}
}
