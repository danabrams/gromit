//go:build llmcontract

package validator_test

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/specloop/stages"
	"github.com/danabrams/gromit/internal/next/validator"
)

// RunShellValidatorContract runs the contract suite against any stages.FinalValidator.
// ShellValidator is not LLM-based, so no GROMIT_LLM_CONTRACT gate is needed.
func RunShellValidatorContract(t *testing.T, v stages.FinalValidator) {
	t.Run("passing checks produce pass result", func(t *testing.T) {
		checks := []validator.Check{
			{Name: "echo", Command: "echo hello", Type: "always"},
		}
		result, err := v.RunFinal(context.Background(), checks, nil, t.TempDir())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Pass {
			t.Fatal("expected Pass=true for passing checks")
		}
	})

	t.Run("failing checks produce failure with details", func(t *testing.T) {
		always := []validator.Check{
			{Name: "pass", Command: "echo ok", Type: "always"},
		}
		project := []validator.Check{
			{Name: "fail", Command: "exit 1", Type: "project"},
		}
		result, err := v.RunFinal(context.Background(), always, project, t.TempDir())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Pass {
			t.Fatal("expected Pass=false when a project check fails")
		}
		if !result.AlwaysRun.AllPass() {
			t.Fatal("expected always-run checks to still pass")
		}
		if result.ProjectChecks.AllPass() {
			t.Fatal("expected project checks to have failures")
		}
	})

	t.Run("empty checks produce pass result", func(t *testing.T) {
		result, err := v.RunFinal(context.Background(), nil, nil, t.TempDir())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Pass {
			t.Fatal("expected Pass=true for empty checks")
		}
	})
}

func TestContract_ShellValidator(t *testing.T) {
	sv := validator.NewShellValidator(validator.NewRunner())
	RunShellValidatorContract(t, sv)
}
