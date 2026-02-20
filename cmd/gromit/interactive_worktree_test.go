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

var (
	_ sessionWorktreeCreator = (*mockSessionWorktreeCreator)(nil)
	_ pendingBranchRecorder  = (*mockPendingBranchRecorder)(nil)
)

func setupRunWithSessionWorktreeTest(t *testing.T, command string) (mainDir string, gromitDir string, session *worktree.SessionWorktree) {
	t.Helper()

	mainDir = t.TempDir()
	gromitDir = filepath.Join(mainDir, ".gromit")
	session = &worktree.SessionWorktree{
		BranchName:  "gromit/" + command + "-test-branch",
		WorktreeDir: filepath.Join(mainDir, "session-"+command),
	}

	return mainDir, gromitDir, session
}

func withInteractiveWorktreeFactories(
	t *testing.T,
	managerFn func(mainDir string) (sessionWorktreeCreator, error),
	stateFileFn func(gromitDir string) (pendingBranchRecorder, error),
) {
	t.Helper()

	origManagerFn := interactiveWorktreeNewManagerFn
	origStateFileFn := interactiveWorktreeNewStateFileFn
	interactiveWorktreeNewManagerFn = managerFn
	interactiveWorktreeNewStateFileFn = stateFileFn
	t.Cleanup(func() {
		interactiveWorktreeNewManagerFn = origManagerFn
		interactiveWorktreeNewStateFileFn = origStateFileFn
	})
}

func TestRunWithSessionWorktreeExecutesCallbackInSessionDir(t *testing.T) {
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "refine")
	session.BranchName = "gromit/refine-123"

	withInteractiveWorktreeFactories(t, func(gotMainDir string) (sessionWorktreeCreator, error) {
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
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{}, nil
	})

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
	_, gromitDir, session := setupRunWithSessionWorktreeTest(t, "plan")
	session.BranchName = "gromit/plan-456"

	recordedBranch := ""
	callbackRan := false
	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			AddPendingWorktreeBranchFn: func(branch string) error {
				if !callbackRan {
					t.Fatal("branch recording happened before callback completed")
				}
				recordedBranch = branch
				return nil
			},
		}, nil
	})

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
	_, gromitDir, session := setupRunWithSessionWorktreeTest(t, "explore")
	session.BranchName = "gromit/explore-789"

	recordCalled := false
	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			AddPendingWorktreeBranchFn: func(string) error {
				recordCalled = true
				return nil
			},
		}, nil
	})

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
