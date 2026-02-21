package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadCmdAcceptanceFile(t *testing.T, projectRoot, rel string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(projectRoot, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func countCmdAcceptanceTests(src string) int {
	return strings.Count(src, "\nfunc Test")
}

func listCmdAcceptanceTests(src string) []string {
	lines := strings.Split(src, "\n")
	var names []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "func Test") {
			continue
		}
		afterFunc := strings.TrimPrefix(trimmed, "func ")
		parenIdx := strings.Index(afterFunc, "(")
		if parenIdx == -1 {
			continue
		}
		names = append(names, afterFunc[:parenIdx])
	}
	return names
}

func TestCmdDebugAcceptanceFile_OnlyContainsSmokeTest(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	src := loadCmdAcceptanceFile(t, projectRoot, "cmd/gromit/debug_agent_acceptance_test.go")
	tests := listCmdAcceptanceTests(src)
	if len(tests) != 1 {
		t.Fatalf("debug_agent_acceptance_test.go has %d tests, expected 1", len(tests))
	}
	for _, name := range tests {
		if !strings.HasPrefix(name, "TestCmdSmoke_") {
			t.Fatalf("debug_agent_acceptance_test.go contains non-smoke test %s", name)
		}
	}
}

func TestCmdExploreAcceptanceFile_OnlyContainsSmokeTest(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	src := loadCmdAcceptanceFile(t, projectRoot, "cmd/gromit/explore_codex_help_acceptance_test.go")
	tests := listCmdAcceptanceTests(src)
	if len(tests) != 1 {
		t.Fatalf("explore_codex_help_acceptance_test.go has %d tests, expected 1", len(tests))
	}
	for _, name := range tests {
		if !strings.HasPrefix(name, "TestCmdSmoke_") {
			t.Fatalf("explore_codex_help_acceptance_test.go contains non-smoke test %s", name)
		}
	}
}

func TestCmdAcceptanceSmokeSuite_IsSlimAndFocused(t *testing.T) {
	// Expected failure: future smoke-only tests such as
	// TestCmdSmoke_DebugAgentResolutionEndToEnd and
	// TestCmdSmoke_ReviewSpecValidationEndToEnd do not exist yet, and
	// non-smoke acceptance files have not been removed from cmd/gromit.
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(projectRoot, "cmd/gromit/*_acceptance_test.go"))
	if err != nil {
		t.Fatalf("glob cmd acceptance files: %v", err)
	}

	allowedFiles := map[string]bool{
		"cmd/gromit/debug_agent_acceptance_test.go":              true,
		"cmd/gromit/explore_codex_help_acceptance_test.go":       true,
		"cmd/gromit/review_spec_validation_acceptance_test.go":   true,
	}

	for _, abs := range matches {
		rel, err := filepath.Rel(projectRoot, abs)
		if err != nil {
			t.Fatalf("filepath.Rel: %v", err)
		}
		rel = filepath.ToSlash(rel)
		if !allowedFiles[rel] {
			t.Fatalf("unexpected cmd acceptance file in smoke suite: %s", rel)
		}
	}

	requiredFutureSmokeTests := map[string][]string{
		"cmd/gromit/debug_agent_acceptance_test.go": {
			"func TestCmdSmoke_DebugAgentResolutionEndToEnd",
		},
		"cmd/gromit/explore_codex_help_acceptance_test.go": {
			"func TestCmdSmoke_ExploreAgentSelectionEndToEnd",
		},
		"cmd/gromit/review_spec_validation_acceptance_test.go": {
			"func TestCmdSmoke_ReviewSpecValidationEndToEnd",
		},
	}

	totalTests := 0
	for rel, mustContain := range requiredFutureSmokeTests {
		src := loadCmdAcceptanceFile(t, projectRoot, rel)
		totalTests += countCmdAcceptanceTests(src)
		for _, symbol := range mustContain {
			if !strings.Contains(src, symbol) {
				t.Fatalf("%s missing future smoke test %q", rel, symbol)
			}
		}
	}

	forbiddenHelpTests := []string{
		"func TestDebugHelpIncludesAgentFlags(",
		"func TestExploreHelpIncludesCodexAgentExample(",
		"func TestExploreHelpDocumentsAgentSelectionBehavior(",
	}
	for _, token := range forbiddenHelpTests {
		for rel := range allowedFiles {
			src := loadCmdAcceptanceFile(t, projectRoot, rel)
			if strings.Contains(src, token) {
				t.Fatalf("%s still contains non-E2E help assertion %q", rel, token)
			}
		}
	}

	if totalTests > 9 {
		t.Fatalf("cmd acceptance smoke suite has %d tests, expected <= 9", totalTests)
	}
}

func TestCmdNonE2EChecks_AreReclassifiedIntoUnitSuite(t *testing.T) {
	// Expected failure: future unit tests proving non-E2E reclassification,
	// such as TestEpicBeadCountsReclassified_ClosedCountsIncludeAllStatuses and
	// TestReviewSpecValidationRefactorReclassified_UsesSharedHelpers, do not
	// exist yet in cmd/gromit unit tests.
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	listed := packageTestNameIndex(t, projectRoot, "cmd/gromit")
	requiredFutureUnitTests := []string{
		"TestEpicBeadCountsReclassified_ClosedCountsIncludeAllStatuses",
		"TestEpicBeadCountsReclassified_AllClosedBeadsCounted",
		"TestReviewSpecValidationRefactorReclassified_UsesSharedHelpers",
	}
	for _, name := range requiredFutureUnitTests {
		if _, ok := listed[name]; !ok {
			t.Fatalf("cmd unit suite missing reclassified non-E2E test %s", name)
		}
	}
}
