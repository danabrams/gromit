package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/testutil"
	"github.com/danabrams/gromit/internal/next/validator"
)

// TestScenario_PreexistingFailureExcludedAfterResume verifies that when a run captures
// baseline failures before pausing mid-cycle, resuming the run keeps those failures
// excluded from validation so no replan is triggered.
func TestScenario_PreexistingFailureExcludedAfterResume(t *testing.T) {
	tmp := t.TempDir()
	testutil.WriteMinimalProjectFixtures(t, tmp)

	worktree := filepath.Join(tmp, ".gromit-next", "worktrees", "wt-resume")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "go.mod"), []byte("module resume\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	eventLog := runstore.NewEventLog(filepath.Join(tmp, "events.jsonl"))

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{Name: "unit-tests", Pass: false, Output: "baseline fail"},
				},
			},
			ProjectChecks: validator.CheckResults{},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: worktree,
		RepoDir: tmp,
	}, eventLog, nil, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = worktree
	rs.Resumed = true
	rs.Cycle = 2
	rs.BaselineFailures = map[string]string{"unit-tests": "baseline fail"}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true when all failures are baseline-excluded")
	}
	if got := rs.BaselineFailures["unit-tests"]; got != "baseline fail" {
		t.Fatalf("baseline failure output mutated: got %q, want baseline fail", got)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	found := false
	for _, ev := range events {
		if bfe, ok := ev.(*runstore.BaselineFailureExcludedEvent); ok {
			found = true
			if bfe.CheckName != "unit-tests" {
				t.Fatalf("baseline event check_name = %q, want unit-tests", bfe.CheckName)
			}
			if bfe.Output != "baseline fail" {
				t.Fatalf("baseline event output = %q, want baseline fail", bfe.Output)
			}
		}
	}
	if !found {
		t.Fatal("baseline_failure_excluded event not emitted")
	}
}
