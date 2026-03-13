package validator_test

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/specloop/stages"
	"github.com/danabrams/gromit/internal/next/validator"
)

// Compile-time interface check.
var _ stages.FinalValidator = (*validator.ShellValidator)(nil)

func TestShellValidator_NilRunnerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil runner, got none")
		}
	}()
	validator.NewShellValidator(nil)
}

func TestShellValidator_PassingChecks(t *testing.T) {
	sv := validator.NewShellValidator(validator.NewRunner())

	checks := []validator.Check{
		{Name: "echo", Command: "echo hello", Type: "always"},
	}
	result, err := sv.RunFinal(context.Background(), checks, nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Fatal("expected Pass=true for passing checks")
	}
	if len(result.AlwaysRun.Results) != 1 {
		t.Fatalf("expected 1 always-run result, got %d", len(result.AlwaysRun.Results))
	}
	if !result.AlwaysRun.Results[0].Pass {
		t.Fatal("expected always-run check to pass")
	}
}

func TestShellValidator_FailingCheck(t *testing.T) {
	sv := validator.NewShellValidator(validator.NewRunner())

	always := []validator.Check{
		{Name: "pass", Command: "echo ok", Type: "always"},
	}
	project := []validator.Check{
		{Name: "fail", Command: "exit 1", Type: "project"},
	}
	result, err := sv.RunFinal(context.Background(), always, project, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Fatal("expected Pass=false when a project check fails")
	}
	if !result.AlwaysRun.AllPass() {
		t.Fatal("expected always-run checks to pass")
	}
	if result.ProjectChecks.AllPass() {
		t.Fatal("expected project checks to have failures")
	}
}

func TestShellValidator_EmptyChecks(t *testing.T) {
	sv := validator.NewShellValidator(validator.NewRunner())

	result, err := sv.RunFinal(context.Background(), nil, nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Fatal("expected Pass=true for empty checks")
	}
}
