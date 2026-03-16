//go:build llmcontract

package planner

import (
	"context"
	"os"
	"testing"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// RunPlanAgentContract runs the agent contract suite against any Agent implementation.
func RunPlanAgentContract(t *testing.T, agent Agent) {
	t.Run("returns parseable plan", func(t *testing.T) {
		prompt := buildPlanPrompt(PlanRequest{
			SpecPacket: "Implement a function that adds two numbers and returns the result.",
			Cycle:      1,
		})
		result, err := agent.Invoke(context.Background(), prompt, "medium")
		if err != nil {
			t.Fatalf("agent invocation failed: %v", err)
		}
		if result.Output == "" {
			t.Fatal("expected non-empty output")
		}
		plan, err := ParsePlan(result.Output)
		if err != nil {
			t.Fatalf("output not parseable as Plan: %v", err)
		}
		if len(plan.Tasks) == 0 {
			t.Error("plan should have at least one task")
		}
		for _, task := range plan.Tasks {
			if task.TaskID == "" {
				t.Error("task missing task_id")
			}
			if task.Objective == "" {
				t.Error("task missing objective")
			}
		}
	})

	t.Run("returns token counts", func(t *testing.T) {
		result, err := agent.Invoke(context.Background(), "Generate a plan for: add logging", "low")
		if err != nil {
			t.Fatalf("agent invocation failed: %v", err)
		}
		if result.TokensIn == 0 {
			t.Error("expected non-zero TokensIn")
		}
		if result.TokensOut == 0 {
			t.Error("expected non-zero TokensOut")
		}
		if result.Cost == 0 {
			t.Error("expected non-zero Cost (check provider pricing config)")
		}
	})
}

func TestContract_ProviderPlanAgent(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1 to run contract tests")
	}
	agent := buildRealPlanAgent(t)
	RunPlanAgentContract(t, agent)
}

func buildRealPlanAgent(t *testing.T) Agent {
	t.Helper()
	return NewProviderPlanAgent(llmadapter.ContractInvoker(t), "sonnet")
}

func TestContract_ProviderPlanAgent_Claude(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1 to run contract tests")
	}
	agent := buildRealPlanAgentClaude(t)
	RunPlanAgentContract(t, agent)
}

func buildRealPlanAgentClaude(t *testing.T) Agent {
	t.Helper()
	return NewProviderPlanAgent(llmadapter.ContractClaudeInvoker(t), "sonnet")
}

func TestContract_ProviderPlanAgent_Codex(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1 to run contract tests")
	}
	agent := buildRealPlanAgentCodex(t)
	RunPlanAgentContract(t, agent)
}

func buildRealPlanAgentCodex(t *testing.T) Agent {
	t.Helper()
	return NewProviderPlanAgent(llmadapter.ContractCodexInvoker(t), "sonnet")
}
