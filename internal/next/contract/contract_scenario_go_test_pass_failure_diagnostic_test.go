package contract

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_GoTestPassAssertionFailsWithDiagnosticOutput(t *testing.T) {
	// Seed
	workDir := t.TempDir()
	store := runstore.NewStore(filepath.Join(workDir, ".store"))
	rs := runstore.NewRunState("run-002", "proj-001")
	rs.SpecID = "spec-001"
	rs.Status = runstore.StatusRunning
	rs.StartedAt = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run state: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte("module example.com/scenario\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	pkgDir := filepath.Join(workDir, "pkg", "bar")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir pkg dir: %v", err)
	}

	testSrc := `package bar

import "testing"

func TestScenario_Bar(t *testing.T) {
	t.Fatalf("expected 'Reviewer Instructions' section, got empty prompt")
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "bar_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatalf("write bar_test.go: %v", err)
	}

	contract := &ScenarioContract{
		Scenarios: []ScenarioAssertions{{
			Name: "go-test-pass-diagnostic",
			Assertions: []ContractAssertion{{
				GoTestPass: &GoTestPassAssertion{
					Pkg:      "./pkg/...",
					TestName: "TestScenario_Bar",
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
	if len(failures) != 1 {
		t.Fatalf("expected 1 contract failure, got %d: %+v", len(failures), failures)
	}

	f := failures[0]
	if f.AssertionType != "go_test_pass" {
		t.Fatalf("expected assertion type go_test_pass, got %q", f.AssertionType)
	}
	if !strings.Contains(f.Details, "expected 'Reviewer Instructions' section, got empty prompt") {
		t.Fatalf("expected go test diagnostic output in failure details, got: %q", f.Details)
	}
}
