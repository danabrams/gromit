package validator

import (
	"context"
	"testing"
)

func TestRunFinal_CombinesAlwaysRunAndProjectChecks(t *testing.T) {
	r := NewRunner()
	alwaysRun := []Check{{Name: "vet", Command: "true", Type: "lint"}}
	projectChecks := []Check{{Name: "integration", Command: "true", Type: "test"}}

	result, err := r.RunFinal(context.Background(), alwaysRun, projectChecks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pass {
		t.Fatal("expected pass")
	}
	if result.AlwaysRun.PassCount() != 1 || result.ProjectChecks.PassCount() != 1 {
		t.Fatal("unexpected counts")
	}
}

func TestRunFinal_ProjectChecksFail_OverallFails(t *testing.T) {
	r := NewRunner()
	alwaysRun := []Check{{Name: "vet", Command: "true", Type: "lint"}}
	projectChecks := []Check{{Name: "integration", Command: "false", Type: "test"}}

	result, err := r.RunFinal(context.Background(), alwaysRun, projectChecks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if result.Pass {
		t.Fatal("expected fail when project checks fail")
	}
}

func TestRunFinal_AlwaysRunFails_OverallFails(t *testing.T) {
	r := NewRunner()
	alwaysRun := []Check{{Name: "vet", Command: "false", Type: "lint"}}
	projectChecks := []Check{{Name: "integration", Command: "true", Type: "test"}}

	result, err := r.RunFinal(context.Background(), alwaysRun, projectChecks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if result.Pass {
		t.Fatal("expected fail when always-run checks fail")
	}
}
