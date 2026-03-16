//go:build e2e

package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/danabrams/gromit/e2e"
)

const (
	contractsDir = "/Users/dabrams/gromit/contracts"
	fixtureBase  = "/tmp/gromit-fixtures"
)

// TestScenarioContracts runs all scenario contracts found in the contracts/ directory.
//
// Usage:
//
//	GROMIT_E2E=1 go test ./e2e/ -tags e2e -count=1 -timeout 30m
//
// Run a single scenario:
//
//	GROMIT_E2E=1 go test ./e2e/ -tags e2e -count=1 -timeout 30m -run TestScenarioContracts/Scenario01
func TestScenarioContracts(t *testing.T) {
	e2e.RequireE2E(t)

	binary := e2e.BuildBinary(t)
	e2e.SetBinaryPath(binary)

	contracts := e2e.LoadContracts(t, contractsDir)
	for _, c := range contracts {
		c := c
		t.Run(fmt.Sprintf("Scenario%02d_%s", c.Scenario, e2e.Slug(c.Name)), func(t *testing.T) {
			// Run serially by default (cost control).
			// Individual contracts can set Concurrent: true to opt into t.Parallel().
			if c.Concurrent {
				t.Parallel()
			}
			e2e.RunContract(t, c, binary, fixtureBase)
		})
	}
}

// Individual test functions for selective execution.

func TestE2E_Scenario01_HappyPath(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 1, contractsDir, fixtureBase)
}

func TestE2E_Scenario02_UnfixableSpec(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 2, contractsDir, fixtureBase)
}

func TestE2E_Scenario03_BudgetExhaustion(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 3, contractsDir, fixtureBase)
}

func TestE2E_Scenario04_UnfixableConflict(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 4, contractsDir, fixtureBase)
}

func TestE2E_Scenario05_DryRun(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 5, contractsDir, fixtureBase)
}

func TestE2E_Scenario09_CostLimit(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 9, contractsDir, fixtureBase)
}

func TestE2E_Scenario10_Timeout(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 10, contractsDir, fixtureBase)
}

func TestE2E_Scenario11_CLIInspection(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 11, contractsDir, fixtureBase)
}

func TestE2E_Scenario13_ReviewAcceptanceHappyPath(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 13, contractsDir, fixtureBase)
}

func TestE2E_Scenario14_ReviewTriggeredFixCycle(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 14, contractsDir, fixtureBase)
}

func TestE2E_Scenario15_ConfigurableThreshold(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 15, contractsDir, fixtureBase)
}

func TestE2E_Scenario16_AcceptanceFailTriggersFixCycle(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 16, contractsDir, fixtureBase)
}

func TestE2E_Scenario17_AcceptanceUnclearAddsEvidence(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 17, contractsDir, fixtureBase)
}

func TestE2E_Scenario07_AcceptanceUnclearExhaustsBudget(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 7, contractsDir, fixtureBase)
}

func TestE2E_Scenario18_LogicGapsFacet(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 18, contractsDir, fixtureBase)
}

func TestE2E_Scenario19_NewVsPreexistingFinding(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 19, contractsDir, fixtureBase)
}

func TestE2E_Scenario20_MissingAcceptanceCriteria(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 20, contractsDir, fixtureBase)
}

func TestE2E_Scenario21_BlockedWorktreeCleanup(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 21, contractsDir, fixtureBase)
}

func TestE2E_Scenario22_ProviderIdentification(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 22, contractsDir, fixtureBase)
}

func TestE2E_Scenario23_AdapterWiringVerification(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 23, contractsDir, fixtureBase)
}

func TestE2E_Scenario24_RouterPhasePreferences(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 24, contractsDir, fixtureBase)
}

func TestE2E_Scenario06_TaskRepair(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 6, contractsDir, fixtureBase)
}

func TestE2E_Scenario07_TaskSplit(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunContractByFile(t, contractsDir, "scenario-07-task-split.yaml", fixtureBase)
}

func TestE2E_Scenario08_MultiProjectIsolation(t *testing.T) {
	e2e.RequireE2E(t)
	binary := e2e.BuildBinary(t)
	e2e.SetBinaryPath(binary)

	// Calc contract: add-subtract.md on fixture-calc
	calcContract := e2e.Contract{
		Name:     "Scenario 8 calc — Multi-Project Isolation",
		Scenario: 8,
		Spec:     "specs/add-subtract.md",
		Fixture:  "fixture-calc",
		Policy:   "policies/fixture-calc-execution.json",
		StoreDir: ".gromit-next",
		FixtureReset: e2e.FixtureReset{
			GitFiles: []e2e.GitFileRestore{
				{Commit: "7f6de76", Files: []string{"calc/calc.go", "calc/calc_test.go"}},
			},
			RemoveFiles: []string{"calc/divide_test.go", "calc/divide_edge_test.go", "calc/divide_exact_test.go"},
		},
		Assertions: []e2e.Assertion{
			{Status: "ready_for_review"},
			{FinalValidationPassed: boolPtr(true)},
			{CostUSDGt: float64Ptr(0)},
			{EndedAtSet: boolPtr(true)},
			{FilesChangedNonempty: boolPtr(true)},
			{AnyTaskFilesChangedContains: "calc/calc.go"},
			{FileContains: &e2e.FileContainsAssertion{Path: "calc/calc.go", Pattern: "func Subtract"}},
			{EventsContainType: "task_validation_result"},
			{ExecShowFullNotContains: "running"},
			{ExecListContains: "ready_for_review"},
		},
	}

	// Greeter contract: add-farewell.md on fixture-greeter
	greeterContract := e2e.Contract{
		Name:         "Scenario 8 greeter — Multi-Project Isolation",
		Scenario:     8,
		Spec:         "add-farewell.md",
		Fixture:      "fixture-greeter",
		Policy:       "policies/fixture-greeter-execution.json",
		StoreDir:     ".gromit-next",
		FixtureReset: e2e.FixtureReset{},
		Assertions: []e2e.Assertion{
			{Status: "ready_for_review"},
			{FinalValidationPassed: boolPtr(true)},
			{CostUSDGt: float64Ptr(0)},
			{EndedAtSet: boolPtr(true)},
			{FilesChangedNonempty: boolPtr(true)},
			{EventsContainType: "task_validation_result"},
			{ExecShowFullNotContains: "running"},
			{ExecListContains: "ready_for_review"},
		},
	}

	// Run both concurrently.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		e2e.RunContract(t, calcContract, binary, fixtureBase)
	}()

	go func() {
		defer wg.Done()
		e2e.RunContract(t, greeterContract, binary, fixtureBase)
	}()

	wg.Wait()

	// Cross-contamination checks: scan evidence files in each store dir
	// and ensure each run's evidence does not reference the other project.
	calcStoreDir := filepath.Join(fixtureBase, "fixture-calc", ".gromit-next")
	greeterStoreDir := filepath.Join(fixtureBase, "fixture-greeter", ".gromit-next")

	checkDirNotContains(t, "calc evidence", calcStoreDir, []string{"farewell", "greeter"})
	checkDirNotContains(t, "greeter evidence", greeterStoreDir, []string{"subtract", "calculator"})
}

// checkDirNotContains walks dir and fails the test if any file contains any of
// the forbidden strings.
func checkDirNotContains(t *testing.T, label, dir string, forbidden []string) {
	t.Helper()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := strings.ToLower(string(data))
		for _, word := range forbidden {
			if strings.Contains(content, strings.ToLower(word)) {
				t.Errorf("cross-contamination: %s file %s contains %q", label, path, word)
			}
		}
		return nil
	})
	if err != nil {
		t.Errorf("cross-contamination walk %s: %v", label, err)
	}
}

func TestE2E_Scenario12_BroadRefactor(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 12, contractsDir, fixtureBase)
}

func TestE2E_Scenario25_CostCallback(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 25, contractsDir, fixtureBase)
}

func TestE2E_Scenario26_SingleProviderMode(t *testing.T) {
	e2e.SetBinaryPath(e2e.BuildBinary(t))
	e2e.RunNamedContract(t, 26, contractsDir, fixtureBase)
}

func boolPtr(b bool) *bool          { return &b }
func float64Ptr(f float64) *float64 { return &f }
