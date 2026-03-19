package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// TestScenario_ExecSpec_ExplicitResumeID_BypassesPicker verifies that when
// --resume=run-abc1234567890abc is passed explicitly, the run is resumed
// directly without showing any picker UI. The run ID must appear in the
// output and the resumed run's tasks must be preserved.
func TestScenario_ExecSpec_ExplicitResumeID_BypassesPicker(t *testing.T) {
	tmp := t.TempDir()

	// Seed: create a prior run with a specific 16-hex-char ID
	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "gromit")
	// Overwrite the auto-generated RunID with our explicit one
	prior.RunID = "run-abc1234567890abc"
	prior.Status = runstore.StatusBlocked
	prior.TerminalReason = "stage_needs_human"
	prior.EndedAt = time.Now()
	prior.WorktreePath = "/tmp/fake-worktree"
	prior.Tasks = []runstore.Task{
		{TaskID: "task-1", Objective: "implement feature", Status: "done"},
		{TaskID: "task-2", Objective: "write tests", Status: "failed"},
	}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	// Invoke: pass --resume with the explicit run ID
	var order []string
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorder{name: "plan", orderPtr: &order},
			&stageRecorder{name: "execute", orderPtr: &order},
			&stageRecorder{name: "validate", orderPtr: &order},
		},
	}

	cmd := newExecSpecCmdWithProvider(provider)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"--spec", "my-spec.md",
		"--project", "gromit",
		"--store-dir", tmp,
		"--resume=run-abc1234567890abc",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Assert: the exact run ID is used (no picker involved)
	if !strings.Contains(output, "run-abc1234567890abc") {
		t.Errorf("expected output to contain explicit run ID run-abc1234567890abc, got: %s", output)
	}

	// Assert: no picker UI was shown (no numbered list)
	if strings.Contains(output, "1.") && strings.Contains(output, "Select") {
		t.Errorf("expected no picker UI when explicit resume ID provided, got: %s", output)
	}

	// Assert: stages actually ran (pipeline executed, not blocked by picker)
	if len(order) == 0 {
		t.Fatal("expected stages to run when resuming with explicit ID")
	}

	// Assert: tasks from the prior run are preserved
	loaded, err := store.Get("run-abc1234567890abc")
	if err != nil {
		t.Fatalf("load resumed run: %v", err)
	}
	if len(loaded.Tasks) != 2 {
		t.Fatalf("expected 2 tasks preserved from prior run, got %d", len(loaded.Tasks))
	}
	if loaded.Tasks[0].TaskID != "task-1" {
		t.Errorf("expected first task to be task-1, got %s", loaded.Tasks[0].TaskID)
	}

	// Assert: run was marked as resumed
	if !loaded.Resumed {
		t.Error("expected Resumed flag to be true")
	}

	// Assert: status was reset to running (then finalized by pipeline)
	// The pipeline completed, so status should reflect the final state
	if loaded.Status == runstore.StatusBlocked {
		t.Error("expected status to be reset from blocked during resume")
	}

	// Assert: cycle was incremented
	if loaded.Cycle < 1 {
		t.Errorf("expected cycle to be incremented on resume, got %d", loaded.Cycle)
	}
}
