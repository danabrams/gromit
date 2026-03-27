package main

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// --- Contract Tests ---

func TestResumeContract_ResumedRunPreservesCompletedTasks(t *testing.T) {
	tmp := t.TempDir()

	// Create a prior run with 3 tasks: 2 done, 1 pending
	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusNeedsHuman
	prior.TerminalReason = "cycles_exhausted"
	prior.EndedAt = time.Now()
	prior.WorktreePath = "/tmp/wt"
	prior.Tasks = []runstore.Task{
		{TaskID: "t-001", Objective: "first task", Status: "done"},
		{TaskID: "t-002", Objective: "second task", Status: "done"},
		{TaskID: "t-003", Objective: "third task", Status: "pending"},
	}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	// Track which tasks are seen by the execute stage
	var capturedTasks []runstore.Task
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "execute",
				fn: func(rs *runstore.RunState) {
					capturedTasks = append(capturedTasks, rs.Tasks...)
				},
			},
		},
	}

	r := &execSpecRun{
		specPath:      "my-spec.md",
		projectID:     "my-proj",
		resumeRunID:   prior.RunID,
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 3 tasks should be present in the resumed state
	if len(capturedTasks) != 3 {
		t.Fatalf("expected 3 tasks preserved, got %d", len(capturedTasks))
	}

	// The 2 done tasks must still be done
	for _, task := range capturedTasks {
		switch task.TaskID {
		case "t-001", "t-002":
			if task.Status != "done" {
				t.Errorf("task %s should still be done, got %s", task.TaskID, task.Status)
			}
		case "t-003":
			if task.Status != "pending" {
				t.Errorf("task %s should still be pending before execution, got %s", task.TaskID, task.Status)
			}
		}
	}
}

func TestResumeContract_ResumedRunReusesWorktree(t *testing.T) {
	tmp := t.TempDir()

	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusNeedsHuman
	prior.EndedAt = time.Now()
	prior.WorktreePath = "/tmp/original-worktree"
	prior.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done"},
	}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	var capturedWT string
	var stagesRun []string
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "init",
				fn: func(rs *runstore.RunState) {
					stagesRun = append(stagesRun, "init")
				},
			},
			&stageRecorderFunc{
				name: "compile",
				fn: func(rs *runstore.RunState) {
					stagesRun = append(stagesRun, "compile")
				},
			},
			&stageRecorderFunc{
				name: "write_contracts",
				fn: func(rs *runstore.RunState) {
					stagesRun = append(stagesRun, "write_contracts")
				},
			},
			&stageRecorderFunc{
				name: "execute",
				fn: func(rs *runstore.RunState) {
					stagesRun = append(stagesRun, "execute")
					capturedWT = rs.WorktreePath
				},
			},
		},
	}

	r := &execSpecRun{
		specPath:      "my-spec.md",
		projectID:     "my-proj",
		resumeRunID:   prior.RunID,
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Same worktree path should be reused
	if capturedWT != "/tmp/original-worktree" {
		t.Errorf("expected worktree %q, got %q", "/tmp/original-worktree", capturedWT)
	}

	// init and compile must NOT have run (filtered out on resume)
	// write_contracts should run (relies on ContractsWritten flag for idempotency)
	for _, name := range stagesRun {
		if name == "init" {
			t.Error("init stage should not run on resume")
		}
		if name == "compile" {
			t.Error("compile stage should not run on resume")
		}
	}
}

func TestResumeContract_CyclesOverridesBudget(t *testing.T) {
	tmp := t.TempDir()

	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusNeedsHuman
	prior.EndedAt = time.Now()
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	// Use a stage that captures the budget's MaxCycles via the RunState
	var capturedMaxCycles int
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "execute",
				fn:   func(_ *runstore.RunState) {},
			},
		},
	}

	// Override BuildStages to capture the budget
	budgetCapture := &budgetCapturingProvider{
		inner:    provider,
		captured: &capturedMaxCycles,
	}

	r := &execSpecRun{
		specPath:      "my-spec.md",
		projectID:     "my-proj",
		resumeRunID:   prior.RunID,
		resumeCycles:  5,
		storeDir:      tmp,
		stageProvider: budgetCapture,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedMaxCycles != 5 {
		t.Errorf("expected MaxCycles to be 5 (from --cycles), got %d", capturedMaxCycles)
	}
}

// budgetCapturingProvider wraps a StageProvider and captures the budget's MaxCycles.
type budgetCapturingProvider struct {
	inner    StageProvider
	captured *int
}

func (p *budgetCapturingProvider) BuildStages(policy execpolicy.Policy, rs *runstore.RunState, budget *specloop.Budget, eventLog *runstore.EventLog) ([]specloop.Stage, error) {
	*p.captured = budget.MaxCycles()
	return p.inner.BuildStages(policy, rs, budget, eventLog)
}

func TestResumeContract_ResumedRunIncludesPlanStage(t *testing.T) {
	tmp := t.TempDir()

	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusNeedsHuman
	prior.EndedAt = time.Now()
	prior.WorktreePath = "/tmp/wt"
	prior.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done"},
	}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	var stagesRun []string
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "compile",
				fn:   func(_ *runstore.RunState) { stagesRun = append(stagesRun, "compile") },
			},
			&stageRecorderFunc{
				name: "plan",
				fn:   func(_ *runstore.RunState) { stagesRun = append(stagesRun, "plan") },
			},
			&stageRecorderFunc{
				name: "write_contracts",
				fn:   func(_ *runstore.RunState) { stagesRun = append(stagesRun, "write_contracts") },
			},
			&stageRecorderFunc{
				name: "execute",
				fn:   func(_ *runstore.RunState) { stagesRun = append(stagesRun, "execute") },
			},
		},
	}

	r := &execSpecRun{
		specPath:      "my-spec.md",
		projectID:     "my-proj",
		resumeRunID:   prior.RunID,
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Plan stage must be present in the filtered stages (needed for replan)
	planRan := false
	for _, name := range stagesRun {
		if name == "plan" {
			planRan = true
		}
	}
	if !planRan {
		t.Error("plan stage should be included on resume (needed for replan cycles)")
	}

	// compile must not have run (filtered out on resume)
	// write_contracts should run (relies on ContractsWritten flag for idempotency)
	for _, name := range stagesRun {
		if name == "compile" {
			t.Error("compile should not run on resume")
		}
	}
}

func TestResumeContract_GateFlagsResetOnResume(t *testing.T) {
	tmp := t.TempDir()

	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusNeedsHuman
	prior.EndedAt = time.Now()
	// All gates passed in the prior run
	prior.FinalValidationPassed = true
	prior.FinalReviewPassed = true
	prior.FinalAcceptancePassed = true
	prior.ReviewFindings = []string{"prior-finding"}
	prior.AcceptanceResults = []string{"prior-result"}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	var capturedRS *runstore.RunState
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "execute",
				fn: func(rs *runstore.RunState) {
					capturedRS = rs
				},
			},
		},
	}

	r := &execSpecRun{
		specPath:      "my-spec.md",
		projectID:     "my-proj",
		resumeRunID:   prior.RunID,
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedRS == nil {
		t.Fatal("stage did not capture RunState")
	}
	if capturedRS.FinalValidationPassed {
		t.Error("FinalValidationPassed should be false after resume")
	}
	if capturedRS.FinalReviewPassed {
		t.Error("FinalReviewPassed should be false after resume")
	}
	if capturedRS.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should be false after resume")
	}
	if len(capturedRS.ReviewFindings) != 0 {
		t.Errorf("ReviewFindings should be empty after resume, got %v", capturedRS.ReviewFindings)
	}
	if len(capturedRS.AcceptanceResults) != 0 {
		t.Errorf("AcceptanceResults should be empty after resume, got %v", capturedRS.AcceptanceResults)
	}
}

// TestResumeContract_PreservesBaselineFailuresAcrossResume tests via execSpecRun.run directly.
// See also TestExecSpec_ResumePreservesBaselineFailures in resume_test.go
// which covers the same behavior via the Cobra command path.
func TestResumeContract_PreservesBaselineFailuresAcrossResume(t *testing.T) {
	tmp := t.TempDir()

	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusNeedsHuman
	prior.EndedAt = time.Now()
	prior.WorktreePath = "/tmp/baseline-worktree"
	prior.BaselineFailures = map[string]string{"unit-tests": "baseline fail"}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	var captured map[string]string
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "execute",
				fn: func(rs *runstore.RunState) {
					captured = rs.BaselineFailures
				},
			},
		},
	}

	r := &execSpecRun{
		specPath:      "my-spec.md",
		projectID:     "my-proj",
		resumeRunID:   prior.RunID,
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured == nil {
		t.Fatal("expected baseline failures to be present when resuming")
	}
	if got := captured["unit-tests"]; got != "baseline fail" {
		t.Errorf("baseline failure output mismatch: got %q", got)
	}

	loaded, err := store.Get(prior.RunID)
	if err != nil {
		t.Fatalf("load resumed run: %v", err)
	}
	if got := loaded.BaselineFailures["unit-tests"]; got != "baseline fail" {
		t.Errorf("expected persisted baseline failure, got %q", got)
	}
	if loaded.FinalValidationPassed {
		t.Error("FinalValidationPassed should be false after resume")
	}
}

// --- Scenario Tests ---

func TestResumeScenario_HumanSaysKeepGoing(t *testing.T) {
	tmp := t.TempDir()

	// Simulate: prior run hit cycles_exhausted after 3 cycles
	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusNeedsHuman
	prior.TerminalReason = "cycles_exhausted"
	prior.EndedAt = time.Now()
	prior.Cycle = 3
	prior.WorktreePath = "/tmp/wt"
	prior.Tasks = []runstore.Task{
		{TaskID: "t-001", Objective: "setup config", Status: "done"},
		{TaskID: "t-002", Objective: "write handler", Status: "done"},
		{TaskID: "t-003", Objective: "integration tests", Status: "pending"},
	}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	// Track how many times each task ID is seen by the execute stage
	executedTaskIDs := map[string]int{}
	cyclesSeen := 0

	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "plan",
				fn:   func(_ *runstore.RunState) {},
			},
			&stageRecorderFunc{
				name: "execute",
				fn: func(rs *runstore.RunState) {
					cyclesSeen++
					for _, task := range rs.Tasks {
						if task.Status == "pending" {
							executedTaskIDs[task.TaskID]++
						}
					}
				},
			},
			&stageRecorderFunc{
				name: "validate",
				fn:   func(_ *runstore.RunState) {},
			},
		},
	}

	// Resume with --cycles 3 (3 fresh cycles)
	r := &execSpecRun{
		specPath:      "my-spec.md",
		projectID:     "my-proj",
		resumeRunID:   prior.RunID,
		resumeCycles:  3,
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the budget was set to 3 cycles
	loaded, err := store.Get(prior.RunID)
	if err != nil {
		t.Fatalf("load resumed run: %v", err)
	}

	// The run should complete (pipeline runs through stages)
	// Completed tasks from prior run should remain done
	for _, task := range loaded.Tasks {
		if task.TaskID == "t-001" || task.TaskID == "t-002" {
			if task.Status != "done" {
				t.Errorf("task %s should still be done, got %s", task.TaskID, task.Status)
			}
		}
	}

	// Execute stage should have seen the tasks at least once
	if cyclesSeen == 0 {
		t.Error("execute stage should have run at least once")
	}

	// t-003 is the only pending task, so only it should appear in executedTaskIDs
	if _, ok := executedTaskIDs["t-003"]; !ok {
		t.Error("pending task t-003 should have been seen by execute stage")
	}
	// Done tasks should NOT appear as pending
	if _, ok := executedTaskIDs["t-001"]; ok {
		t.Error("done task t-001 should not appear as pending")
	}
	if _, ok := executedTaskIDs["t-002"]; ok {
		t.Error("done task t-002 should not appear as pending")
	}
}

func TestResumeScenario_ResumeAfterBlockedTask(t *testing.T) {
	tmp := t.TempDir()

	// Prior run: t-001 and t-002 done, t-003 failed (blocked)
	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusBlocked
	prior.TerminalReason = "task_failed"
	prior.EndedAt = time.Now()
	prior.WorktreePath = "/tmp/wt"
	prior.Tasks = []runstore.Task{
		{TaskID: "t-001", Objective: "setup config", Status: "done"},
		{TaskID: "t-002", Objective: "write handler", Status: "done"},
		{TaskID: "t-003", Objective: "integration tests", Status: "failed"},
	}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	// Custom execute stage that records which task IDs have non-done status
	pendingTaskIDs := []string{}
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "plan",
				fn:   func(_ *runstore.RunState) {},
			},
			&stageRecorderFunc{
				name: "execute",
				fn: func(rs *runstore.RunState) {
					for _, task := range rs.Tasks {
						if task.Status != "done" {
							pendingTaskIDs = append(pendingTaskIDs, task.TaskID)
						}
					}
				},
			},
			&stageRecorderFunc{
				name: "validate",
				fn:   func(_ *runstore.RunState) {},
			},
		},
	}

	r := &execSpecRun{
		specPath:      "my-spec.md",
		projectID:     "my-proj",
		resumeRunID:   prior.RunID,
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}

	if err := r.run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only t-003 should be non-done (the execute stage sees it as failed/pending)
	if len(pendingTaskIDs) != 1 {
		t.Fatalf("expected 1 non-done task for execute, got %d: %v", len(pendingTaskIDs), pendingTaskIDs)
	}
	if pendingTaskIDs[0] != "t-003" {
		t.Errorf("expected t-003 as the non-done task, got %s", pendingTaskIDs[0])
	}

	// Verify t-001 and t-002 status unchanged in persisted state
	loaded, err := store.Get(prior.RunID)
	if err != nil {
		t.Fatalf("load resumed run: %v", err)
	}
	for _, task := range loaded.Tasks {
		switch task.TaskID {
		case "t-001":
			if task.Status != "done" {
				t.Errorf("t-001 status should be done, got %s", task.Status)
			}
		case "t-002":
			if task.Status != "done" {
				t.Errorf("t-002 status should be done, got %s", task.Status)
			}
		case "t-003":
			// t-003 was failed; the execute stage sees it but doesn't change status
			// in this test (no real task runner). Status preserved as-is.
			if task.Status != "failed" {
				t.Errorf("t-003 status should still be failed (no task runner in test), got %s", task.Status)
			}
		}
	}
}
