package stages

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// Verify FinalizeStage satisfies the Stage interface.
var _ specloop.Stage = (*FinalizeStage)(nil)

func TestFinalizeStage_SetsReadyForReview(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStage(gitOps, store)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = true
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done"},
		{TaskID: "t-002", Status: "done"},
	}
	rs.WorktreePath = "/tmp/worktree"

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if rs.Status != runstore.StatusReadyForReview {
		t.Fatalf("expected status ready_for_review, got %q", rs.Status)
	}
}

func TestFinalizeStage_AllTasksDoneButValidationFailed_NeedsHuman(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStage(gitOps, store)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = false
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done"},
		{TaskID: "t-002", Status: "done"},
	}
	rs.WorktreePath = "/tmp/worktree"

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if rs.Status != runstore.StatusNeedsHuman {
		t.Fatalf("expected status needs_human, got %q", rs.Status)
	}
}

func TestFinalizeStage_SetsNeedsHumanWhenTasksFailed(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStage(gitOps, store)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = true
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done"},
		{TaskID: "t-002", Status: "failed"},
	}
	rs.WorktreePath = "/tmp/worktree"

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if rs.Status != runstore.StatusNeedsHuman {
		t.Fatalf("expected status needs_human, got %q", rs.Status)
	}
}

func TestFinalizeStage_PreservesWorktreeForReadyForReview(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStage(gitOps, store)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = true
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	rs.WorktreePath = "/tmp/worktree"

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gitOps.removedPath != "" {
		t.Fatal("worktree should not be removed for ready_for_review")
	}
}

func TestFinalizeStage_PreservesWorktreeForNeedsHuman(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStage(gitOps, store)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = false
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "failed"}}
	rs.WorktreePath = "/tmp/worktree"

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gitOps.removedPath != "" {
		t.Fatal("worktree should not be removed for needs_human")
	}
}

func TestFinalizeStage_CleansWorktreeForBlocked(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStage(gitOps, store)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Status = runstore.StatusBlocked
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	rs.WorktreePath = "/tmp/worktree"

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gitOps.removedPath != "/tmp/worktree" {
		t.Fatalf("expected worktree to be removed, got removedPath=%q", gitOps.removedPath)
	}
}

func TestFinalizeStage_RecordsWorktreePathInRunJSON(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStage(gitOps, store)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = true
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	rs.WorktreePath = "/tmp/my-worktree"

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back run.json and verify WorktreePath
	runJSONPath := filepath.Join(store.RunDir(rs.RunID), "run.json")
	data, err := os.ReadFile(runJSONPath)
	if err != nil {
		t.Fatalf("run.json not written: %v", err)
	}
	var saved runstore.RunState
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("unmarshal run.json: %v", err)
	}
	if saved.WorktreePath != "/tmp/my-worktree" {
		t.Fatalf("expected worktree_path '/tmp/my-worktree', got %q", saved.WorktreePath)
	}
}
