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
