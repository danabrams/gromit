package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// TestScenario_HealthCheckPassesOnSecondCycle verifies that the worktree health
// check runs on every cycle — it is not gated by any "already checked" flag.
// Given cycle 1 completed successfully with a healthy worktree, when the validate
// stage runs on cycle 2 (after a replan triggered by code failures), the health
// check runs again, passes, and validation proceeds normally.
func TestScenario_HealthCheckPassesOnSecondCycle(t *testing.T) {
	dir := t.TempDir()

	// Seed: a healthy worktree with both .git and go.mod present
	healthyWorktree := filepath.Join(dir, ".gromit-next", "worktrees", "wt-abc123")
	if err := os.MkdirAll(healthyWorktree, 0o755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(healthyWorktree, ".git"), []byte("gitdir: /fake"), 0o644); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(healthyWorktree, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("create go.mod: %v", err)
	}

	// Seed: event log to verify health check does not emit recovery events
	eventLogPath := filepath.Join(dir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	// GitOps is provided but should NOT be called for recovery since worktree is healthy
	fakeGit := &validateScenarioFakeGitOps{}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: true,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "go test ./...", Pass: true}},
			},
			ProjectChecks: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "lint", Pass: true}},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: "/tmp/work",
		RepoDir: dir,
	}, eventLog, nil, fakeGit)

	// Seed: RunState simulating cycle 2 after a prior successful cycle 1
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.WorktreePath = healthyWorktree
	rs.SpecID = "spec-001"
	rs.RunID = "run-001"
	rs.Cycle = 2

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: validation proceeds normally (Continue, not Blocked)
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v (blocker: %s)", action.Kind, rs.BlockerSummary)
	}

	// Assert: final validation passed
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true")
	}

	// Assert: no recovery was attempted (worktree was healthy)
	if fakeGit.removeCalled {
		t.Error("RemoveWorktree should not be called for a healthy worktree")
	}
	if fakeGit.recoverCalled {
		t.Error("RecoverWorktree should not be called for a healthy worktree")
	}

	// Assert: no worktree_recovery event emitted (health check passed)
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll events: %v", err)
	}
	for _, ev := range events {
		if _, ok := ev.(*runstore.WorktreeRecoveryEvent); ok {
			t.Error("unexpected worktree_recovery event — health check should have passed")
		}
	}

	// Assert: final_validation_result event emitted with Passed=true
	var foundValidation bool
	for _, ev := range events {
		if fvr, ok := ev.(*runstore.FinalValidationResultEvent); ok {
			foundValidation = true
			if !fvr.Passed {
				t.Error("expected final_validation_result Passed=true")
			}
		}
	}
	if !foundValidation {
		t.Error("expected final_validation_result event to be emitted")
	}

	// Assert: BlockerSummary is empty (no infrastructure issue)
	if rs.BlockerSummary != "" {
		t.Errorf("expected empty BlockerSummary, got %q", rs.BlockerSummary)
	}
}
