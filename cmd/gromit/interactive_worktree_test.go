package main

import (
	"errors"
	"path/filepath"
	"strings"
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

func TestRunWithSessionWorktreeConflictManualPolicyKeepsPendingBranch(t *testing.T) {
	_, gromitDir, session := setupRunWithSessionWorktreeTest(t, "review")
	session.BranchName = "gromit/review-111"

	var (
		removeCalled  bool
		cleanupCalled bool
	)

	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(string) error {
				return errors.New("merge conflict for branch gromit/review-111")
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			RemovePendingWorktreeBranchFn: func(string) error {
				removeCalled = true
				return nil
			},
		}, nil
	}, func(string, string) error {
		cleanupCalled = true
		return nil
	})

	result, err := runWithSessionWorktreeWithConflictSettings(gromitDir, "review", sessionConflictSettings{
		Policy: "manual",
	}, func(string) error { return nil })
	if err == nil {
		t.Fatal("expected conflict handoff error, got nil")
	}
	if !isMergeConflictHandoffError(err) {
		t.Fatalf("expected merge conflict handoff error, got %T (%v)", err, err)
	}
	if result == nil {
		t.Fatal("expected session result to be returned on conflict")
	}
	if result.BranchName != session.BranchName {
		t.Fatalf("result.BranchName = %q, want %q", result.BranchName, session.BranchName)
	}
	if removeCalled {
		t.Fatal("pending branch should remain on manual conflict handoff")
	}
	if cleanupCalled {
		t.Fatal("session worktree should be preserved on manual conflict handoff")
	}
	if !strings.Contains(err.Error(), "manual handoff") {
		t.Fatalf("error should contain manual handoff guidance, got: %v", err)
	}
}

func TestRunWithSessionWorktreeConflictAgentPolicyRetriesAndMerges(t *testing.T) {
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "refine")
	session.BranchName = "gromit/refine-222"

	var (
		mergeCalls        int
		resolveCalls      int
		removeCalled      bool
		cleanupCalled     bool
		cleanupMainDirGot string
	)

	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(branch string) error {
				mergeCalls++
				if branch != session.BranchName {
					t.Fatalf("merge branch = %q, want %q", branch, session.BranchName)
				}
				if mergeCalls == 1 {
					return errors.New("merge conflict")
				}
				return nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			RemovePendingWorktreeBranchFn: func(string) error {
				removeCalled = true
				return nil
			},
		}, nil
	}, func(gotMainDir, gotSessionDir string) error {
		cleanupCalled = true
		cleanupMainDirGot = gotMainDir
		if gotSessionDir != session.WorktreeDir {
			t.Fatalf("cleanup session dir = %q, want %q", gotSessionDir, session.WorktreeDir)
		}
		return nil
	})

	result, err := runWithSessionWorktreeWithConflictSettings(gromitDir, "refine", sessionConflictSettings{
		Policy:   "agent",
		RetryCap: 2,
		AgentConflictResolver: func(sessionDir, branch string, attempt int) error {
			resolveCalls++
			if sessionDir != session.WorktreeDir {
				t.Fatalf("resolver sessionDir = %q, want %q", sessionDir, session.WorktreeDir)
			}
			if branch != session.BranchName {
				t.Fatalf("resolver branch = %q, want %q", branch, session.BranchName)
			}
			if attempt != 1 {
				t.Fatalf("resolver attempt = %d, want 1", attempt)
			}
			return nil
		},
	}, func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktreeWithConflictSettings() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if mergeCalls != 2 {
		t.Fatalf("merge calls = %d, want 2 (initial + post-resolver)", mergeCalls)
	}
	if resolveCalls != 1 {
		t.Fatalf("resolver calls = %d, want 1", resolveCalls)
	}
	if !removeCalled {
		t.Fatal("expected pending branch removal after agent-assisted merge success")
	}
	if !cleanupCalled {
		t.Fatal("expected cleanup after agent-assisted merge success")
	}
	if cleanupMainDirGot != mainDir {
		t.Fatalf("cleanup mainDir = %q, want %q", cleanupMainDirGot, mainDir)
	}
}

func TestRunWithSessionWorktreeConflictAgentPolicyFallsBackToManual(t *testing.T) {
	_, gromitDir, session := setupRunWithSessionWorktreeTest(t, "debug")
	session.BranchName = "gromit/debug-333"

	var (
		mergeCalls    int
		resolveCalls  int
		removeCalled  bool
		cleanupCalled bool
	)

	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(string) error {
				mergeCalls++
				return errors.New("merge conflict")
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			RemovePendingWorktreeBranchFn: func(string) error {
				removeCalled = true
				return nil
			},
		}, nil
	}, func(string, string) error {
		cleanupCalled = true
		return nil
	})

	result, err := runWithSessionWorktreeWithConflictSettings(gromitDir, "debug", sessionConflictSettings{
		Policy:   "agent",
		RetryCap: 2,
		AgentConflictResolver: func(string, string, int) error {
			resolveCalls++
			return nil
		},
	}, func(string) error { return nil })
	if err == nil {
		t.Fatal("expected manual-handoff error after exhausted retries")
	}
	if !isMergeConflictHandoffError(err) {
		t.Fatalf("expected merge conflict handoff error, got %T (%v)", err, err)
	}
	if !strings.Contains(err.Error(), "after 2 agent attempt(s)") {
		t.Fatalf("error should include retry attempts, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil session result on conflict fallback")
	}
	if mergeCalls != 3 {
		t.Fatalf("merge calls = %d, want 3 (initial + 2 retries)", mergeCalls)
	}
	if resolveCalls != 2 {
		t.Fatalf("resolver calls = %d, want 2", resolveCalls)
	}
	if removeCalled {
		t.Fatal("pending branch should be retained after fallback to manual handoff")
	}
	if cleanupCalled {
		t.Fatal("session worktree should be preserved after fallback to manual handoff")
	}
}
