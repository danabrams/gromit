package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/worktree"
)

type mockSessionWorktreeCreator struct {
	CreateSessionWorktreeFn func(command string) (*worktree.SessionWorktree, error)
}

func (m *mockSessionWorktreeCreator) CreateSessionWorktree(command string) (*worktree.SessionWorktree, error) {
	if m != nil && m.CreateSessionWorktreeFn != nil {
		return m.CreateSessionWorktreeFn(command)
	}
	return nil, nil
}

type mockPendingBranchRecorder struct {
	AddPendingWorktreeBranchFn func(branch string) error
}

func (m *mockPendingBranchRecorder) AddPendingWorktreeBranch(branch string) error {
	if m != nil && m.AddPendingWorktreeBranchFn != nil {
		return m.AddPendingWorktreeBranchFn(branch)
	}
	return nil
}

func TestRunWithSessionWorktreeExecutesCallbackInSessionDir(t *testing.T) {
	t.Helper()

	mainDir := t.TempDir()
	gromitDir := filepath.Join(mainDir, ".gromit")
	session := &worktree.SessionWorktree{
		BranchName:  "gromit/refine-123",
		WorktreeDir: filepath.Join(mainDir, "session-refine"),
	}

	origManagerFn := interactiveWorktreeNewManagerFn
	origStateFileFn := interactiveWorktreeNewStateFileFn
	t.Cleanup(func() {
		interactiveWorktreeNewManagerFn = origManagerFn
		interactiveWorktreeNewStateFileFn = origStateFileFn
	})

	interactiveWorktreeNewManagerFn = func(gotMainDir string) (sessionWorktreeCreator, error) {
		if gotMainDir != mainDir {
			t.Fatalf("mainDir = %q, want %q", gotMainDir, mainDir)
		}
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(command string) (*worktree.SessionWorktree, error) {
				if command != "refine" {
					t.Fatalf("command = %q, want %q", command, "refine")
				}
				return session, nil
			},
		}, nil
	}
	interactiveWorktreeNewStateFileFn = func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{}, nil
	}

	callbackCalled := false
	callbackDir := ""
	result, err := runWithSessionWorktree(gromitDir, "refine", func(sessionDir string) error {
		callbackCalled = true
		callbackDir = sessionDir
		return nil
	})
	if err != nil {
		t.Fatalf("runWithSessionWorktree() error = %v", err)
	}
	if !callbackCalled {
		t.Fatal("callback was not called")
	}
	if callbackDir != session.WorktreeDir {
		t.Fatalf("callback dir = %q, want %q", callbackDir, session.WorktreeDir)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.BranchName != session.BranchName {
		t.Fatalf("result.BranchName = %q, want %q", result.BranchName, session.BranchName)
	}
}

func TestRunWithSessionWorktreeRecordsPendingBranch(t *testing.T) {
	t.Helper()

	mainDir := t.TempDir()
	gromitDir := filepath.Join(mainDir, ".gromit")
	session := &worktree.SessionWorktree{
		BranchName:  "gromit/plan-456",
		WorktreeDir: filepath.Join(mainDir, "session-plan"),
	}

	origManagerFn := interactiveWorktreeNewManagerFn
	origStateFileFn := interactiveWorktreeNewStateFileFn
	t.Cleanup(func() {
		interactiveWorktreeNewManagerFn = origManagerFn
		interactiveWorktreeNewStateFileFn = origStateFileFn
	})

	interactiveWorktreeNewManagerFn = func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
		}, nil
	}

	recordedBranch := ""
	callbackRan := false
	interactiveWorktreeNewStateFileFn = func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			AddPendingWorktreeBranchFn: func(branch string) error {
				if !callbackRan {
					t.Fatal("branch recording happened before callback completed")
				}
				recordedBranch = branch
				return nil
			},
		}, nil
	}

	_, err := runWithSessionWorktree(gromitDir, "plan", func(string) error {
		callbackRan = true
		return nil
	})
	if err != nil {
		t.Fatalf("runWithSessionWorktree() error = %v", err)
	}
	if recordedBranch != session.BranchName {
		t.Fatalf("recorded branch = %q, want %q", recordedBranch, session.BranchName)
	}
}

func TestRunWithSessionWorktreeDoesNotRecordBranchWhenCallbackFails(t *testing.T) {
	t.Helper()

	mainDir := t.TempDir()
	gromitDir := filepath.Join(mainDir, ".gromit")
	session := &worktree.SessionWorktree{
		BranchName:  "gromit/explore-789",
		WorktreeDir: filepath.Join(mainDir, "session-explore"),
	}

	origManagerFn := interactiveWorktreeNewManagerFn
	origStateFileFn := interactiveWorktreeNewStateFileFn
	t.Cleanup(func() {
		interactiveWorktreeNewManagerFn = origManagerFn
		interactiveWorktreeNewStateFileFn = origStateFileFn
	})

	interactiveWorktreeNewManagerFn = func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
		}, nil
	}

	recordCalled := false
	interactiveWorktreeNewStateFileFn = func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			AddPendingWorktreeBranchFn: func(string) error {
				recordCalled = true
				return nil
			},
		}, nil
	}

	wantErr := errors.New("callback failed")
	_, err := runWithSessionWorktree(gromitDir, "explore", func(string) error {
		return wantErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
	if recordCalled {
		t.Fatal("branch should not be recorded when callback fails")
	}
}
