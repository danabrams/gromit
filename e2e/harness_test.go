//go:build e2e

package e2e_test

import (
	"fmt"
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
