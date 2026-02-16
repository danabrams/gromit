package main

import (
	"strings"
	"testing"
)

func cmdAcceptanceTestFiles() []string {
	return []string{
		"cmd/gromit/debug_agent_acceptance_test.go",
		"cmd/gromit/explore_codex_help_acceptance_test.go",
		"cmd/gromit/review_spec_validation_acceptance_test.go",
	}
}

func loadCmdSmokeMatrix(t *testing.T) (string, map[string]CmdSmokeMatrixEntry) {
	t.Helper()

	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	matrix, err := LoadCmdSmokeMatrix(projectRoot)
	if err != nil {
		t.Fatalf("LoadCmdSmokeMatrix: %v", err)
	}

	return projectRoot, matrix
}

func TestCmdSmokeMatrix_AllCmdCasesHaveDecisions(t *testing.T) {
	projectRoot, matrix := loadCmdSmokeMatrix(t)

	cases := collectAcceptanceTests(t, projectRoot, cmdAcceptanceTestFiles())
	for _, caseID := range cases {
		entry, ok := matrix[caseID]
		if !ok {
			t.Fatalf("cmd smoke matrix missing case %s", caseID)
		}
		if entry.Decision != "keep" && entry.Decision != "move" {
			t.Fatalf("%s has invalid decision %q", caseID, entry.Decision)
		}
		if entry.Rationale == "" || entry.Rationale == "-" {
			t.Fatalf("%s has empty rationale", caseID)
		}
	}

	for caseID := range matrix {
		found := false
		for _, known := range cases {
			if caseID == known {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("cmd smoke matrix includes unknown case %s", caseID)
		}
	}
}

func TestCmdSmokeMatrix_MovedCasesHaveConcreteUnitDestinations(t *testing.T) {
	projectRoot, matrix := loadCmdSmokeMatrix(t)
	unitTests := listCmdUnitTests(t, projectRoot)

	for caseID, entry := range matrix {
		if entry.Decision != "move" {
			continue
		}
		if entry.Destination == "" || entry.Destination == "-" {
			t.Fatalf("%s has empty move destination", caseID)
		}
		parts := strings.Split(entry.Destination, ":")
		if len(parts) != 2 {
			t.Fatalf("%s destination %q must be file:suite", caseID, entry.Destination)
		}
		if !strings.HasSuffix(parts[0], "_test.go") {
			t.Fatalf("%s destination file must be *_test.go, got %q", caseID, parts[0])
		}
		if !strings.Contains(unitTests, parts[1]) {
			t.Fatalf("%s destination suite %s not found in cmd tests", caseID, parts[1])
		}
	}
}

func TestLoadCmdSmokeMatrix_IncludesKnownCase(t *testing.T) {
	_, matrix := loadCmdSmokeMatrix(t)

	entry, ok := matrix["cmd/gromit/debug_agent_acceptance_test.go:TestCmdSmoke_DebugAgentResolutionEndToEnd"]
	if !ok {
		t.Fatalf("expected known cmd smoke case to be present")
	}
	if entry.Decision != "keep" {
		t.Fatalf("expected known cmd smoke case to be keep, got %q", entry.Decision)
	}
}
