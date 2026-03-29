package contract

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_GoTestPassAssertionPassesOnGreenTest(t *testing.T) {
	// Seed
	workDir := t.TempDir()
	store := runstore.NewStore(filepath.Join(workDir, ".store"))
	rs := runstore.NewRunState("run-001", "proj-001")
	rs.SpecID = "spec-001"
	rs.Status = runstore.StatusRunning
	rs.StartedAt = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run state: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte("module example.com/scenario\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	pkgDir := filepath.Join(workDir, "pkg", "foo")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir pkg dir: %v", err)
	}
	testSrc := `package foo

import "testing"

func TestScenario_Foo(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "foo_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatalf("write foo_test.go: %v", err)
	}

	contract := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "go-test-pass-scenario",
			Assertions: []ContractAssertion{{
				GoTestPass: &GoTestPassAssertion{
					Pkg:      "./pkg/...",
					TestName: "TestScenario_Foo",
				},
			}},
		}},
	}
	evaluator := &DefaultContractEvaluator{}

	// Invoke
	failures, err := evaluator.Evaluate(context.Background(), contract, workDir)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	// Assert
	if len(failures) != 0 {
		t.Fatalf("expected no contract failures for passing go_test_pass assertion, got %d: %+v", len(failures), failures)
	}
}
