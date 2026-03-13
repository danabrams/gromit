//go:build llmcontract

package acceptor

import (
	"context"
	"os"
	"testing"
)

// RunAcceptAgentContract runs the agent contract suite against any AcceptAgent implementation.
func RunAcceptAgentContract(t *testing.T, agent AcceptAgent) {
	t.Run("returns parseable criterion result for passing criterion", func(t *testing.T) {
		prompt := `Evaluate whether the following code meets this acceptance criterion:
Criterion: "The function adds two numbers and returns the result"

Code:
func Add(a, b int) int {
    return a + b
}

Return a JSON object with fields: criterion, status ("pass", "fail", or "unclear"), rationale, evidence_refs (array of strings).`

		result, err := agent.EvaluateCriterion(context.Background(), prompt)
		if err != nil {
			t.Fatalf("agent invocation failed: %v", err)
		}
		if result.Status != "pass" && result.Status != "fail" && result.Status != "unclear" {
			t.Errorf("unexpected status: %q (want pass/fail/unclear)", result.Status)
		}
	})

	t.Run("returns rationale on fail", func(t *testing.T) {
		prompt := `Evaluate whether the following code meets this acceptance criterion:
Criterion: "The function handles negative numbers by returning an error"

Code:
func Add(a, b int) int {
    return a + b
}

This code does NOT handle negative numbers or return errors. It should fail.
Return a JSON object with fields: criterion, status ("pass", "fail", or "unclear"), rationale, evidence_refs (array of strings).`

		result, err := agent.EvaluateCriterion(context.Background(), prompt)
		if err != nil {
			t.Fatalf("agent invocation failed: %v", err)
		}
		if result.Status == "pass" {
			t.Error("expected fail or unclear for unmet criterion")
		}
		if result.Rationale == "" {
			t.Error("expected non-empty rationale on fail/unclear")
		}
	})
}

func TestContract_ProviderAcceptAgent(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1 to run contract tests")
	}
	agent := buildRealAcceptAgent(t)
	RunAcceptAgentContract(t, agent)
}

func buildRealAcceptAgent(t *testing.T) AcceptAgent {
	t.Helper()
	t.Skip("TODO: wire real provider for contract tests")
	return nil
}
