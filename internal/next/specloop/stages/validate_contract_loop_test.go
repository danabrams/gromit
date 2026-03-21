package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// --- helpers ---

// setupContractLoopWorktree creates a minimal healthy worktree and evidence dir
// with a scenario-contracts.yaml file. Returns (worktreePath, evidenceDir).
func setupContractLoopWorktree(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()

	wt := filepath.Join(dir, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod: %v", err)
	}

	evDir := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}
	contractYAML := `scenarios:
- name: spec picker
  assertions:
  - file_contains:
      path: cmd/gromit-next/exec_test.go
      pattern: feature/foo
`
	if err := os.WriteFile(filepath.Join(evDir, "scenario-contracts.yaml"), []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract: %v", err)
	}

	return wt, evDir
}

func contractFailure(scenario, details string) contract.ContractFailure {
	return contract.ContractFailure{
		ScenarioName:  scenario,
		AssertionType: "file_contains",
		Details:       details,
		Assertion: contract.ContractAssertion{
			FileContains: &contract.FileContainsAssertion{
				Path:    "cmd/gromit-next/exec_test.go",
				Pattern: "feature/foo",
			},
		},
	}
}

// --- tests ---

func TestContractLoop_SameFailuresConsecutiveCycles_NeedsHuman(t *testing.T) {
	wt, evDir := setupContractLoopWorktree(t)

	failureStr := `contract:spec picker — file_contains failed: pattern "feature/foo" not found in "cmd/gromit-next/exec_test.go"`

	eval := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			contractFailure("spec picker", `pattern "feature/foo" not found in "cmd/gromit-next/exec_test.go"`),
		},
	}
	v := &fakeValidator{result: validator.FinalResult{Pass: true}}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     wt,
		EvidenceDir: evDir,
	}, nil, eval, &validateScenarioFakeGitOps{})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = wt
	rs.Cycle = 2
	// Simulate previous cycle storing the same failure
	rs.LastContractFailures = []string{failureStr}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.NeedsHuman {
		t.Fatalf("expected NeedsHuman, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) < 2 {
		t.Fatalf("expected at least 2 entries in failures (header + failure), got %d", len(action.Context.Failures))
	}
	if !strings.Contains(action.Context.Failures[0], "repeated contract failures") {
		t.Errorf("expected header mentioning repeated contract failures, got %q", action.Context.Failures[0])
	}
	// LastContractFailures should be unchanged (not updated since we short-circuited)
	if len(rs.LastContractFailures) != 1 || rs.LastContractFailures[0] != failureStr {
		t.Errorf("LastContractFailures should be unchanged, got %v", rs.LastContractFailures)
	}
}

func TestContractLoop_DifferentFailuresSecondCycle_Replans(t *testing.T) {
	wt, evDir := setupContractLoopWorktree(t)

	eval := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			contractFailure("spec picker", `pattern "feature/bar" not found in "cmd/gromit-next/exec_test.go"`),
		},
	}
	v := &fakeValidator{result: validator.FinalResult{
		Pass: false,
		AlwaysRun: validator.CheckResults{
			Results: []validator.CheckResult{{Name: "test", Pass: true}},
		},
	}}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     wt,
		EvidenceDir: evDir,
	}, nil, eval, &validateScenarioFakeGitOps{})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = wt
	rs.Cycle = 2
	rs.LastContractFailures = []string{
		`contract:spec picker — file_contains failed: pattern "feature/foo" not found in "cmd/gromit-next/exec_test.go"`,
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	// LastContractFailures should be updated to the new failure
	if len(rs.LastContractFailures) != 1 {
		t.Fatalf("expected 1 LastContractFailure, got %d", len(rs.LastContractFailures))
	}
	if !strings.Contains(rs.LastContractFailures[0], "feature/bar") {
		t.Errorf("expected updated failure with feature/bar, got %q", rs.LastContractFailures[0])
	}
}

func TestContractLoop_FailuresResolve_PassesAndClearsLastFailures(t *testing.T) {
	wt, evDir := setupContractLoopWorktree(t)

	// Evaluator returns no failures (contract resolved)
	eval := &fakeContractEvaluator{failures: nil}
	v := &fakeValidator{result: validator.FinalResult{Pass: true}}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     wt,
		EvidenceDir: evDir,
	}, nil, eval, &validateScenarioFakeGitOps{})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = wt
	rs.Cycle = 2
	rs.LastContractFailures = []string{
		`contract:spec picker — file_contains failed: pattern "feature/foo" not found in "cmd/gromit-next/exec_test.go"`,
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true")
	}
	// LastContractFailures should be cleared (empty slice from formatting loop)
	if len(rs.LastContractFailures) != 0 {
		t.Errorf("expected LastContractFailures to be empty, got %v", rs.LastContractFailures)
	}
}

func TestContractLoop_FirstCycleFailure_Replans(t *testing.T) {
	wt, evDir := setupContractLoopWorktree(t)

	eval := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			contractFailure("spec picker", `pattern "feature/foo" not found in "cmd/gromit-next/exec_test.go"`),
		},
	}
	v := &fakeValidator{result: validator.FinalResult{
		Pass: false,
		AlwaysRun: validator.CheckResults{
			Results: []validator.CheckResult{{Name: "test", Pass: true}},
		},
	}}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     wt,
		EvidenceDir: evDir,
	}, nil, eval, &validateScenarioFakeGitOps{})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = wt
	rs.Cycle = 1
	// LastContractFailures is empty on first cycle
	rs.LastContractFailures = []string{}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	// LastContractFailures should now be set
	if len(rs.LastContractFailures) != 1 {
		t.Fatalf("expected 1 LastContractFailure, got %d", len(rs.LastContractFailures))
	}
}

func TestContractLoop_ShellFailuresOnly_NoLoopDetection(t *testing.T) {
	wt, evDir := setupContractLoopWorktree(t)

	// No contract failures
	eval := &fakeContractEvaluator{failures: nil}
	// Shell check fails
	v := &fakeValidator{result: validator.FinalResult{
		Pass: false,
		AlwaysRun: validator.CheckResults{
			Results: []validator.CheckResult{{Name: "test", Pass: false, Output: "FAIL"}},
		},
	}}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     wt,
		EvidenceDir: evDir,
	}, nil, eval, &validateScenarioFakeGitOps{})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = wt
	rs.Cycle = 2
	// Previous cycle had same-ish shell failures but no contract failures
	rs.LastContractFailures = []string{}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should replan, not NeedsHuman — shell failures are not tracked for loop detection
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom (shell failures don't trigger loop detection), got %v", action.Kind)
	}
}

// --- slicesEqual unit tests ---

func TestSlicesEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want bool
	}{
		{"both empty", []string{}, []string{}, true},
		{"both nil", nil, nil, true},
		{"identical single", []string{"a"}, []string{"a"}, true},
		{"identical multi", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different length", []string{"a"}, []string{"a", "b"}, false},
		{"different content", []string{"a"}, []string{"b"}, false},
		{"different order", []string{"a", "b"}, []string{"b", "a"}, false},
		{"nil vs empty", nil, []string{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slicesEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("slicesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
