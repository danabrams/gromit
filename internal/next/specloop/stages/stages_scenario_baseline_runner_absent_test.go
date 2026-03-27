package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/testutil"
	"github.com/danabrams/gromit/internal/next/validator"
)

func TestScenario_BaselineRunnerAbsent_RunProceedsNormally(t *testing.T) {
	// Seed
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)
	specPath, policyPath := writeScenarioFixtures(t, storeDir)
	policyJSONPath := filepath.Join(storeDir, "policy.json")
	if err := os.WriteFile(policyJSONPath, []byte(`{"budgets":{"max_spec_cycles":3}}`), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}

	rs := runstore.NewRunState("spec-scenario-baseline-absent", "project-scenario")
	if err := store.Save(rs); err != nil {
		t.Fatalf("seed save run state: %v", err)
	}

	eventLogPath := filepath.Join(store.RunDir(rs.RunID), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)
	gitOps := &fakeGitOps{worktreePath: filepath.Join(t.TempDir(), "scenario-worktree")}
	testutil.WriteMinimalProjectFixtures(t, gitOps.worktreePath)
	if err := os.WriteFile(filepath.Join(gitOps.worktreePath, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitOps.worktreePath, "go.mod"), []byte("module scenario\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	initStage := NewInitStage(InitStageConfig{
		SpecPath:   specPath,
		PolicyPath: policyPath,
		RepoDir:    storeDir,
		GitOps:     gitOps,
	}, store, eventLog)

	// Invoke
	initAction, err := initStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("init run: %v", err)
	}
	validateStage := NewValidateStage(&fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{Name: "unit-tests", Pass: false, Output: "FAIL: new regression"},
				},
			},
			ProjectChecks: validator.CheckResults{},
		},
	}, ValidateStageConfig{WorkDir: gitOps.worktreePath}, eventLog, nil, nil)

	validateAction, err := validateStage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("validate run: %v", err)
	}

	// Assert
	if initAction.Kind != specloop.Continue {
		t.Fatalf("expected Continue from init, got %v", initAction.Kind)
	}
	if len(rs.BaselineFailures) != 0 {
		t.Fatalf("BaselineFailures len = %d, want 0", len(rs.BaselineFailures))
	}
	if validateAction.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom from validate, got %v", validateAction.Kind)
	}
	if validateAction.Context == nil || len(validateAction.Context.Failures) == 0 {
		t.Fatal("expected validation failures")
	}
	if !strings.Contains(validateAction.Context.Failures[0], "unit-tests") {
		t.Fatalf("expected executor-introduced failure mentioning unit-tests, got %q", validateAction.Context.Failures[0])
	}

	rawEvents, err := os.ReadFile(eventLogPath)
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	if strings.Contains(string(rawEvents), "baseline_captured") {
		t.Fatal("baseline_captured event should not be emitted when baseline runner is absent")
	}
}

func writeScenarioFixtures(t testing.TB, projectDir string) (specPath, policyPath string) {
	t.Helper()
	fixtures := testutil.WriteMinimalProjectFixtures(t, projectDir)

	specsDir := filepath.Join(projectDir, "docs", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs dir: %v", err)
	}
	specPath = filepath.Join(specsDir, "spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	return specPath, fixtures.PolicyPath
}
