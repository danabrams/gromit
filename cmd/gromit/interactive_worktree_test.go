package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/danabrams/gromit/internal/integrationqueue"
	"github.com/danabrams/gromit/internal/worktree"
)

type mockSessionWorktreeCreator struct {
	CreateSessionWorktreeFn           func(command string) (*worktree.SessionWorktree, error)
	CreateSessionWorktreeWithCtxFn    func(ctx context.Context, command string) (*worktree.SessionWorktree, error)
	MergeBackFn                       func(branch string) error
}

func (m *mockSessionWorktreeCreator) CreateSessionWorktree(ctx context.Context, command string) (*worktree.SessionWorktree, error) {
	if m != nil {
		if m.CreateSessionWorktreeWithCtxFn != nil {
			return m.CreateSessionWorktreeWithCtxFn(ctx, command)
		}
		if m.CreateSessionWorktreeFn != nil {
			return m.CreateSessionWorktreeFn(command)
		}
	}
	return nil, nil
}

func (m *mockSessionWorktreeCreator) MergeBack(_ context.Context, branch string) error {
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

type mockQueueStore struct {
	SaveFn   func(entry integrationqueue.Entry) error
	DeleteFn func(branch string) error
}

func (m *mockQueueStore) Save(entry integrationqueue.Entry) error {
	if m != nil && m.SaveFn != nil {
		return m.SaveFn(entry)
	}
	return nil
}

func (m *mockQueueStore) Delete(branch string) error {
	if m != nil && m.DeleteFn != nil {
		return m.DeleteFn(branch)
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

	cleanupGit := overrideGitRun(autoCommitGitRun(""))
	t.Cleanup(cleanupGit)

	return mainDir, gromitDir, session
}

func defaultTestGitRun(dir string, args ...string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	switch args[0] {
	case "rev-parse":
		if len(args) > 1 && args[1] == "HEAD" {
			return "default-head", nil
		}
		if len(args) > 1 && args[1] == "HEAD^" {
			return "default-base", nil
		}
	case "diff":
		return "cmd/gromit/default.go\n", nil
	}
	return "", nil
}

func autoCommitGitRun(changedFile string) func(dir string, args ...string) (string, error) {
	if changedFile == "" {
		changedFile = "cmd/gromit/example.go"
	}
	return func(dir string, args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "add", "commit":
			return "", nil
		case "status":
			return fmt.Sprintf(" M %s\n", changedFile), nil
		case "rev-parse":
			if len(args) > 1 {
				if args[1] == "HEAD" {
					return "headsha", nil
				}
				if args[1] == "HEAD^" {
					return "baseref", nil
				}
			}
		case "diff":
			if len(args) > 1 {
				if args[1] == "--cached" {
					return "", nil
				}
				if len(args) > 2 && args[1] == "--name-only" {
					return changedFile + "\n", nil
				}
			}
		}
		return "", nil
	}
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

func overrideGitRun(fn func(dir string, args ...string) (string, error)) func() {
	original := interactiveWorktreeGitRunFn
	interactiveWorktreeGitRunFn = fn
	return func() {
		interactiveWorktreeGitRunFn = original
	}
}

func overrideQueueStore(saveFn func(entry integrationqueue.Entry) error) func() {
	return overrideQueueStoreWithDelete(saveFn, nil)
}

func overrideQueueStoreWithDelete(
	saveFn func(entry integrationqueue.Entry) error,
	deleteFn func(branch string) error,
) func() {
	original := interactiveWorktreeNewQueueStoreFn
	interactiveWorktreeNewQueueStoreFn = func(gromitDir string) (sessionQueueStore, error) {
		return &mockQueueStore{SaveFn: saveFn, DeleteFn: deleteFn}, nil
	}
	return func() {
		interactiveWorktreeNewQueueStoreFn = original
	}
}

func TestRunWithSessionWorktreeExecutesCallbackInSessionDir(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
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

func TestRunWithSessionWorktreeWithConflictSettingsPropagatesContext(t *testing.T) {
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "context-propagation")
	session.BranchName = "gromit/context-propagation-123"

	ctx := context.WithValue(context.Background(), struct{}{}, "marker")
	var gotCtx context.Context

	cleanupGit := overrideGitRun(autoCommitGitRun(""))
	t.Cleanup(cleanupGit)

	withInteractiveWorktreeFactories(t,
		func(gotMainDir string) (sessionWorktreeCreator, error) {
			if gotMainDir != mainDir {
				t.Fatalf("mainDir = %q, want %q", gotMainDir, mainDir)
			}
			return &mockSessionWorktreeCreator{
				CreateSessionWorktreeWithCtxFn: func(c context.Context, command string) (*worktree.SessionWorktree, error) {
					if command != "context-propagation" {
						t.Fatalf("command = %q, want %q", command, "context-propagation")
					}
					gotCtx = c
					return session, nil
				},
			}, nil
		},
		func(string) (pendingBranchRecorder, error) {
			return &mockPendingBranchRecorder{AddPendingWorktreeBranchFn: func(string) error { return nil }}, nil
		},
		func(string, string) error {
			return nil
		},
	)

	_, err := runWithSessionWorktreeWithConflictSettings(ctx, gromitDir, "context-propagation", sessionConflictSettings{}, func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktreeWithConflictSettings() error = %v", err)
	}
	if gotCtx != ctx {
		t.Fatalf("context = %v, want %v", gotCtx, ctx)
	}
}

func TestRunWithSessionWorktreeAutoCommitInvoked(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "auto")
	session.BranchName = "gromit/auto-456"

	var commands []string
	cleanupGit := overrideGitRun(func(dir string, args ...string) (string, error) {
		commands = append(commands, strings.Join(args, " "))
		if len(args) > 0 {
			switch args[0] {
			case "rev-parse":
				if len(args) > 1 && args[1] == "HEAD" {
					return "headsha", nil
				}
				if len(args) > 1 && args[1] == "HEAD^" {
					return "baseref", nil
				}
			case "diff":
				return "cmd/gromit/example.go\n", nil
			case "status":
				return " M cmd/gromit/example.go\n", nil
			}
		}
		return "", nil
	})
	t.Cleanup(cleanupGit)

	withInteractiveWorktreeFactories(t, func(gotMainDir string) (sessionWorktreeCreator, error) {
		if gotMainDir != mainDir {
			t.Fatalf("mainDir = %q, want %q", gotMainDir, mainDir)
		}
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{AddPendingWorktreeBranchFn: func(string) error { return nil }}, nil
	}, func(string, string) error {
		return nil
	})

	_, err := runWithSessionWorktree(gromitDir, "auto", func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktree() error = %v", err)
	}

	var sawAdd, sawCommit, sawAllowEmpty bool
	for _, cmd := range commands {
		if strings.Contains(cmd, "add -A") {
			sawAdd = true
		}
		if strings.Contains(cmd, "commit") && strings.Contains(cmd, "-m") {
			sawCommit = true
		}
		if strings.Contains(cmd, "commit --allow-empty") {
			sawAllowEmpty = true
		}
	}
	if !sawAdd || !sawCommit || !sawAllowEmpty {
		t.Fatalf("auto commit commands not run: %v", commands)
	}
}

func TestRunWithSessionWorktreeSkipsCommitWhenNoChanges(t *testing.T) {
	_, gromitDir, session := setupRunWithSessionWorktreeTest(t, "nochange")
	session.BranchName = "gromit/nochange-123"

	var commands []string
	cleanupGit := overrideGitRun(func(dir string, args ...string) (string, error) {
		commands = append(commands, strings.Join(args, " "))
		if len(args) > 0 && args[0] == "diff" {
			if len(args) > 2 && args[1] == "--cached" && args[2] == "--quiet" {
				return "", nil
			}
		}
		return "", nil
	})
	t.Cleanup(cleanupGit)

	var recordedEntries []integrationqueue.Entry
	var deletedBranches []string
	cleanupStore := overrideQueueStoreWithDelete(func(entry integrationqueue.Entry) error {
		recordedEntries = append(recordedEntries, entry)
		return nil
	}, func(branch string) error {
		deletedBranches = append(deletedBranches, branch)
		return nil
	})
	t.Cleanup(cleanupStore)

	cleanupCalled := false
	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{}, nil
	}, func(string, string) error {
		cleanupCalled = true
		return nil
	})

	_, err := runWithSessionWorktree(gromitDir, "nochange", func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktree() error = %v", err)
	}

	for _, cmd := range commands {
		if strings.Contains(cmd, "commit") {
			t.Fatalf("unexpected commit command: %v", commands)
		}
	}
	if !cleanupCalled {
		t.Fatal("expected cleanup to run")
	}
	if len(recordedEntries) != 1 {
		t.Fatalf("expected only draft entry, got %d", len(recordedEntries))
	}
	if recordedEntries[0].State != integrationqueue.StateDraft {
		t.Fatalf("draft entry has state %q", recordedEntries[0].State)
	}
	if len(deletedBranches) != 1 || deletedBranches[0] != session.BranchName {
		t.Fatalf("deleted branches = %v, want [%s]", deletedBranches, session.BranchName)
	}
}

func TestRunWithSessionWorktreeIgnoresDiffWarnings(t *testing.T) {
	_, gromitDir, session := setupRunWithSessionWorktreeTest(t, "warning")
	session.BranchName = "gromit/warning-123"

	var commands []string
	cleanupGit := overrideGitRun(func(dir string, args ...string) (string, error) {
		commands = append(commands, strings.Join(args, " "))
		if len(args) > 2 && args[0] == "diff" && args[1] == "--cached" && args[2] == "--quiet" {
			return "warning: CRLF will be replaced by LF in file.go\n", nil
		}
		return "", nil
	})
	t.Cleanup(cleanupGit)

	var recordedEntries []integrationqueue.Entry
	var deletedBranches []string
	cleanupStore := overrideQueueStoreWithDelete(func(entry integrationqueue.Entry) error {
		recordedEntries = append(recordedEntries, entry)
		return nil
	}, func(branch string) error {
		deletedBranches = append(deletedBranches, branch)
		return nil
	})
	t.Cleanup(cleanupStore)

	cleanupCalled := false
	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{}, nil
	}, func(string, string) error {
		cleanupCalled = true
		return nil
	})

	if _, err := runWithSessionWorktree(gromitDir, "warning", func(string) error { return nil }); err != nil {
		t.Fatalf("runWithSessionWorktree() error = %v", err)
	}

	for _, cmd := range commands {
		if strings.Contains(cmd, "commit") {
			t.Fatalf("unexpected commit command: %v", commands)
		}
	}
	if !cleanupCalled {
		t.Fatal("expected cleanup to run")
	}
	if len(recordedEntries) != 1 {
		t.Fatalf("expected only draft entry, got %d", len(recordedEntries))
	}
	if recordedEntries[0].State != integrationqueue.StateDraft {
		t.Fatalf("draft entry has state %q", recordedEntries[0].State)
	}
	if len(deletedBranches) != 1 || deletedBranches[0] != session.BranchName {
		t.Fatalf("deleted branches = %v, want [%s]", deletedBranches, session.BranchName)
	}
}

func TestRunWithSessionWorktreeQueuesReadyBranch(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "ready-queue")
	session.BranchName = "gromit/ready-123"

	var recordedEntry *integrationqueue.Entry
	cleanupStore := overrideQueueStore(func(entry integrationqueue.Entry) error {
		recordedEntry = &entry
		return nil
	})
	t.Cleanup(cleanupStore)

	cleanupGit := overrideGitRun(func(dir string, args ...string) (string, error) {
		switch args[0] {
		case "add":
			return "", nil
		case "commit":
			return "", nil
		case "rev-parse":
			if len(args) > 1 && args[1] == "HEAD" {
				return "headsha", nil
			}
			if len(args) > 1 && args[1] == "HEAD^" {
				return "baseref", nil
			}
		case "diff":
			return "cmd/gromit/example.go\n", nil
		case "status":
			return " M cmd/gromit/example.go\n", nil
		}
		return "", nil
	})
	t.Cleanup(cleanupGit)

	withInteractiveWorktreeFactories(t, func(gotMainDir string) (sessionWorktreeCreator, error) {
		if gotMainDir != mainDir {
			t.Fatalf("mainDir = %q, want %q", gotMainDir, mainDir)
		}
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{AddPendingWorktreeBranchFn: func(string) error { return nil }}, nil
	}, func(string, string) error {
		return nil
	})

	_, err := runWithSessionWorktree(gromitDir, "ready-queue", func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktree() error = %v", err)
	}
	if recordedEntry == nil {
		t.Fatal("expected queue entry to be recorded")
	}
	if recordedEntry.State != "ready" {
		t.Fatalf("entry state = %q, want ready", recordedEntry.State)
	}
	if recordedEntry.Branch != session.BranchName {
		t.Fatalf("entry branch = %q, want %q", recordedEntry.Branch, session.BranchName)
	}
	if recordedEntry.OriginCommand != "ready-queue" {
		t.Fatalf("entry origin command = %q, want %q", recordedEntry.OriginCommand, "ready-queue")
	}
	if len(recordedEntry.ChangedFiles) != 1 || recordedEntry.ChangedFiles[0] != "cmd/gromit/example.go" {
		t.Fatalf("changed files = %v, want [cmd/gromit/example.go]", recordedEntry.ChangedFiles)
	}
}

func TestRunWithSessionWorktreeRecordsBlockedQueueEntryOnCommitFailure(t *testing.T) {
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "blocked")
	session.BranchName = "gromit/blocked-123"

	var recordedEntry *integrationqueue.Entry
	cleanupStore := overrideQueueStore(func(entry integrationqueue.Entry) error {
		recordedEntry = &entry
		return nil
	})
	t.Cleanup(cleanupStore)

	commitErr := errors.New("git commit failed")
	cleanupGit := overrideGitRun(func(dir string, args ...string) (string, error) {
		switch args[0] {
		case "commit":
			return "", commitErr
		case "rev-parse":
			if len(args) > 1 && args[1] == "HEAD" {
				return "blocked-head", nil
			}
		case "status":
			return " M cmd/gromit/blocked.go\n", nil
		}
		return "", nil
	})
	t.Cleanup(cleanupGit)

	var pendingRecorded bool
	withInteractiveWorktreeFactories(t, func(gotMainDir string) (sessionWorktreeCreator, error) {
		if gotMainDir != mainDir {
			t.Fatalf("mainDir = %q, want %q", gotMainDir, mainDir)
		}
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			AddPendingWorktreeBranchFn: func(branch string) error {
				if branch != session.BranchName {
					t.Fatalf("AddPendingWorktreeBranch got %q, want %q", branch, session.BranchName)
				}
				pendingRecorded = true
				return nil
			},
		}, nil
	}, func(string, string) error {
		return nil
	})

	_, err := runWithSessionWorktree(gromitDir, "blocked", func(string) error { return nil })
	if err == nil {
		t.Fatal("expected auto commit error, got nil")
	}
	if !strings.Contains(err.Error(), "auto-commit failed") {
		t.Fatalf("error = %v, want auto-commit failure", err)
	}
	if !pendingRecorded {
		t.Fatal("pending branch should be recorded on failure")
	}
	if recordedEntry == nil {
		t.Fatal("expected blocked queue entry")
	}
	if recordedEntry.State != "conflict" {
		t.Fatalf("entry state = %q, want conflict", recordedEntry.State)
	}
	if recordedEntry.LastErrorCode != "session_commit_failed" {
		t.Fatalf("error code = %q, want session_commit_failed", recordedEntry.LastErrorCode)
	}
	if !strings.Contains(recordedEntry.LastErrorMessage, session.BranchName) {
		t.Fatalf("last error message %q should mention branch %q", recordedEntry.LastErrorMessage, session.BranchName)
	}
	if len(recordedEntry.ChangedFiles) != 1 || recordedEntry.ChangedFiles[0] != "cmd/gromit/blocked.go" {
		t.Fatalf("changed files = %v, want [cmd/gromit/blocked.go]", recordedEntry.ChangedFiles)
	}
}

// TestEnqueueBranchBaseRefFallback ensures ready and blocked queue entries populate base_ref
// even when the provided metadata only contains the head SHA.
func TestEnqueueBranchBaseRefFallback(t *testing.T) {
	gromitDir := t.TempDir()
	session := &worktree.SessionWorktree{
		BranchName:  "gromit/base-ref-test",
		WorktreeDir: filepath.Join(gromitDir, "session-base-ref-test"),
	}

	var recorded []integrationqueue.Entry
	cleanupStore := overrideQueueStore(func(entry integrationqueue.Entry) error {
		recorded = append(recorded, entry)
		return nil
	})
	t.Cleanup(cleanupStore)

	readyMeta := &sessionCommitMetadata{headSHA: "ready-head"}
	if err := enqueueReadyBranch(gromitDir, "ready", session, readyMeta); err != nil {
		t.Fatalf("enqueueReadyBranch() error = %v", err)
	}
	if len(recorded) != 1 {
		t.Fatalf("ready entry count = %d, want 1", len(recorded))
	}
	if recorded[0].BaseRef != readyMeta.headSHA {
		t.Fatalf("ready entry base ref = %q, want %q", recorded[0].BaseRef, readyMeta.headSHA)
	}

	recorded = nil
	blockedMeta := &sessionCommitMetadata{headSHA: "blocked-head"}
	if err := enqueueBlockedBranch(gromitDir, "blocked", session, blockedMeta, errors.New("failed")); err != nil {
		t.Fatalf("enqueueBlockedBranch() error = %v", err)
	}
	if len(recorded) != 1 {
		t.Fatalf("blocked entry count = %d, want 1", len(recorded))
	}
	if recorded[0].BaseRef != blockedMeta.headSHA {
		t.Fatalf("blocked entry base ref = %q, want %q", recorded[0].BaseRef, blockedMeta.headSHA)
	}
}

func TestRunWithSessionWorktreeRecordsPendingBranch(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	_, gromitDir, session := setupRunWithSessionWorktreeTest(t, "plan")
	session.BranchName = "gromit/plan-456"

	cleanupGit := overrideGitRun(autoCommitGitRun(""))
	t.Cleanup(cleanupGit)

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
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
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

// TestRunWithSessionWorktreeSuccessPath_RecordsAndQueuesForCoordinator verifies single-writer
// semantics: session success path records branch as pending for coordinator, does NOT merge.
func TestRunWithSessionWorktreeSuccessPath_RecordsAndQueuesForCoordinator(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "debug")
	session.BranchName = "gromit/debug-123"

	cleanupGit := overrideGitRun(autoCommitGitRun(""))
	t.Cleanup(cleanupGit)

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
				// Single-writer: sessions should NOT merge. This is a regression.
				t.Fatalf("regression: session attempted to merge branch %q to main", branch)
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
				// Branch should remain pending for coordinator; no removal here
				removeCalled = true
				return nil
			},
		}, nil
	}, func(gotMainDir, sessionDir string) error {
		if !addCalled {
			t.Fatal("cleanup happened before pending branch was recorded")
		}
		if removeCalled {
			t.Fatal("cleanup should not remove pending branch in single-writer model")
		}
		cleanupMain = gotMainDir
		cleanupDir = sessionDir
		return nil
	})

	_, err := runWithSessionWorktree(gromitDir, "debug", func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktree() error = %v", err)
	}
	if mergeCalled {
		t.Fatal("session should not attempt merge under single-writer policy")
	}
	if !addCalled {
		t.Fatal("expected branch to be recorded as pending for coordinator")
	}
	if cleanupMain != mainDir {
		t.Fatalf("cleanup mainDir = %q, want %q", cleanupMain, mainDir)
	}
	if cleanupDir != session.WorktreeDir {
		t.Fatalf("cleanup sessionDir = %q, want %q", cleanupDir, session.WorktreeDir)
	}
}

// TestRunWithSessionWorktreeOrderingInSingleWriter verifies single-writer event ordering:
// pending recorded, cleanup, then return (no merge, no removal in session path).
func TestRunWithSessionWorktreeOrderingInSingleWriter(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "sync")
	session.BranchName = "gromit/sync-999"

	cleanupGit := overrideGitRun(autoCommitGitRun(""))
	t.Cleanup(cleanupGit)

	var events []string

	withInteractiveWorktreeFactories(t, func(gotMainDir string) (sessionWorktreeCreator, error) {
		if gotMainDir != mainDir {
			t.Fatalf("mainDir = %q, want %q", gotMainDir, mainDir)
		}
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(branch string) error {
				// Single-writer: no merge in session path
				t.Fatalf("regression: session attempted to merge %q", branch)
				return nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			AddPendingWorktreeBranchFn: func(branch string) error {
				if branch != session.BranchName {
					t.Fatalf("add branch = %q, want %q", branch, session.BranchName)
				}
				events = append(events, "add")
				return nil
			},
			RemovePendingWorktreeBranchFn: func(branch string) error {
				if branch != session.BranchName {
					t.Fatalf("remove branch = %q, want %q", branch, session.BranchName)
				}
				events = append(events, "remove")
				return nil
			},
		}, nil
	}, func(gotMainDir, sessionDir string) error {
		if gotMainDir != mainDir {
			t.Fatalf("cleanup mainDir = %q, want %q", gotMainDir, mainDir)
		}
		if sessionDir != session.WorktreeDir {
			t.Fatalf("cleanup sessionDir = %q, want %q", sessionDir, session.WorktreeDir)
		}
		events = append(events, "cleanup")
		return nil
	})

	_, err := runWithSessionWorktree(gromitDir, "sync", func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktree() error = %v", err)
	}

	// Single-writer: add pending, cleanup worktree, then return. No merge or removal in session.
	want := []string{"add", "cleanup"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

// TestSessionCleanupHappensBeforeCoordinatorTakeover verifies cleanup of session worktree
// happens before returning to orchestrator for coordination. Single-writer semantics.
func TestSessionCleanupHappensBeforeCoordinatorTakeover(t *testing.T) {
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "cleanup-before-merge")
	session.BranchName = "gromit/cleanup-before-merge"

	var events []string
	cleanupGit := overrideGitRun(autoCommitGitRun(""))
	t.Cleanup(cleanupGit)

	withInteractiveWorktreeFactories(t, func(gotMainDir string) (sessionWorktreeCreator, error) {
		if gotMainDir != mainDir {
			t.Fatalf("mainDir = %q, want %q", gotMainDir, mainDir)
		}
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(branch string) error {
				// Single-writer: sessions don't merge
				t.Fatalf("regression: session attempted to merge %q", branch)
				return nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			AddPendingWorktreeBranchFn: func(branch string) error {
				if branch != session.BranchName {
					t.Fatalf("add branch = %q, want %q", branch, session.BranchName)
				}
				events = append(events, "add")
				return nil
			},
			RemovePendingWorktreeBranchFn: func(branch string) error {
				if branch != session.BranchName {
					t.Fatalf("remove branch = %q, want %q", branch, session.BranchName)
				}
				events = append(events, "remove")
				return nil
			},
		}, nil
	}, func(gotMainDir, sessionDir string) error {
		if gotMainDir != mainDir {
			t.Fatalf("cleanup mainDir = %q, want %q", gotMainDir, mainDir)
		}
		if sessionDir != session.WorktreeDir {
			t.Fatalf("cleanup sessionDir = %q, want %q", sessionDir, session.WorktreeDir)
		}
		events = append(events, "cleanup")
		return nil
	})

	_, err := runWithSessionWorktree(gromitDir, "cleanup-before-merge", func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktree() error = %v", err)
	}

	// Single-writer: add, cleanup, then return for coordinator
	want := []string{"add", "cleanup"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestImmediatePath_CleanupFailureSkipsMerge(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "repair")
	session.BranchName = "gromit/repair-111"

	cleanupGit := overrideGitRun(autoCommitGitRun(""))
	t.Cleanup(cleanupGit)

	preRemoveErr := errors.New("cannot remove worktree: uncommitted changes")
	var (
		addCalled    bool
		mergeCalled  bool
		removeCalled bool
	)

	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(string) error {
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
			RemovePendingWorktreeBranchFn: func(string) error {
				removeCalled = true
				return nil
			},
		}, nil
	}, func(gotMainDir, gotSessionDir string) error {
		if gotMainDir != mainDir {
			t.Fatalf("cleanup mainDir = %q, want %q", gotMainDir, mainDir)
		}
		if gotSessionDir != session.WorktreeDir {
			t.Fatalf("cleanup sessionDir = %q, want %q", gotSessionDir, session.WorktreeDir)
		}
		return preRemoveErr
	})

	result, err := runWithSessionWorktree(gromitDir, "repair", func(string) error { return nil })
	if err == nil {
		t.Fatal("expected pre-remove error, got nil")
	}
	if !errors.Is(err, preRemoveErr) {
		t.Fatalf("error = %v, want wrapped %v", err, preRemoveErr)
	}
	if result == nil {
		t.Fatal("expected session result to be returned on pre-remove failure")
	}
	if result.BranchName != session.BranchName {
		t.Fatalf("result.BranchName = %q, want %q", result.BranchName, session.BranchName)
	}
	if !addCalled {
		t.Fatal("expected pending branch to be recorded before cleanup")
	}
	if mergeCalled {
		t.Fatal("merge should not run when cleanup fails")
	}
	if removeCalled {
		t.Fatal("pending branch should not be removed when cleanup fails")
	}
}

func TestRunWithSessionWorktreeCleanupFailureWrapsBranchContext(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	_, gromitDir, session := setupRunWithSessionWorktreeTest(t, "context")
	session.BranchName = "gromit/context-222"

	cleanupGit := overrideGitRun(autoCommitGitRun(""))
	t.Cleanup(cleanupGit)

	preRemoveErr := errors.New("cannot remove worktree")
	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{}, nil
	}, func(string, string) error {
		return preRemoveErr
	})

	_, err := runWithSessionWorktree(gromitDir, "context", func(string) error { return nil })
	if err == nil {
		t.Fatal("expected pre-remove error, got nil")
	}
	if !errors.Is(err, preRemoveErr) {
		t.Fatalf("error = %v, want wrapped %v", err, preRemoveErr)
	}
	if !strings.Contains(err.Error(), session.BranchName) {
		t.Fatalf("error should include branch context %q, got: %v", session.BranchName, err)
	}
}

// TestSessionPathDoesNotAttemptConflictResolution verifies single-writer semantics:
// sessions never attempt merges, so no conflict scenarios occur in session path.
func TestSessionPathDoesNotAttemptConflictResolution(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	_, gromitDir, session := setupRunWithSessionWorktreeTest(t, "review")
	session.BranchName = "gromit/review-111"

	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(branch string) error {
				// Single-writer: sessions never call MergeBack
				t.Fatalf("regression: session attempted merge of %q", branch)
				return nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{}, nil
	}, func(string, string) error {
		return nil
	})

	// Under single-writer, session completion succeeds without conflict handling
	result, err := runWithSessionWorktreeWithConflictSettings(gromitDir, "review", sessionConflictSettings{
		Policy: "manual",
	}, func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktreeWithConflictSettings() error = %v (expected nil)", err)
	}
	if result == nil {
		t.Fatal("expected non-nil session result")
	}
	if result.BranchName != session.BranchName {
		t.Fatalf("result.BranchName = %q, want %q", result.BranchName, session.BranchName)
	}
}

// TestAgentConflictPolicies_NotApplicableToSessionPath verifies that conflict resolution
// policies have no effect in single-writer mode where sessions don't attempt merges.
func TestAgentConflictPolicies_NotApplicableToSessionPath(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "refine")
	session.BranchName = "gromit/refine-222"

	var (
		mergeCalls   int
		resolveCalls int
	)

	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(branch string) error {
				mergeCalls++
				// Single-writer: this should never be called
				t.Fatalf("regression: merge called in session path")
				return nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{}, nil
	}, func(gotMainDir, gotSessionDir string) error {
		if gotMainDir != mainDir {
			t.Fatalf("cleanup mainDir = %q, want %q", gotMainDir, mainDir)
		}
		return nil
	})

	// Conflict settings don't matter in single-writer model
	result, err := runWithSessionWorktreeWithConflictSettings(gromitDir, "refine", sessionConflictSettings{
		Policy:   "agent",
		RetryCap: 2,
		AgentConflictResolver: func(sessionDir, branch string, attempt int) error {
			resolveCalls++
			return nil
		},
	}, func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktreeWithConflictSettings() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if mergeCalls != 0 {
		t.Fatalf("merge calls = %d, want 0 (no merges in session path)", mergeCalls)
	}
	if resolveCalls != 0 {
		t.Fatalf("resolver calls = %d, want 0 (no conflict handling in session path)", resolveCalls)
	}
}

// TestSessionCompletionImmediatelyQueues_NoConflictRetry verifies that session
// completion queues branch immediately without attempting merge. Conflict resolution
// will occur later in coordinator path, not in session.
func TestSessionCompletionImmediatelyQueues_NoConflictRetry(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "review")
	session.BranchName = "gromit/review-234"

	cleanupGit := overrideGitRun(autoCommitGitRun(""))
	t.Cleanup(cleanupGit)

	var events []string

	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(branch string) error {
				// Single-writer: no merge in session
				t.Fatalf("regression: session attempted merge of %q", branch)
				return nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			AddPendingWorktreeBranchFn: func(branch string) error {
				if branch != session.BranchName {
					t.Fatalf("add branch = %q, want %q", branch, session.BranchName)
				}
				events = append(events, "add")
				return nil
			},
		}, nil
	}, func(gotMainDir, gotSessionDir string) error {
		if gotMainDir != mainDir {
			t.Fatalf("cleanup mainDir = %q, want %q", gotMainDir, mainDir)
		}
		if gotSessionDir != session.WorktreeDir {
			t.Fatalf("cleanup sessionDir = %q, want %q", gotSessionDir, session.WorktreeDir)
		}
		events = append(events, "cleanup")
		return nil
	})

	_, err := runWithSessionWorktreeWithConflictSettings(gromitDir, "review", sessionConflictSettings{
		Policy:   "agent",
		RetryCap: 1,
		AgentConflictResolver: func(sessionDir, branch string, attempt int) error {
			// Never called in single-writer session path
			t.Fatal("conflict resolver should not be invoked in session path")
			return nil
		},
	}, func(string) error { return nil })
	if err != nil {
		t.Fatalf("runWithSessionWorktreeWithConflictSettings() error = %v", err)
	}

	// Single-writer: just add and cleanup, return for coordinator
	want := []string{"add", "cleanup"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

// TestSessionSuccessDoesNotDependOnConflictPolicy verifies conflict policies
// (manual/agent) have no effect in session path under single-writer model.
func TestSessionSuccessDoesNotDependOnConflictPolicy(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	_, gromitDir, session := setupRunWithSessionWorktreeTest(t, "debug")
	session.BranchName = "gromit/debug-333"

	var (
		mergeCalls   int
		resolveCalls int
	)

	withInteractiveWorktreeFactories(t, func(string) (sessionWorktreeCreator, error) {
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(string) error {
				mergeCalls++
				// Single-writer: sessions don't merge
				t.Fatal("regression: session attempted merge")
				return nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{}, nil
	}, func(string, string) error {
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
	if err != nil {
		t.Fatalf("runWithSessionWorktreeWithConflictSettings() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil session result")
	}
	if mergeCalls != 0 {
		t.Fatalf("merge calls = %d, want 0 (no merge in session path)", mergeCalls)
	}
	if resolveCalls != 0 {
		t.Fatalf("resolver calls = %d, want 0 (conflict resolution in coordinator, not session)", resolveCalls)
	}
}

// TestConcurrentSessions_BothQueueWithoutConflict verifies single-writer isolation:
// multiple concurrent sessions queue their branches independently without merge contention.
func TestConcurrentSessions_BothQueueWithoutConflict(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	cleanupGit := overrideGitRun(autoCommitGitRun(""))
	t.Cleanup(cleanupGit)
	cleanupStore := overrideQueueStore(func(entry integrationqueue.Entry) error { return nil })
	t.Cleanup(cleanupStore)

	mainDir := t.TempDir()
	gromitDir := filepath.Join(mainDir, ".gromit")
	sessionA := &worktree.SessionWorktree{
		BranchName:  "gromit/dual-session-a",
		WorktreeDir: filepath.Join(mainDir, "session-a"),
	}
	sessionB := &worktree.SessionWorktree{
		BranchName:  "gromit/dual-session-b",
		WorktreeDir: filepath.Join(mainDir, "session-b"),
	}

	var (
		recorded = map[string]struct{}{}
		recordMu sync.Mutex
	)

	manager := &mockSessionWorktreeCreator{
		CreateSessionWorktreeFn: func(command string) (*worktree.SessionWorktree, error) {
			switch command {
			case "dual-session-a":
				return sessionA, nil
			case "dual-session-b":
				return sessionB, nil
			default:
				t.Fatalf("unexpected command %q", command)
				return nil, nil
			}
		},
		MergeBackFn: func(branch string) error {
			// Single-writer: no merges in session path
			t.Fatalf("regression: session attempted merge of %q", branch)
			return nil
		},
	}

	stateRecorder := &mockPendingBranchRecorder{
		AddPendingWorktreeBranchFn: func(branch string) error {
			recordMu.Lock()
			recorded[branch] = struct{}{}
			recordMu.Unlock()
			return nil
		},
		RemovePendingWorktreeBranchFn: func(branch string) error {
			// Branches remain pending for coordinator
			t.Fatalf("regression: session attempted to remove pending branch %s", branch)
			return nil
		},
	}

	withInteractiveWorktreeFactories(t, func(gotMainDir string) (sessionWorktreeCreator, error) {
		if gotMainDir != mainDir {
			t.Fatalf("mainDir = %q, want %q", gotMainDir, mainDir)
		}
		return manager, nil
	}, func(gotGromitDir string) (pendingBranchRecorder, error) {
		if gotGromitDir != gromitDir {
			t.Fatalf("gromitDir = %q, want %q", gotGromitDir, gromitDir)
		}
		return stateRecorder, nil
	}, func(gotMainDir, gotSessionDir string) error {
		return nil
	})

	resCh := make(chan struct {
		branch  string
		session *worktree.SessionWorktree
		err     error
	}, 2)

	runSession := func(command string) {
		go func() {
			session, err := runWithSessionWorktreeWithConflictSettings(gromitDir, command, sessionConflictSettings{}, func(string) error {
				return nil
			})
			resCh <- struct {
				branch  string
				session *worktree.SessionWorktree
				err     error
			}{branch: command, session: session, err: err}
		}()
	}

	runSession("dual-session-a")
	runSession("dual-session-b")

	// Collect both results
	for i := 0; i < 2; i++ {
		result := <-resCh
		if result.session == nil {
			t.Fatalf("session result is nil for command %s", result.branch)
		}
		if result.err != nil {
			t.Fatalf("unexpected error for %s: %v", result.branch, result.err)
		}
	}

	// Verify both branches queued independently
	recordMu.Lock()
	if len(recorded) != 2 {
		t.Fatalf("recorded pending branches %v, want 2", recorded)
	}
	for branch := range recorded {
		if branch != sessionA.BranchName && branch != sessionB.BranchName {
			t.Fatalf("unexpected branch recorded: %s", branch)
		}
	}
	recordMu.Unlock()
}

// NOTE: Conflict handoff and merge state cleanup tests are legacy and no longer applicable
// in single-writer model where sessions never attempt merges. These helper functions remain
// for compatibility but are not invoked from the session path. Coordinator-path tests will
// verify these behaviors when conflict handling is implemented in coordinator.

type syncBarrier struct {
	total   int
	count   int
	release chan struct{}
	mu      sync.Mutex
}

func newSyncBarrier(total int) *syncBarrier {
	if total <= 0 {
		total = 1
	}
	return &syncBarrier{
		total:   total,
		release: make(chan struct{}),
	}
}

func (b *syncBarrier) Wait() {
	b.mu.Lock()
	if b.count++; b.count == b.total {
		close(b.release)
	}
	release := b.release
	b.mu.Unlock()
	<-release
}

// TestSingleWriterInvariant_SessionsDoNotMergeToMainDirectly is a regression guard asserting
// that interactive sessions do NOT directly merge branches to main. Sessions must instead
// queue branches for coordinator-mediated integration. This enforces single-writer semantics
// where only the Orchestrator/coordinator path can mutate main.
func TestSingleWriterInvariant_SessionsDoNotMergeToMainDirectly(t *testing.T) {
	// Not parallel: withInteractiveWorktreeFactories mutates package-level globals.
	mainDir, gromitDir, session := setupRunWithSessionWorktreeTest(t, "session-no-merge")
	session.BranchName = "gromit/session-no-merge"

	withInteractiveWorktreeFactories(t, func(gotMainDir string) (sessionWorktreeCreator, error) {
		if gotMainDir != mainDir {
			t.Fatalf("mainDir = %q, want %q", gotMainDir, mainDir)
		}
		return &mockSessionWorktreeCreator{
			CreateSessionWorktreeFn: func(string) (*worktree.SessionWorktree, error) {
				return session, nil
			},
			MergeBackFn: func(branch string) error {
				// REGRESSION GUARD: Sessions should NOT be able to merge branches to main.
				// This merge attempt indicates a violation of single-writer semantics.
				// Only coordinator path should perform main integration.
				t.Fatalf("regression: session attempted direct merge to main for branch %q; "+
					"single-writer policy requires coordinator-mediated integration", branch)
				return nil
			},
		}, nil
	}, func(string) (pendingBranchRecorder, error) {
		return &mockPendingBranchRecorder{
			AddPendingWorktreeBranchFn: func(branch string) error {
				// Branches should be recorded for later coordinator processing
				return nil
			},
		}, nil
	}, func(string, string) error { return nil })

	// When coordinator pattern is fully implemented, sessions should NOT attempt merge.
	// This test will fail if the session path still calls MergeBack.
	// For now, runWithSessionWorktree will trigger the t.Fatalf above if merge is attempted.
	_, _ = runWithSessionWorktree(gromitDir, "session-no-merge", func(string) error { return nil })
}
