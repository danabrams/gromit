package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type cmdSmokeMatrixEntry struct {
	Decision    string
	Rationale   string
	Destination string
}

var (
	cmdSmokeAnnotationRe = regexp.MustCompile(`^//\s*smoke-matrix:\s*(keep|move)\s*\|\s*rationale:\s*(.+?)\s*\|\s*destination:\s*(.+)\s*$`)
	cmdSmokeFuncRe       = regexp.MustCompile(`^func (Test[[:alnum:]_]+)\(`)
)

func parseCmdSmokeMatrixForFile(t *testing.T, projectRoot, rel string) (map[string]cmdSmokeMatrixEntry, []string) {
	t.Helper()

	path := filepath.Join(projectRoot, rel)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", rel, err)
	}
	t.Cleanup(func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Fatalf("close %s: %v", rel, closeErr)
		}
	})

	entries := make(map[string]cmdSmokeMatrixEntry)
	tests := make([]string, 0)
	var pending *cmdSmokeMatrixEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := cmdSmokeAnnotationRe.FindStringSubmatch(line); m != nil {
			pending = &cmdSmokeMatrixEntry{
				Decision:    strings.TrimSpace(m[1]),
				Rationale:   strings.TrimSpace(m[2]),
				Destination: strings.TrimSpace(m[3]),
			}
			continue
		}

		if m := cmdSmokeFuncRe.FindStringSubmatch(line); m != nil {
			testName := strings.TrimSpace(m[1])
			tests = append(tests, testName)
			if pending != nil {
				entries[testName] = *pending
			}
			pending = nil
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", rel, err)
	}

	return entries, tests
}

func listCmdUnitTests(t *testing.T, projectRoot string) map[string]struct{} {
	t.Helper()
	return packageTestNameIndex(t, projectRoot, "cmd/gromit")
}

func TestCmdSmokeCoverageMatrix_CaseDecisionsIncludeRationale(t *testing.T) {
	// Expected failure: SmokeCoverageMatrixEntry annotations and parseSmokeCoverageMatrixLine
	// conventions do not exist in cmd acceptance tests yet.
	projectRoot := loadProjectRoot(t)

	for _, rel := range cmdAcceptanceTestFiles() {
		entries, tests := parseCmdSmokeMatrixForFile(t, projectRoot, rel)
		for _, testName := range tests {
			entry, ok := entries[testName]
			if !ok {
				t.Fatalf("%s missing smoke-matrix mapping for %s", rel, testName)
			}
			if entry.Decision != "keep" && entry.Decision != "move" {
				t.Fatalf("%s has invalid decision %q for %s", rel, entry.Decision, testName)
			}
			if entry.Rationale == "" || entry.Rationale == "-" {
				t.Fatalf("%s has empty rationale for %s", rel, testName)
			}
		}
	}
}

func TestCmdSmokeCoverageMatrix_KeepSetIsHighValueE2E(t *testing.T) {
	// Expected failure: cmdSmokeCoverageMatrixFromAnnotations has not been applied,
	// so keep/move decisions for high-value E2E outcomes are not encoded yet.
	projectRoot := loadProjectRoot(t)

	expectedKeep := map[string]bool{
		"TestCmdSmoke_DebugAgentResolutionEndToEnd":  true,
		"TestCmdSmoke_ExploreAgentSelectionEndToEnd": true,
		"TestCmdSmoke_ReviewSpecValidationEndToEnd":  true,
	}

	actualKeep := make(map[string]bool)
	for _, rel := range cmdAcceptanceTestFiles() {
		entries, tests := parseCmdSmokeMatrixForFile(t, projectRoot, rel)
		for _, testName := range tests {
			entry, ok := entries[testName]
			if !ok {
				t.Fatalf("%s missing smoke-matrix mapping for %s", rel, testName)
			}
			if entry.Decision == "keep" {
				actualKeep[testName] = true
			}
		}
	}

	if len(actualKeep) != len(expectedKeep) {
		t.Fatalf("cmd keep set size=%d, want=%d (actual=%v)", len(actualKeep), len(expectedKeep), actualKeep)
	}
	for testName := range expectedKeep {
		if !actualKeep[testName] {
			t.Fatalf("cmd keep set missing high-value E2E case %s", testName)
		}
	}
}

func TestCmdSmokeCoverageMatrix_MovedCasesPointToConcreteUnitSuites(t *testing.T) {
	// Expected failure: MoveToUnitSuiteDestination mappings and moveCoverageDestinations
	// are not defined yet for cmd acceptance cases.
	projectRoot := loadProjectRoot(t)
	unitTests := listCmdUnitTests(t, projectRoot)

	expectedMovedDestinations := map[string]string{
		"TestDebugChooseAgentUsesPicker":     "cmd/gromit/debug_agent_test.go:TestDebugChooseAgentUsesPicker_Reclassified",
		"TestDebugPhaseConfigUsesAgent":      "cmd/gromit/debug_agent_test.go:TestDebugPhaseConfigUsesAgent_Reclassified",
		"TestExplorePhaseConfigSelectsAgent": "cmd/gromit/explore_agent_test.go:TestExplorePhaseConfigSelectsAgent_Reclassified",
	}

	for _, rel := range cmdAcceptanceTestFiles() {
		entries, tests := parseCmdSmokeMatrixForFile(t, projectRoot, rel)
		for _, testName := range tests {
			expectedDest, shouldMove := expectedMovedDestinations[testName]
			entry, ok := entries[testName]
			if !ok {
				t.Fatalf("%s missing smoke-matrix mapping for %s", rel, testName)
			}

			if shouldMove {
				if entry.Decision != "move" {
					t.Fatalf("%s should be move, got %q", testName, entry.Decision)
				}
				if entry.Destination != expectedDest {
					t.Fatalf("%s destination=%q, want %q", testName, entry.Destination, expectedDest)
				}
				parts := strings.Split(entry.Destination, ":")
				if len(parts) != 2 {
					t.Fatalf("%s destination %q must be file:suite", testName, entry.Destination)
				}
				if !strings.HasSuffix(parts[0], "_test.go") {
					t.Fatalf("%s destination file must be *_test.go, got %q", testName, parts[0])
				}
				if _, ok := unitTests[parts[1]]; !ok {
					t.Fatalf("cmd unit suite missing moved destination test %s", parts[1])
				}
				continue
			}

			if entry.Decision == "move" {
				t.Fatalf("%s marked move but no concrete destination expected", testName)
			}
		}
	}
}
