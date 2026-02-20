package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/worktree"
)

type mockSessionWorktreeCreator struct {
	CreateSessionWorktreeFn func(command string) (*worktree.SessionWorktree, error)
	MergeBackFn             func(branch string) error
}

func (m *mockSessionWorktreeCreator) CreateSessionWorktree(command string) (*worktree.SessionWorktree, error) {
	if m != nil && m.CreateSessionWorktreeFn != nil {
		return m.CreateSessionWorktreeFn(command)
	}
	return nil, nil
}

func (m *mockSessionWorktreeCreator) MergeBack(branch string) error {
	if m != nil && m.MergeBackFn != nil {
		return m.MergeBackFn(branch)
	}
	return nil
}

type mockPendingBranchRecorder struct {
	AddPendingWorktreeBranchFn    func(branch string) error
	RemovePendingWorktreeBranchFn func(branch string) error
}

func (m *mockPendingBranchRecorder) AddPendingWorktreeBranch(branch string) error {
	if m != nil && m.AddPendingWorktreeBranchFn != nil {
		return m.AddPendingWorktreeBranchFn(branch)
	}
	return nil
}

func (m *mockPendingBranchRecorder) RemovePendingWorktreeBranch(branch string) error {
	if m != nil && m.RemovePendingWorktreeBranchFn != nil {
		return m.RemovePendingWorktreeBranchFn(branch)
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
	cleanupFn func(mainDir, sessionDir string) error,
) {
	t.Helper()

	origManagerFn := interactiveWorktreeNewManagerFn
	origStateFileFn := interactiveWorktreeNewStateFileFn
	origCleanupFn := interactiveWorktreeCleanupSessionFn
	interactiveWorktreeNewManagerFn = managerFn
	interactiveWorktreeNewStateFileFn = stateFileFn
	interactiveWorktreeCleanupSessionFn = cleanupFn
	t.Cleanup(func() {
		interactiveWorktreeNewManagerFn = origManagerFn
		interactiveWorktreeNewStateFileFn = origStateFileFn
		interactiveWorktreeCleanupSessionFn = origCleanupFn
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
	}, func(string, string) error { return nil })

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
	}, func(string, string) error { return nil })

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
	}, func(string, string) error { return nil })

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

func TestRunWithSessionWorktreeImmediateMergeSuccessRunsCleanupAndClearsPendingBranch(t *testing.T) {
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "debug")
	session.BranchName = "gromit/debug-123"

	var (
		addCalled    bool
		mergeCalled  bool
		removeCalled bool
		cleanupMain  string
		cleanupDir   string
	)

	withInteractiveWorktreeFactories(t, func(gotMainDir string) (sessionWorktreeCreator, error) {
		if gotMainDir != mainDir {
			t.Fatalf("mainDir = %q, want %q", gotMainDir, mainDir)
		}
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(branch string) error {
				if !addCalled {
					t.Fatal("merge happened before pending branch was recorded")
				}
				if branch != session.BranchName {
					t.Fatalf("merge branch = %q, want %q", branch, session.BranchName)
				}
				mergeCalled = true
				return nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			AddPendingWorktreeBranchFn: func(branch string) error {
				if branch != session.BranchName {
					t.Fatalf("add branch = %q, want %q", branch, session.BranchName)
				}
				addCalled = true
				return nil
			},
			RemovePendingWorktreeBranchFn: func(branch string) error {
				if !mergeCalled {
					t.Fatal("pending branch removed before merge succeeded")
				}
				if branch != session.BranchName {
					t.Fatalf("remove branch = %q, want %q", branch, session.BranchName)
				}
				removeCalled = true
				return nil
			},
		}, nil
	}, func(gotMainDir, sessionDir string) error {
		if !removeCalled {
			t.Fatal("cleanup happened before pending branch was removed")
		}
		cleanupMain = gotMainDir
		cleanupDir = sessionDir
		return nil
	})

	_, err := runWithSessionWorktree(gromitDir, "debug", func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktree() error = %v", err)
	}
	if !mergeCalled {
		t.Fatal("expected immediate merge attempt after callback success")
	}
	if !removeCalled {
		t.Fatal("expected pending branch removal after successful merge")
	}
	if cleanupMain != mainDir {
		t.Fatalf("cleanup mainDir = %q, want %q", cleanupMain, mainDir)
	}
	if cleanupDir != session.WorktreeDir {
		t.Fatalf("cleanup sessionDir = %q, want %q", cleanupDir, session.WorktreeDir)
	}
}
