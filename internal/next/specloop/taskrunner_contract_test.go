//go:build llmcontract

package specloop

import (
	"context"
	"os"
	"testing"

	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// RunTaskRunnerContract runs the contract suite against any TaskRunner implementation.
func RunTaskRunnerContract(t *testing.T, runner TaskRunner) {
	t.Run("RunTask returns result with status", func(t *testing.T) {
		task := runstore.Task{
			TaskID:              "contract-test-1",
			Objective:           "Create a Go function that returns the string 'hello world'",
			ExpectedTouchedArea: []string{"main.go"},
			ProofChecks:         []string{"go build ./..."},
		}
		result, err := runner.RunTask(context.Background(), task)
		if err != nil {
			t.Fatalf("RunTask failed: %v", err)
		}
		validStatuses := map[string]bool{
			"done":        true,
			"failed":      true,
			"needs_split": true,
		}
		if !validStatuses[result.Status] {
			t.Errorf("expected a valid status (done, failed, needs_split), got %q", result.Status)
		}
		if result.TokensUsed == 0 {
			t.Error("expected non-zero TokensUsed")
		}
	})

	t.Run("RepairTask returns result with status", func(t *testing.T) {
		task := runstore.Task{
			TaskID:    "contract-test-2",
			Objective: "Fix the compilation error in the Add function",
		}
		failures := []string{"./main.go:5: undefined: fmt"}
		result, err := runner.RepairTask(context.Background(), task, failures)
		if err != nil {
			t.Fatalf("RepairTask failed: %v", err)
		}
		validStatuses := map[string]bool{
			"done":        true,
			"failed":      true,
			"needs_split": true,
		}
		if !validStatuses[result.Status] {
			t.Errorf("expected a valid status (done, failed, needs_split), got %q", result.Status)
		}
		if result.TokensUsed == 0 {
			t.Error("expected non-zero TokensUsed")
		}
	})
}

func TestContract_ProviderTaskRunner(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1 to run contract tests")
	}
	runner := buildRealTaskRunner(t)
	RunTaskRunnerContract(t, runner)
}

func buildRealTaskRunner(t *testing.T) TaskRunner {
	t.Helper()
	return NewProviderTaskRunner(llmadapter.ContractInvoker(t), func() string { return "" })
}

func TestContract_ProviderTaskRunner_Claude(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1 to run contract tests")
	}
	runner := buildRealTaskRunnerClaude(t)
	RunTaskRunnerContract(t, runner)
}

func buildRealTaskRunnerClaude(t *testing.T) TaskRunner {
	t.Helper()
	return NewProviderTaskRunner(llmadapter.ContractClaudeInvoker(t), func() string { return "" })
}

func TestContract_ProviderTaskRunner_Codex(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1 to run contract tests")
	}
	runner := buildRealTaskRunnerCodex(t)
	RunTaskRunnerContract(t, runner)
}

func buildRealTaskRunnerCodex(t *testing.T) TaskRunner {
	t.Helper()
	return NewProviderTaskRunner(llmadapter.ContractCodexInvoker(t), func() string { return "" })
}
