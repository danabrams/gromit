package runner

// RunnerSmokeApprovedMatrixCases returns the approved runner-level smoke cases.
func RunnerSmokeApprovedMatrixCases() map[string]bool {
	return map[string]bool{
		"TestRunnerSmoke_RunSingleBeadHappyPath":         true,
		"TestRunnerSmoke_ValidationFailureEscalatesTier": true,
		"TestRunnerSmoke_WorktreeMergeModesEndToEnd":     true,
	}
}
