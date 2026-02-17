package runner

import "testing"

func TestLoadRunnerSmokeMatrix_IncludesRunnerSmokeCase(t *testing.T) {
	projectRoot := runnerSmokeSuiteRepoRoot(t)

	matrix, err := LoadRunnerSmokeMatrix(projectRoot)
	if err != nil {
		t.Fatalf("LoadRunnerSmokeMatrix: %v", err)
	}

	caseID := "internal/runner/validation_extraction_acceptance_test.go:TestRunnerSmoke_RunSingleBeadHappyPath"
	entry, ok := matrix[caseID]
	if !ok {
		t.Fatalf("missing smoke matrix entry for %s", caseID)
	}
	if entry.Decision != "keep" {
		t.Fatalf("%s decision=%q, want keep", caseID, entry.Decision)
	}
	if entry.Destination != "internal/runner/validation_extraction_acceptance_test.go:TestRunnerSmoke_RunSingleBeadHappyPath" {
		t.Fatalf("%s destination=%q, want %q", caseID, entry.Destination, caseID)
	}
}

func TestLoadRunnerSmokeMatrix_SkipsNonRunnerCases(t *testing.T) {
	projectRoot := runnerSmokeSuiteRepoRoot(t)

	matrix, err := LoadRunnerSmokeMatrix(projectRoot)
	if err != nil {
		t.Fatalf("LoadRunnerSmokeMatrix: %v", err)
	}

	caseID := "cmd/gromit/debug_agent_acceptance_test.go:TestCmdSmoke_DebugAgentResolutionEndToEnd"
	if _, ok := matrix[caseID]; ok {
		t.Fatalf("unexpected non-runner case %s in runner smoke matrix", caseID)
	}
}
