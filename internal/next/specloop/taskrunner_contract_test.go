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
		if result.Status == "" {
			t.Error("expected non-empty status")
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
		if result.Status == "" {
			t.Error("expected non-empty status")
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
	return NewProviderTaskRunner(llmadapter.ContractInvoker(t))
}
