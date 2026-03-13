//go:build llmcontract

package specloop

import (
	"os"
	"testing"
)

func TestIntegration_HappyPath(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1")
	}
	t.Skip("TODO: implement happy path scenario")
}

func TestIntegration_ValidationFailureTriggersRepair(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1")
	}
	t.Skip("TODO: implement validation failure scenario")
}

func TestIntegration_ReviewTriggersReplan(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1")
	}
	t.Skip("TODO: implement review replan scenario")
}

func TestIntegration_BudgetExhaustion(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1")
	}
	t.Skip("TODO: implement budget exhaustion scenario")
}
