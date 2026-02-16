package runner

import "testing"

func TestRunnerSmokeApprovedMatrixCases_IncludesExpected(t *testing.T) {
	approved := RunnerSmokeApprovedMatrixCases()

	expected := []string{
		"TestRunnerSmoke_RunSingleBeadHappyPath",
		"TestRunnerSmoke_ValidationFailureEscalatesTier",
		"TestRunnerSmoke_WorktreeMergeModesEndToEnd",
	}
	for _, name := range expected {
		if !approved[name] {
			t.Fatalf("approved smoke cases missing %s", name)
		}
	}
}

func TestRunnerSmokeSuiteApprovedRoots_IncludesRunnerRoot(t *testing.T) {
	roots := RunnerSmokeSuiteApprovedRoots()
	if !roots["internal/runner"] {
		t.Fatal("expected internal/runner to be an approved smoke suite root")
	}
}

func TestRunnerSmokeMatrixMovedCases_IncludesAndonCoverage(t *testing.T) {
	moved := RunnerSmokeMatrixMovedCases()

	expected := map[string]string{
		"TestFailureClasses_CanonicalCatalog":                                   "internal/runner/andon/types_test.go:TestAllFailureClasses_CanonicalOrderAndLabels",
		"TestLevels_CanonicalCatalog":                                           "internal/runner/andon/types_test.go:TestAllAndonLevels_CanonicalOrder",
		"TestDefaultThresholdDefinition_IsPureAndPolicyConsumable_Acceptance":   "internal/runner/andon/types_test.go:TestDefaultThresholdDefinition_IsPureAndPolicyConsumable",
		"TestEvaluateFailure_ClassifiesAndSelectsDecisionForAllClasses":          "internal/runner/andon/policy_test.go:TestEvaluateFailureWithTrace_CoversDecisionPathForEachClass",
		"TestEvaluateFailure_EnforcesL1L2BoundaryAtPublicEntryPoint":             "internal/runner/andon/policy_test.go:TestEvaluateFailure_EnforcesL1ToL2BoundaryAtRetryCap",
		"TestEvaluateFailure_UsesClassifiedDecisionPathAtPublicEntryPoint":       "internal/runner/andon/policy_test.go:TestEvaluateFailureWithTrace_CoversDecisionPathForEachClass",
		"TestEvaluateClassifiedFailure_HasExplicitDecisionPathPerFailureClass":   "internal/runner/andon/policy_test.go:TestEvaluateClassifiedFailure_HasExplicitPathPerClass",
		"TestEvaluateFailure_UnknownSignalRemainsDeterministicWithWorkflowFallbackPath": "internal/runner/andon/policy_test.go:TestEvaluateFailure_UnknownKindUsesDeterministicWorkflowFallbackPath",
	}

	for source, destination := range expected {
		if moved[source] != destination {
			t.Fatalf("moved smoke case %s = %q, want %q", source, moved[source], destination)
		}
	}
}
