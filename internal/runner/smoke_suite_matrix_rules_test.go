//go:build acceptance
// +build acceptance

package runner

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerSmokeSuite_ApprovedMatrixCasesOnly(t *testing.T) {
	// Expected failure: RunnerSmokeApprovedMatrixCases does not exist yet and
	// acceptance files still include non-smoke scenarios outside the approved set.
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
	// Expected failure: RunnerSmokeSuiteApprovedRoots does not exist yet and
	// acceptance tests still live under internal/runner subpackages like andon.
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
	// Expected failure: RunnerSmokeMatrixMovedCases does not exist yet and
	// reclassified unit tests are not yet present for all moved behavior.
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
		if !strings.Contains(unitTests, destination) {
			t.Fatalf("unit suite missing destination test %s for moved case %s", destination, source)
		}
	}
}
