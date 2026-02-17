package runner

// RunnerSmokeSuiteMatrixVersion is the version of the approved smoke matrix.
const RunnerSmokeSuiteMatrixVersion = "v1"

// RunnerSmokeSuiteTestPrefix is the required prefix for all runner smoke acceptance tests.
const RunnerSmokeSuiteTestPrefix = "TestRunnerSmoke_"

// RunnerSmokeSuiteMaxCases is the maximum number of runner smoke acceptance tests allowed.
const RunnerSmokeSuiteMaxCases = 3

// RunnerSmokeApprovedMatrixCases returns the approved runner-level smoke cases.
func RunnerSmokeApprovedMatrixCases() map[string]bool {
	return map[string]bool{
		"TestRunnerSmoke_RunSingleBeadHappyPath":         true,
		"TestRunnerSmoke_ValidationFailureEscalatesTier": true,
		"TestRunnerSmoke_WorktreeMergeModesEndToEnd":     true,
	}
}

// RunnerSmokeSuiteApprovedRoots returns the only directories that should contain
// runner-level acceptance tests in the smoke suite.
func RunnerSmokeSuiteApprovedRoots() map[string]bool {
	return map[string]bool{
		"internal/runner": true,
	}
}

// RunnerSmokeMatrixMovedCases lists acceptance tests that were reclassified into
// unit coverage, mapping the old acceptance test name to its unit destination.
func RunnerSmokeMatrixMovedCases() map[string]string {
	return map[string]string{
		"TestFailureClasses_CanonicalCatalog":                                           "internal/runner/andon/types_test.go:TestAllFailureClasses_CanonicalOrderAndLabels",
		"TestLevels_CanonicalCatalog":                                                   "internal/runner/andon/types_test.go:TestAllAndonLevels_CanonicalOrder",
		"TestDefaultThresholdDefinition_IsPureAndPolicyConsumable_Acceptance":           "internal/runner/andon/types_test.go:TestDefaultThresholdDefinition_IsPureAndPolicyConsumable",
		"TestEvaluateFailure_ClassifiesAndSelectsDecisionForAllClasses":                 "internal/runner/andon/policy_test.go:TestEvaluateFailureWithTrace_CoversDecisionPathForEachClass",
		"TestEvaluateFailure_EnforcesL1L2BoundaryAtPublicEntryPoint":                    "internal/runner/andon/policy_test.go:TestEvaluateFailure_EnforcesL1ToL2BoundaryAtRetryCap",
		"TestEvaluateFailure_UsesClassifiedDecisionPathAtPublicEntryPoint":              "internal/runner/andon/policy_test.go:TestEvaluateFailureWithTrace_CoversDecisionPathForEachClass",
		"TestEvaluateClassifiedFailure_HasExplicitDecisionPathPerFailureClass":          "internal/runner/andon/policy_test.go:TestEvaluateClassifiedFailure_HasExplicitPathPerClass",
		"TestEvaluateFailure_UnknownSignalRemainsDeterministicWithWorkflowFallbackPath": "internal/runner/andon/policy_test.go:TestEvaluateFailure_UnknownKindUsesDeterministicWorkflowFallbackPath",
	}
}
