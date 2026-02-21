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

	type buildTagExpectation struct {
		reclassifiedRelPath string
		reclassifiedTag     string
		acceptanceRelPath   string
		acceptanceTag       string
	}

	expectations := []buildTagExpectation{
		{
			reclassifiedRelPath: filepath.Join("test", "contracts", "codex_harness_test.go"),
			reclassifiedTag:     "contract",
			acceptanceRelPath:   filepath.Join("test", "contracts", "codex_harness_acceptance_test.go"),
			acceptanceTag:       "acceptance",
		},
		{
			reclassifiedRelPath: filepath.Join("test", "e2e", "codex_harness_test.go"),
			reclassifiedTag:     "e2e",
			acceptanceRelPath:   filepath.Join("test", "e2e", "codex_harness_acceptance_test.go"),
			acceptanceTag:       "acceptance",
		},
	}

	for _, expectation := range expectations {
		reclassifiedPath := filepath.Join(projectRoot, expectation.reclassifiedRelPath)
		acceptancePath := filepath.Join(projectRoot, expectation.acceptanceRelPath)
		_, reclassifiedErr := os.Stat(reclassifiedPath)
		_, acceptanceErr := os.Stat(acceptancePath)

		switch {
		case reclassifiedErr == nil:
			if !hasBuildTag(t, reclassifiedPath, expectation.reclassifiedTag) {
				t.Errorf("%s must include //go:build %s", expectation.reclassifiedRelPath, expectation.reclassifiedTag)
			}
			if acceptanceErr == nil && !hasBuildTag(t, acceptancePath, expectation.acceptanceTag) {
				t.Errorf("%s must include //go:build %s when present", expectation.acceptanceRelPath, expectation.acceptanceTag)
			}
		case acceptanceErr == nil:
			if !hasBuildTag(t, acceptancePath, expectation.acceptanceTag) {
				t.Errorf("%s must include //go:build %s", expectation.acceptanceRelPath, expectation.acceptanceTag)
			}
		case os.IsNotExist(reclassifiedErr) && os.IsNotExist(acceptanceErr):
			t.Errorf("expected either %s or %s to exist", expectation.reclassifiedRelPath, expectation.acceptanceRelPath)
		default:
			if reclassifiedErr != nil && !os.IsNotExist(reclassifiedErr) {
				t.Fatalf("stat %s: %v", expectation.reclassifiedRelPath, reclassifiedErr)
			}
			if acceptanceErr != nil && !os.IsNotExist(acceptanceErr) {
				t.Fatalf("stat %s: %v", expectation.acceptanceRelPath, acceptanceErr)
			}
		}
	}
}
