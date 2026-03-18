package stages

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// TestRecoverWorktree_Success verifies that when both RemoveWorktree and
// RecoverWorktree succeed, rs.WorktreePath is updated and a success
// WorktreeRecoveryEvent is emitted.
func TestRecoverWorktree_Success(t *testing.T) {
	eventLogPath := filepath.Join(t.TempDir(), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	fakeGitOps := &validateScenarioFakeGitOps{
		recoveredPath: "/recovered/worktree",
	}

	stage := NewValidateStage(nil, ValidateStageConfig{
		RepoDir: "/repo",
	}, eventLog, nil, fakeGitOps)

	rs := runstore.NewRunState("spec-123", "run-456")
	rs.WorktreePath = "/old/worktree"
	rs.SpecID = "spec-123"
	rs.RunID = "run-456"

	healthErr := errors.New("go.mod not found")
	err := stage.recoverWorktree(context.Background(), rs, healthErr)

	// Verify no error returned
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify RemoveWorktree was called
	if !fakeGitOps.removeCalled {
		t.Fatal("expected RemoveWorktree to be called")
	}

	// Verify RecoverWorktree was called with correct branch name
	if !fakeGitOps.recoverCalled {
		t.Fatal("expected RecoverWorktree to be called")
	}
	expectedBranch := "gromit/spec-spec-123-run-456"
	if fakeGitOps.recoverBranch != expectedBranch {
		t.Fatalf("expected branch %q, got %q", expectedBranch, fakeGitOps.recoverBranch)
	}

	// Verify WorktreePath was updated
	if rs.WorktreePath != "/recovered/worktree" {
		t.Fatalf("expected WorktreePath to be updated to %q, got %q", "/recovered/worktree", rs.WorktreePath)
	}

	// Verify WorktreeRecoveryEvent was emitted with RecoverySucceeded=true
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	recoveryEvent, ok := events[0].(*runstore.WorktreeRecoveryEvent)
	if !ok {
		t.Fatalf("expected WorktreeRecoveryEvent, got %T", events[0])
	}

	if !recoveryEvent.RecoverySucceeded {
		t.Fatal("expected RecoverySucceeded to be true")
	}
	if recoveryEvent.HealthCheckFailure != "go.mod not found" {
		t.Fatalf("expected HealthCheckFailure to be %q, got %q", "go.mod not found", recoveryEvent.HealthCheckFailure)
	}
	if recoveryEvent.NewWorktreePath != "/recovered/worktree" {
		t.Fatalf("expected NewWorktreePath to be %q, got %q", "/recovered/worktree", recoveryEvent.NewWorktreePath)
	}
	if recoveryEvent.Type != "worktree_recovery" {
		t.Fatalf("expected event type 'worktree_recovery', got %q", recoveryEvent.Type)
	}
}

// TestRecoverWorktree_RemoveFails verifies that when RemoveWorktree fails,
// the function returns an error with "worktree cleanup failed" message,
// a failure WorktreeRecoveryEvent is emitted, and RecoverWorktree is not called.
func TestRecoverWorktree_RemoveFails(t *testing.T) {
	eventLogPath := filepath.Join(t.TempDir(), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	removeErr := errors.New("failed to remove directory")
	fakeGitOps := &validateScenarioFakeGitOps{
		removeErr: removeErr,
	}

	stage := NewValidateStage(nil, ValidateStageConfig{
		RepoDir: "/repo",
	}, eventLog, nil, fakeGitOps)

	rs := runstore.NewRunState("spec-789", "run-012")
	rs.WorktreePath = "/old/worktree"
	rs.SpecID = "spec-789"
	rs.RunID = "run-012"

	healthErr := errors.New(".git file missing")
	err := stage.recoverWorktree(context.Background(), rs, healthErr)

	// Verify error contains "worktree cleanup failed"
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errMsg := err.Error(); !strings.Contains(errMsg, "worktree cleanup failed") {
		t.Fatalf("expected error to contain 'worktree cleanup failed', got %q", errMsg)
	}

	// Verify RemoveWorktree was called
	if !fakeGitOps.removeCalled {
		t.Fatal("expected RemoveWorktree to be called")
	}

	// Verify RecoverWorktree was NOT called
	if fakeGitOps.recoverCalled {
		t.Fatal("expected RecoverWorktree to NOT be called when RemoveWorktree fails")
	}

	// Verify failure WorktreeRecoveryEvent was emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	recoveryEvent, ok := events[0].(*runstore.WorktreeRecoveryEvent)
	if !ok {
		t.Fatalf("expected WorktreeRecoveryEvent, got %T", events[0])
	}

	if recoveryEvent.RecoverySucceeded {
		t.Fatal("expected RecoverySucceeded to be false")
	}
	if recoveryEvent.HealthCheckFailure != ".git file missing" {
		t.Fatalf("expected HealthCheckFailure to be %q, got %q", ".git file missing", recoveryEvent.HealthCheckFailure)
	}
}

// TestRecoverWorktree_RecoverFails verifies that when RemoveWorktree succeeds
// but RecoverWorktree fails, the function returns an error, a failure
// WorktreeRecoveryEvent is emitted, and rs.WorktreePath is not updated.
func TestRecoverWorktree_RecoverFails(t *testing.T) {
	eventLogPath := filepath.Join(t.TempDir(), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	recoverErr := errors.New("failed to create new worktree")
	fakeGitOps := &validateScenarioFakeGitOps{
		recoverErr: recoverErr,
	}

	stage := NewValidateStage(nil, ValidateStageConfig{
		RepoDir: "/repo",
	}, eventLog, nil, fakeGitOps)

	rs := runstore.NewRunState("spec-345", "run-678")
	originalPath := "/old/worktree"
	rs.WorktreePath = originalPath
	rs.SpecID = "spec-345"
	rs.RunID = "run-678"

	healthErr := errors.New("directory does not exist")
	err := stage.recoverWorktree(context.Background(), rs, healthErr)

	// Verify error is returned (the RecoverWorktree error)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != recoverErr {
		t.Fatalf("expected error to be %v, got %v", recoverErr, err)
	}

	// Verify RemoveWorktree was called
	if !fakeGitOps.removeCalled {
		t.Fatal("expected RemoveWorktree to be called")
	}

	// Verify RecoverWorktree was called
	if !fakeGitOps.recoverCalled {
		t.Fatal("expected RecoverWorktree to be called")
	}

	// Verify WorktreePath was not updated
	if rs.WorktreePath != originalPath {
		t.Fatalf("expected WorktreePath to remain %q, got %q", originalPath, rs.WorktreePath)
	}

	// Verify failure WorktreeRecoveryEvent was emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	recoveryEvent, ok := events[0].(*runstore.WorktreeRecoveryEvent)
	if !ok {
		t.Fatalf("expected WorktreeRecoveryEvent, got %T", events[0])
	}

	if recoveryEvent.RecoverySucceeded {
		t.Fatal("expected RecoverySucceeded to be false")
	}
	if recoveryEvent.HealthCheckFailure != "directory does not exist" {
		t.Fatalf("expected HealthCheckFailure to be %q, got %q", "directory does not exist", recoveryEvent.HealthCheckFailure)
	}
	if recoveryEvent.NewWorktreePath != "" {
		t.Fatalf("expected NewWorktreePath to be empty on failure, got %q", recoveryEvent.NewWorktreePath)
	}
}
