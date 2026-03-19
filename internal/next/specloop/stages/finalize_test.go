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

func TestFinalizeStage_SetsNeedsHumanWhenReviewFailed(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStage(gitOps, store, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = true
	// FinalReviewPassed and FinalAcceptancePassed default to false
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

func TestFinalizeStage_AllGatesPassedWithFailedTask_ReadyForReview(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStage(gitOps, store, nil)

	// Simulates a multi-cycle run: t-001 failed, fix tasks t-002 and t-003 succeeded.
	// All three gates pass in the final cycle, so status should be ready_for_review.
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = true
	rs.FinalReviewPassed = true
	rs.FinalAcceptancePassed = true
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "failed"},
		{TaskID: "t-002", Status: "done"},
		{TaskID: "t-003", Status: "done"},
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

func TestFinalizeStage_WritesReviewPacket(t *testing.T) {
	tmp := t.TempDir()
	evidenceDir := filepath.Join(tmp, "evidence")

	// Create evidence directory with test data
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	// Write validation.json
	validationData := map[string]interface{}{
		"passed": true,
		"checks": 5,
	}
	writeTestJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// Write review.json
	reviewData := map[string]interface{}{
		"blocking": []interface{}{},
		"findings": []interface{}{},
	}
	writeTestJSON(t, filepath.Join(evidenceDir, "review.json"), reviewData)

	// Write acceptance.json
	acceptanceData := map[string]interface{}{
		"passed": 3,
		"failed": 0,
		"unclear": 0,
	}
	writeTestJSON(t, filepath.Join(evidenceDir, "acceptance.json"), acceptanceData)

	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStageWithConfig(gitOps, store, nil, &FinalizeStageConfig{
		SpecContent: "## Scenarios\n\n**Scenario:** Test scenario\n- Given X\n- When Y\n- Then Z",
		EvidenceDir: evidenceDir,
	})

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

	// Verify review packet artifacts were written
	productReviewPath := filepath.Join(evidenceDir, "product-review.json")
	if _, err := os.Stat(productReviewPath); os.IsNotExist(err) {
		t.Fatalf("product-review.json not written")
	}

	processReviewPath := filepath.Join(evidenceDir, "process-review.json")
	if _, err := os.Stat(processReviewPath); os.IsNotExist(err) {
		t.Fatalf("process-review.json not written")
	}

	manualChecklistPath := filepath.Join(evidenceDir, "manual-checklist.json")
	if _, err := os.Stat(manualChecklistPath); os.IsNotExist(err) {
		t.Fatalf("manual-checklist.json not written")
	}
}

func TestFinalizeStage_PacketGenerationFailure(t *testing.T) {
	tmp := t.TempDir()
	evidenceDir := filepath.Join(tmp, "nonexistent")

	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStageWithConfig(gitOps, store, nil, &FinalizeStageConfig{
		SpecContent: "## Test",
		EvidenceDir: evidenceDir,
	})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = true
	rs.FinalReviewPassed = true
	rs.FinalAcceptancePassed = true
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	rs.WorktreePath = "/tmp/worktree"

	// Should not error even if packet generation fails
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Status should still be set correctly
	if rs.Status != runstore.StatusReadyForReview {
		t.Fatalf("expected status ready_for_review, got %q", rs.Status)
	}

	// Continue action should still be returned
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
}

func TestFinalizeStage_DiagnosticPacket(t *testing.T) {
	tmp := t.TempDir()
	evidenceDir := filepath.Join(tmp, "evidence")

	// Create evidence directory with minimal data
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	validationData := map[string]interface{}{
		"passed": false,
		"checks": 0,
	}
	writeTestJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	reviewData := map[string]interface{}{
		"blocking": []interface{}{},
	}
	writeTestJSON(t, filepath.Join(evidenceDir, "review.json"), reviewData)

	acceptanceData := map[string]interface{}{
		"passed": 0,
		"failed": 1,
		"unclear": 0,
	}
	writeTestJSON(t, filepath.Join(evidenceDir, "acceptance.json"), acceptanceData)

	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStageWithConfig(gitOps, store, nil, &FinalizeStageConfig{
		SpecContent: "## Test",
		EvidenceDir: evidenceDir,
	})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.FinalValidationPassed = false
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	rs.WorktreePath = "/tmp/worktree"

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify diagnostic packet was written (IsDiagnostic = true)
	productReviewPath := filepath.Join(evidenceDir, "product-review.json")
	if _, err := os.Stat(productReviewPath); os.IsNotExist(err) {
		t.Fatalf("product-review.json not written for diagnostic run")
	}

	data, err := os.ReadFile(productReviewPath)
	if err != nil {
		t.Fatalf("read product-review.json: %v", err)
	}

	var productReview map[string]interface{}
	if err := json.Unmarshal(data, &productReview); err != nil {
		t.Fatalf("unmarshal product-review.json: %v", err)
	}

	isDiagnostic, ok := productReview["is_diagnostic"].(bool)
	if !ok || !isDiagnostic {
		t.Fatal("expected is_diagnostic to be true for needs_human run")
	}
}

func TestFinalizeStage_BlockedRunGeneratesReviewPacket(t *testing.T) {
	tmp := t.TempDir()
	evidenceDir := filepath.Join(tmp, "evidence")

	// Create evidence directory with minimal data
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	validationData := map[string]interface{}{
		"passed": false,
		"checks": 0,
	}
	writeTestJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	reviewData := map[string]interface{}{
		"blocking": []interface{}{},
	}
	writeTestJSON(t, filepath.Join(evidenceDir, "review.json"), reviewData)

	acceptanceData := map[string]interface{}{
		"passed": 0,
		"failed": 0,
		"unclear": 0,
	}
	writeTestJSON(t, filepath.Join(evidenceDir, "acceptance.json"), acceptanceData)

	store := runstore.NewStore(tmp)
	gitOps := &fakeGitOps{}

	stage := NewFinalizeStageWithConfig(gitOps, store, nil, &FinalizeStageConfig{
		SpecContent: "## Test blocked scenario",
		EvidenceDir: evidenceDir,
	})

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Status = runstore.StatusBlocked
	rs.TerminalReason = "dependency not met"
	rs.Tasks = []runstore.Task{{TaskID: "t-001", Status: "done"}}
	rs.WorktreePath = "/tmp/worktree"

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	if rs.Status != runstore.StatusBlocked {
		t.Fatalf("expected status blocked, got %q", rs.Status)
	}

	// Verify review packet artifacts were written for blocked run
	productReviewPath := filepath.Join(evidenceDir, "product-review.json")
	if _, err := os.Stat(productReviewPath); os.IsNotExist(err) {
		t.Fatalf("product-review.json not written for blocked run")
	}

	processReviewPath := filepath.Join(evidenceDir, "process-review.json")
	if _, err := os.Stat(processReviewPath); os.IsNotExist(err) {
		t.Fatalf("process-review.json not written for blocked run")
	}

	manualChecklistPath := filepath.Join(evidenceDir, "manual-checklist.json")
	if _, err := os.Stat(manualChecklistPath); os.IsNotExist(err) {
		t.Fatalf("manual-checklist.json not written for blocked run")
	}

	// Verify is_diagnostic is true for blocked run
	data, err := os.ReadFile(productReviewPath)
	if err != nil {
		t.Fatalf("read product-review.json: %v", err)
	}

	var productReview map[string]interface{}
	if err := json.Unmarshal(data, &productReview); err != nil {
		t.Fatalf("unmarshal product-review.json: %v", err)
	}

	isDiagnostic, ok := productReview["is_diagnostic"].(bool)
	if !ok || !isDiagnostic {
		t.Fatal("expected is_diagnostic to be true for blocked run")
	}
}

func writeTestJSON(t *testing.T, path string, v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
