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

	stage := NewFinalizeStage(gitOps, store, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = true
	rs.FinalReviewPassed = true
	rs.FinalAcceptancePassed = true
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

	stage := NewFinalizeStage(gitOps, store, nil)

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

	stage := NewFinalizeStage(gitOps, store, nil)

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

	stage := NewFinalizeStage(gitOps, store, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = true
	rs.FinalReviewPassed = true
	rs.FinalAcceptancePassed = true
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

	stage := NewFinalizeStage(gitOps, store, nil)

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

func TestFinalizeStage_PreservesWorktreeForBlockedRuns(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStage(gitOps, store, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Status = runstore.StatusBlocked
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	rs.WorktreePath = "/tmp/worktree"

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gitOps.removedPath != "" {
		t.Fatal("worktree should be preserved for blocked runs (cleanup happens in InitStage)")
	}
	if rs.WorktreePath != "/tmp/worktree" {
		t.Fatalf("worktree_path should be preserved, got %q", rs.WorktreePath)
	}
}

func TestFinalizeStage_ReadyForReview_RequiresAllGates(t *testing.T) {
	tests := []struct {
		name       string
		rs         *runstore.RunState
		wantStatus string
	}{
		{
			name: "all pass -> ready_for_review",
			rs: &runstore.RunState{
				Tasks:                 []runstore.Task{{TaskID: "t1", Status: "done"}},
				FinalValidationPassed: true,
				FinalReviewPassed:     true,
				FinalAcceptancePassed: true,
			},
			wantStatus: runstore.StatusReadyForReview,
		},
		{
			name: "review failed -> needs_human",
			rs: &runstore.RunState{
				Tasks:                 []runstore.Task{{TaskID: "t1", Status: "done"}},
				FinalValidationPassed: true,
				FinalReviewPassed:     false,
				FinalAcceptancePassed: true,
			},
			wantStatus: runstore.StatusNeedsHuman,
		},
		{
			name: "acceptance failed -> needs_human",
			rs: &runstore.RunState{
				Tasks:                 []runstore.Task{{TaskID: "t1", Status: "done"}},
				FinalValidationPassed: true,
				FinalReviewPassed:     true,
				FinalAcceptancePassed: false,
			},
			wantStatus: runstore.StatusNeedsHuman,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeStore := runstore.NewStore(t.TempDir())
			stage := NewFinalizeStage(nil, fakeStore, nil)
			_, err := stage.Run(context.Background(), tt.rs)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if tt.rs.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", tt.rs.Status, tt.wantStatus)
			}
		})
	}
}

func TestFinalizeStage_PreservesWorktreeForBlocked(t *testing.T) {
	fakeStore := runstore.NewStore(t.TempDir())
	removeCalled := false
	fakeGit := &fakeGitOps{
		removeErr: nil,
	}
	// Override to track calls
	origRemove := fakeGit.removedPath
	stage := NewFinalizeStage(fakeGit, fakeStore, nil)
	rs := &runstore.RunState{Status: runstore.StatusBlocked, WorktreePath: "/tmp/test-worktree"}

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = origRemove
	removeCalled = fakeGit.removedPath != ""
	if removeCalled {
		t.Error("FinalizeStage should NOT remove worktree for blocked runs")
	}
}

func TestFinalizeStage_RecordsWorktreePathInRunJSON(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStage(gitOps, store, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = true
	rs.FinalReviewPassed = true
	rs.FinalAcceptancePassed = true
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
