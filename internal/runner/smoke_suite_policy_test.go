package runner

import (
	"sort"
	"strings"
	"testing"
)

func collectRunnerAcceptanceTestSet(t *testing.T) map[string]bool {
	t.Helper()

	projectRoot := runnerSmokeSuiteRepoRoot(t)
	files := listRunnerAcceptanceFiles(t, projectRoot)
	tests := make(map[string]bool)

	for _, rel := range files {
		_, names := parseRunnerSmokeMatrixForFile(t, projectRoot, rel)
		for _, name := range names {
			tests[name] = true
		}
	}

	return tests
}

func sortedRunnerAcceptanceTests(t *testing.T) []string {
	t.Helper()

	tests := collectRunnerAcceptanceTestSet(t)
	names := make([]string, 0, len(tests))
	for name := range tests {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestRunnerSmokeSuite_AcceptanceTestsMatchApprovedMatrix(t *testing.T) {
	_ = RunnerSmokeSuiteMatrixVersion

	approved := RunnerSmokeApprovedMatrixCases()
	actual := collectRunnerAcceptanceTestSet(t)

	if len(actual) != len(approved) {
		t.Fatalf("runner acceptance tests count=%d, want %d (tests=%v)", len(actual), len(approved), sortedRunnerAcceptanceTests(t))
	}
	for name := range approved {
		if !actual[name] {
			t.Fatalf("runner acceptance suite missing approved smoke case %s", name)
		}
	}
}

func TestRunnerSmokeSuite_UsesSmokePrefixOnly(t *testing.T) {
	prefix := RunnerSmokeSuiteTestPrefix

	actual := collectRunnerAcceptanceTestSet(t)
	for name := range actual {
		if !strings.HasPrefix(name, prefix) {
			t.Fatalf("runner acceptance test %s missing smoke prefix %q", name, prefix)
		}
	}
}

func TestRunnerSmokeSuite_IsLean(t *testing.T) {
	maxCases := RunnerSmokeSuiteMaxCases

	actual := collectRunnerAcceptanceTestSet(t)
	if len(actual) > maxCases {
		t.Fatalf("runner acceptance smoke suite has %d cases, expected <= %d", len(actual), maxCases)
	}
}
