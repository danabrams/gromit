package runner

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
