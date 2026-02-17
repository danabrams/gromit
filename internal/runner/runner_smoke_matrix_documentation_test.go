//go:build acceptance

package runner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func collectRunnerAcceptanceCases(t *testing.T, projectRoot string) []string {
	t.Helper()

	files := listRunnerAcceptanceFiles(t, projectRoot)
	cases := make([]string, 0, len(files))

	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(projectRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}

		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			if m := runnerSmokeFuncRe.FindStringSubmatch(line); m != nil {
				cases = append(cases, rel+":"+strings.TrimSpace(m[1]))
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatalf("scan %s: %v", rel, err)
		}
	}

	return cases
}

func TestRunnerSmokeMatrix_DocumentedCasesCoverAcceptanceSuite(t *testing.T) {
	projectRoot := runnerSmokeSuiteRepoRoot(t)

	matrix, err := LoadRunnerSmokeMatrix(projectRoot)
	if err != nil {
		t.Fatalf("LoadRunnerSmokeMatrix: %v", err)
	}

	cases := collectRunnerAcceptanceCases(t, projectRoot)
	for _, caseID := range cases {
		entry, ok := matrix[caseID]
		if !ok {
			t.Fatalf("runner smoke matrix missing case %s", caseID)
		}
		if entry.Decision != "keep" && entry.Decision != "move" {
			t.Fatalf("%s has invalid decision %q", caseID, entry.Decision)
		}
		if entry.Rationale == "" || entry.Rationale == "-" {
			t.Fatalf("%s has empty rationale", caseID)
		}
	}
}

func TestRunnerSmokeMatrix_DocumentedKeepSetIsExact(t *testing.T) {
	projectRoot := runnerSmokeSuiteRepoRoot(t)

	matrix, err := LoadRunnerSmokeMatrix(projectRoot)
	if err != nil {
		t.Fatalf("LoadRunnerSmokeMatrix: %v", err)
	}

	expectedKeep := map[string]bool{
		"internal/runner/validation_extraction_acceptance_test.go:TestRunnerSmoke_RunSingleBeadHappyPath":      true,
		"internal/runner/invocation_timeout_acceptance_test.go:TestRunnerSmoke_ValidationFailureEscalatesTier": true,
		"internal/runner/worktree_merge_acceptance_test.go:TestRunnerSmoke_WorktreeMergeModesEndToEnd":         true,
	}

	actualKeep := make(map[string]bool)
	for caseID, entry := range matrix {
		if entry.Decision == "keep" {
			actualKeep[caseID] = true
		}
	}

	if len(actualKeep) != len(expectedKeep) {
		t.Fatalf("runner keep set size=%d, want=%d (actual=%v)", len(actualKeep), len(expectedKeep), actualKeep)
	}
	for caseID := range expectedKeep {
		if !actualKeep[caseID] {
			t.Fatalf("runner keep set missing %s", caseID)
		}
	}
}

func TestRunnerSmokeMatrix_DocumentedMoveCasesMapToUnitDestinations(t *testing.T) {
	projectRoot := runnerSmokeSuiteRepoRoot(t)

	matrix, err := LoadRunnerSmokeMatrix(projectRoot)
	if err != nil {
		t.Fatalf("LoadRunnerSmokeMatrix: %v", err)
	}

	expectedMoved := map[string]string{
		"internal/runner/andon/policy_classification_acceptance_test.go:TestEvaluateFailure_ClassifiesAndSelectsDecisionForAllClasses":                        "internal/runner/andon/policy_test.go:TestEvaluateFailureWithTrace_CoversDecisionPathForEachClass",
		"internal/runner/andon/policy_classification_acceptance_test.go:TestEvaluateFailure_EnforcesL1L2BoundaryAtPublicEntryPoint":                           "internal/runner/andon/policy_test.go:TestEvaluateFailure_EnforcesL1ToL2BoundaryAtRetryCap",
		"internal/runner/andon/policy_class_aware_selection_acceptance_test.go:TestEvaluateFailure_UsesClassifiedDecisionPathAtPublicEntryPoint":              "internal/runner/andon/policy_test.go:TestEvaluateFailureWithTrace_CoversDecisionPathForEachClass",
		"internal/runner/andon/policy_class_aware_selection_acceptance_test.go:TestEvaluateClassifiedFailure_HasExplicitDecisionPathPerFailureClass":          "internal/runner/andon/policy_test.go:TestEvaluateClassifiedFailure_HasExplicitPathPerClass",
		"internal/runner/andon/policy_class_aware_selection_acceptance_test.go:TestEvaluateFailure_UnknownSignalRemainsDeterministicWithWorkflowFallbackPath": "internal/runner/andon/policy_test.go:TestEvaluateFailure_UnknownKindUsesDeterministicWorkflowFallbackPath",
		"internal/runner/andon/types_acceptance_test.go:TestFailureClasses_CanonicalCatalog":                                                                  "internal/runner/andon/types_test.go:TestAllFailureClasses_CanonicalOrderAndLabels",
		"internal/runner/andon/types_acceptance_test.go:TestLevels_CanonicalCatalog":                                                                          "internal/runner/andon/types_test.go:TestAllAndonLevels_CanonicalOrder",
		"internal/runner/andon/types_acceptance_test.go:TestDefaultThresholdDefinition_IsPureAndPolicyConsumable_Acceptance":                                  "internal/runner/andon/types_test.go:TestDefaultThresholdDefinition_IsPureAndPolicyConsumable",
	}

	unitTests := listRunnerUnitTests(t, projectRoot)

	for caseID, expectedDestination := range expectedMoved {
		entry, ok := matrix[caseID]
		if !ok {
			t.Fatalf("runner smoke matrix missing moved case %s", caseID)
		}
		if entry.Decision != "move" {
			t.Fatalf("%s decision=%q, want move", caseID, entry.Decision)
		}
		if entry.Destination != expectedDestination {
			t.Fatalf("%s destination=%q, want %q", caseID, entry.Destination, expectedDestination)
		}

		parts := strings.Split(entry.Destination, ":")
		if len(parts) != 2 {
			t.Fatalf("%s destination %q must be file:suite", caseID, entry.Destination)
		}
		if !strings.HasSuffix(parts[0], "_test.go") {
			t.Fatalf("%s destination file must be *_test.go, got %q", caseID, parts[0])
		}
		if !strings.Contains(unitTests, parts[1]) {
			t.Fatalf("runner unit suite missing destination test %s", parts[1])
		}
	}
}
