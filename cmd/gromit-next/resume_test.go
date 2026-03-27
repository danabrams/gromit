package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestFilterStagesForResume(t *testing.T) {
	allNames := []string{"init", "compile", "plan", "write_contracts", "execute", "validate", "review", "accept", "evidence"}
	var allStages []specloop.Stage
	for _, name := range allNames {
		allStages = append(allStages, &stageRecorder{name: name})
	}

	t.Run("always skips compile", func(t *testing.T) {
		filtered := filterStagesForResume(allStages, "")
		for _, s := range filtered {
			if s.Name() == "compile" {
				t.Fatal("compile should always be skipped on resume")
			}
		}
	})

	t.Run("keeps write_contracts (runs idempotently via ContractsWritten flag)", func(t *testing.T) {
		filtered := filterStagesForResume(allStages, "")
		found := false
		for _, s := range filtered {
			if s.Name() == "write_contracts" {
				found = true
			}
		}
		if !found {
			t.Fatal("write_contracts should be kept (relies on ContractsWritten flag for idempotency)")
		}
	})

	t.Run("keeps init when no WorktreePath", func(t *testing.T) {
		filtered := filterStagesForResume(allStages, "")
		found := false
		for _, s := range filtered {
			if s.Name() == "init" {
				found = true
			}
		}
		if !found {
			t.Fatal("init should be kept on resume when no WorktreePath is set")
		}
	})

	t.Run("init should be skipped when WorktreePath is set", func(t *testing.T) {
		filtered := filterStagesForResume(allStages, "/tmp/existing-worktree")
		for _, s := range filtered {
			if s.Name() == "init" {
				t.Fatal("init should be skipped when WorktreePath is set")
			}
		}
	})

	t.Run("keeps plan in stage list for replan jumps", func(t *testing.T) {
		filtered := filterStagesForResume(allStages, "")
		found := false
		for _, s := range filtered {
			if s.Name() == "plan" {
				found = true
			}
		}
		if !found {
			t.Fatal("plan should be kept in stage list (it no-ops when tasks exist and no replan context)")
		}
	})

	t.Run("keeps execute validate review accept evidence", func(t *testing.T) {
		filtered := filterStagesForResume(allStages, "")
		names := make(map[string]bool)
		for _, s := range filtered {
			names[s.Name()] = true
		}
		for _, want := range []string{"execute", "validate", "review", "accept", "evidence"} {
			if !names[want] {
				t.Errorf("expected %s stage to be kept", want)
			}
		}
	})
}

func TestExecSpec_ResumeLoadsExistingRunState(t *testing.T) {
	tmp := t.TempDir()

	// Create a prior run with tasks
	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusBlocked
	prior.TerminalReason = "stage_needs_human"
	prior.EndedAt = time.Now()
	prior.WorktreePath = "/tmp/fake-worktree"
	prior.Tasks = []runstore.Task{
		{TaskID: "task-1", Objective: "do something", Status: "done"},
		{TaskID: "task-2", Objective: "do another", Status: "failed"},
	}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	var order []string
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorder{name: "init", orderPtr: &order},
			&stageRecorder{name: "compile", orderPtr: &order},
			&stageRecorder{name: "plan", orderPtr: &order},
			&stageRecorder{name: "write_contracts", orderPtr: &order},
			&stageRecorder{name: "execute", orderPtr: &order},
			&stageRecorder{name: "validate", orderPtr: &order},
		},
	}

	cmd := newExecSpecCmdWithProvider(provider)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"--spec", "my-spec.md",
		"--project", "my-proj",
		"--store-dir", tmp,
		"--resume=" + prior.RunID,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, prior.RunID) {
		t.Errorf("expected output to contain original run ID %s, got: %s", prior.RunID, output)
	}

	// Verify tasks were preserved by loading persisted state
	loaded, err := store.Get(prior.RunID)
	if err != nil {
		t.Fatalf("load resumed run: %v", err)
	}
	if len(loaded.Tasks) != 2 {
		t.Fatalf("expected 2 tasks preserved, got %d", len(loaded.Tasks))
	}
	if loaded.Tasks[0].TaskID != "task-1" {
		t.Errorf("expected first task to be task-1, got %s", loaded.Tasks[0].TaskID)
	}
}

func TestExecSpec_ResumeSkipsCompile(t *testing.T) {
	tmp := t.TempDir()

	// Create a prior run with tasks and worktree
	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusBlocked
	prior.EndedAt = time.Now()
	prior.WorktreePath = "/tmp/fake-worktree"
	prior.Tasks = []runstore.Task{
		{TaskID: "task-1", Status: "done"},
	}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	var order []string
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorder{name: "init", orderPtr: &order},
			&stageRecorder{name: "compile", orderPtr: &order},
			&stageRecorder{name: "plan", orderPtr: &order},
			&stageRecorder{name: "write_contracts", orderPtr: &order},
			&stageRecorder{name: "execute", orderPtr: &order},
			&stageRecorder{name: "validate", orderPtr: &order},
		},
	}

	cmd := newExecSpecCmdWithProvider(provider)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"--spec", "my-spec.md",
		"--project", "my-proj",
		"--store-dir", tmp,
		"--resume=" + prior.RunID,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// compile and init should be skipped (WorktreePath is set)
	for _, skipped := range []string{"compile", "init"} {
		for _, ran := range order {
			if ran == skipped {
				t.Errorf("stage %s should have been skipped on resume", skipped)
			}
		}
	}

	// plan, write_contracts, execute, validate should run (init skipped because WorktreePath is set)
	want := []string{"plan", "write_contracts", "execute", "validate"}
	if len(order) != len(want) {
		t.Fatalf("expected %d stages, got %d: %v", len(want), len(order), order)
	}
	for i, name := range want {
		if order[i] != name {
			t.Errorf("stage %d: want %s, got %s", i, name, order[i])
		}
	}
}

func TestExecSpec_ResumeResetsGateFlags(t *testing.T) {
	tmp := t.TempDir()

	// Create a prior run with gate flags set
	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusNeedsHuman
	prior.TerminalReason = "stage_needs_human"
	prior.EndedAt = time.Now()
	prior.FinalValidationPassed = true
	prior.FinalReviewPassed = true
	prior.FinalAcceptancePassed = true
	prior.ReviewFindings = []string{"finding-1"}
	prior.AcceptanceResults = []string{"result-1"}
	prior.Cycle = 2
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	// Use a stage that captures the RunState for inspection
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
		t.Error("FinalValidationPassed should be reset to false")
	}
	if capturedRS.FinalReviewPassed {
		t.Error("FinalReviewPassed should be reset to false")
	}
	if capturedRS.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should be reset to false")
	}
	if len(capturedRS.ReviewFindings) != 0 {
		t.Errorf("ReviewFindings should be empty, got %v", capturedRS.ReviewFindings)
	}
	if len(capturedRS.AcceptanceResults) != 0 {
		t.Errorf("AcceptanceResults should be empty, got %v", capturedRS.AcceptanceResults)
	}
	if capturedRS.Status != runstore.StatusRunning {
		t.Errorf("Status should be running, got %s", capturedRS.Status)
	}
	if capturedRS.TerminalReason != "" {
		t.Errorf("TerminalReason should be empty, got %s", capturedRS.TerminalReason)
	}
	if !capturedRS.EndedAt.IsZero() {
		t.Errorf("EndedAt should be zero, got %v", capturedRS.EndedAt)
	}
}

func TestExecSpec_ResumePreservesWorktreePath(t *testing.T) {
	tmp := t.TempDir()

	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("my-spec", "my-proj")
	prior.Status = runstore.StatusBlocked
	prior.EndedAt = time.Now()
	prior.WorktreePath = "/tmp/my-worktree-path"
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	var capturedWT string
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "execute",
				fn: func(rs *runstore.RunState) {
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

	if capturedWT != "/tmp/my-worktree-path" {
		t.Errorf("WorktreePath should be preserved, got %s", capturedWT)
	}
}

// TestExecSpec_ResumePreservesBaselineFailures tests the Cobra command path.
// See also TestResumeContract_PreservesBaselineFailuresAcrossResume in resume_contract_test.go
// which covers the same behavior via execSpecRun.run directly.
func TestExecSpec_ResumePreservesBaselineFailures(t *testing.T) {
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

	cmd := newExecSpecCmdWithProvider(provider)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"--project", "my-proj",
		"--store-dir", tmp,
		"--resume=" + prior.RunID,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured == nil {
		t.Fatal("expected baseline failures to be present on resume")
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
}

func TestExecSpec_ResumeErrorOnMissingRunID(t *testing.T) {
	tmp := t.TempDir()

	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorder{name: "execute"},
		},
	}

	r := &execSpecRun{
		specPath:      "my-spec.md",
		projectID:     "my-proj",
		resumeRunID:   "run-nonexistent",
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         runstore.NewStore(tmp),
		out:           io.Discard,
	}

	err := r.run(context.Background())
	if err == nil {
		t.Fatal("expected error when resuming nonexistent run")
	}
	if !strings.Contains(err.Error(), "load run for resume") {
		t.Errorf("expected 'load run for resume' error, got: %v", err)
	}
}

// --- helpers ---

// stageRecorderFunc is a stage that calls a function with the RunState.
type stageRecorderFunc struct {
	name string
	fn   func(rs *runstore.RunState)
}

func (s *stageRecorderFunc) Name() string { return s.name }
func (s *stageRecorderFunc) Run(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	if s.fn != nil {
		s.fn(rs)
	}
	return specloop.NextAction{Kind: specloop.Continue}, nil
}

func ptrPolicy(p execpolicy.Policy) *execpolicy.Policy {
	return &p
}
