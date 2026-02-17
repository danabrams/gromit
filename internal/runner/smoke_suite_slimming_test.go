package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func runnerSmokeSuiteRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

func readRunnerAcceptanceFile(t *testing.T, projectRoot, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectRoot, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func countRunnerAcceptanceTests(src string) int {
	return strings.Count(src, "\nfunc Test")
}

func TestRunnerAcceptanceSmokeSuite_IsSlimAndFocused(t *testing.T) {
	// Expected failure: future runner smoke tests such as
	// TestRunnerSmoke_RunSingleBeadHappyPath and
	// TestRunnerSmoke_ValidationFailureEscalatesTier do not exist yet, and the
	// acceptance suite still includes many non-smoke verification files.
	projectRoot := runnerSmokeSuiteRepoRoot(t)

	matches, err := filepath.Glob(filepath.Join(projectRoot, "internal/runner/*_acceptance_test.go"))
	if err != nil {
		t.Fatalf("glob runner acceptance files: %v", err)
	}

	allowedFiles := map[string]bool{
		"internal/runner/validation_extraction_acceptance_test.go": true,
		"internal/runner/invocation_timeout_acceptance_test.go":    true,
		"internal/runner/worktree_merge_acceptance_test.go":        true,
	}

	for _, abs := range matches {
		rel, err := filepath.Rel(projectRoot, abs)
		if err != nil {
			t.Fatalf("filepath.Rel: %v", err)
		}
		rel = filepath.ToSlash(rel)
		if !allowedFiles[rel] {
			t.Fatalf("unexpected runner acceptance file in smoke suite: %s", rel)
		}
	}

	requiredFutureSmokeTests := map[string][]string{
		"internal/runner/validation_extraction_acceptance_test.go": {
			"func TestRunnerSmoke_RunSingleBeadHappyPath",
		},
		"internal/runner/invocation_timeout_acceptance_test.go": {
			"func TestRunnerSmoke_ValidationFailureEscalatesTier",
		},
		"internal/runner/worktree_merge_acceptance_test.go": {
			"func TestRunnerSmoke_WorktreeMergeModesEndToEnd",
		},
	}

	totalTests := 0
	for rel, mustContain := range requiredFutureSmokeTests {
		src := readRunnerAcceptanceFile(t, projectRoot, rel)
		totalTests += countRunnerAcceptanceTests(src)
		for _, symbol := range mustContain {
			if !strings.Contains(src, symbol) {
				t.Fatalf("%s missing future smoke test %q", rel, symbol)
			}
		}
	}

	forbiddenNonE2ETests := []string{
		"func TestRunnerInvocationTimeout_UsesClaudeTimeout(",
		"func TestRunnerInvocationTimeout_RespectsModelOverride(",
		"func TestRunnerLogsPhaseTimeoutWithElapsedDuration(",
		"func TestRunnerInvocationTimeout_DefaultTimeoutApplied(",
		"func TestRunnerRun_MergeInteractiveBranchesStopsOnFailure(",
		"func TestRunnerRun_SkipsMergeWhenAutoMergeDisabled(",
		"func TestNewRunner_WiresWorktreeManagerWhenEnabled(",
	}
	for _, token := range forbiddenNonE2ETests {
		for rel := range allowedFiles {
			src := readRunnerAcceptanceFile(t, projectRoot, rel)
			if strings.Contains(src, token) {
				t.Fatalf("%s still contains non-E2E test %q", rel, token)
			}
		}
	}

	if totalTests > 12 {
		t.Fatalf("runner acceptance smoke suite has %d tests, expected <= 12", totalTests)
	}
}

func TestRunnerNonE2EChecks_AreReclassifiedIntoUnitSuite(t *testing.T) {
	// Expected failure: unit-test replacements for runner non-E2E acceptance
	// checks, including TestRunnerSplitVerificationReclassified_LineBudgets and
	// TestScopeCheckReclassified_CachedEstimateSkipsDuplicateInvocation, do not
	// exist yet in internal/runner unit tests.
	projectRoot := runnerSmokeSuiteRepoRoot(t)

	listed := runnerPackageTestNameIndex(t, projectRoot, "internal/runner")
	requiredFutureUnitTests := []string{
		"TestRunnerSplitVerificationReclassified_LineBudgets",
		"TestRunnerSplitVerificationReclassified_ImportIsolation",
		"TestScopeCheckReclassified_CachedEstimateSkipsDuplicateInvocation",
	}
	for _, name := range requiredFutureUnitTests {
		if _, ok := listed[name]; !ok {
			t.Fatalf("runner unit suite missing reclassified non-E2E test %s", name)
		}
	}
}
