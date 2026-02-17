//go:build acceptance
// +build acceptance

package runner

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerSmokeSuite_ApprovedMatrixCasesOnly(t *testing.T) {
	approved := RunnerSmokeApprovedMatrixCases()
	if len(approved) == 0 {
		t.Fatalf("approved smoke matrix is empty")
	}

	projectRoot := runnerSmokeSuiteRepoRoot(t)
	files := listRunnerAcceptanceFiles(t, projectRoot)

	for _, rel := range files {
		entries, tests := parseRunnerSmokeMatrixForFile(t, projectRoot, rel)
		for _, testName := range tests {
			entry, ok := entries[testName]
			if !ok {
				t.Fatalf("%s missing smoke-matrix mapping for %s", rel, testName)
			}
			if entry.Decision != "keep" {
				t.Fatalf("%s contains non-smoke test %s with decision %q", rel, testName, entry.Decision)
			}
			if !approved[testName] {
				t.Fatalf("%s contains test %s outside approved smoke matrix", rel, testName)
			}
		}
	}
}

func TestRunnerSmokeSuite_NoSubpackageAcceptanceFiles(t *testing.T) {
	allowedRoots := RunnerSmokeSuiteApprovedRoots()
	projectRoot := runnerSmokeSuiteRepoRoot(t)
	files := listRunnerAcceptanceFiles(t, projectRoot)

	for _, rel := range files {
		subdir := filepath.Dir(rel)
		if !allowedRoots[subdir] {
			t.Fatalf("unexpected runner acceptance file in subdir %s: %s", subdir, rel)
		}
		if strings.Contains(rel, "/andon/") {
			t.Fatalf("andon acceptance file should be removed or migrated: %s", rel)
		}
	}
}

func TestRunnerSmokeSuite_MovedBehaviorHasUnitCoverage(t *testing.T) {
	moved := RunnerSmokeMatrixMovedCases()
	if len(moved) == 0 {
		t.Fatalf("moved smoke cases list is empty")
	}

	projectRoot := runnerSmokeSuiteRepoRoot(t)
	unitTests := listRunnerUnitTests(t, projectRoot)

	for source, destination := range moved {
		if destination == "" {
			t.Fatalf("moved case %s missing destination unit test", source)
		}
		parts := strings.Split(destination, ":")
		if len(parts) != 2 {
			t.Fatalf("moved case %s has invalid destination %q (want file:test)", source, destination)
		}
		if _, ok := unitTests[parts[1]]; !ok {
			t.Fatalf("unit suite missing destination test %s for moved case %s", destination, source)
		}
	}
}
