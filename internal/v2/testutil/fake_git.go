package testutil

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
)

// FakeGit records checkout, diff, and worktree creation calls.
// Error injection: set any of the *Err fields to make the corresponding
// method return that error instead of succeeding.
type FakeGit struct {
	mu                  sync.Mutex
	WorktreeRoot        string
	DiffOutput          string
	CheckoutCalls       []string
	DiffCalls           []string
	CreateWorktreeCalls []string
	CommitCalls         []string
	CommitMessages      []string
	RemoveWorktreeCalls []string
	StatusCalls         []string

	// Error injection fields — when non-nil the corresponding method
	// returns the configured error.
	CheckoutErr        error
	DiffErr            error
	CreateWorktreeErr  error
	CommitErr          error
	RemoveWorktreeErr  error
	StatusErr          error
}

// NewFakeGit returns a fake Git adapter with defaults.
func NewFakeGit() *FakeGit {
	return &FakeGit{WorktreeRoot: "/tmp"}
}

// Checkout records the requested spec and returns a deterministic worktree path.
func (f *FakeGit) Checkout(_ context.Context, specID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.CheckoutCalls = append(f.CheckoutCalls, specID)
	if f.CheckoutErr != nil {
		return "", f.CheckoutErr
	}
	worktree := filepath.Join(f.baseRoot(), specID)
	return worktree, nil
}

// Diff records the worktree and returns the configured diff output.
func (f *FakeGit) Diff(_ context.Context, worktree string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.DiffCalls = append(f.DiffCalls, worktree)
	if f.DiffErr != nil {
		return "", f.DiffErr
	}
	return f.DiffOutput, nil
}

// CreateIsolatedWorktree records the spec and returns a deterministic path.
func (f *FakeGit) CreateIsolatedWorktree(_ context.Context, specID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.CreateWorktreeCalls = append(f.CreateWorktreeCalls, specID)
	if f.CreateWorktreeErr != nil {
		return "", f.CreateWorktreeErr
	}
	worktree := filepath.Join(f.baseRoot(), specID)
	return worktree, nil
}

func (f *FakeGit) Commit(_ context.Context, worktree, message string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CommitCalls = append(f.CommitCalls, worktree)
	f.CommitMessages = append(f.CommitMessages, message)
	if f.CommitErr != nil {
		return "", f.CommitErr
	}
	return "fake-commit", nil
}

func (f *FakeGit) RemoveWorktree(_ context.Context, worktree string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RemoveWorktreeCalls = append(f.RemoveWorktreeCalls, worktree)
	if f.RemoveWorktreeErr != nil {
		return f.RemoveWorktreeErr
	}
	return nil
}

func (f *FakeGit) Status(_ context.Context, worktree string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.StatusCalls = append(f.StatusCalls, worktree)
	if f.StatusErr != nil {
		return "", f.StatusErr
	}
	return "", nil
}

func (f *FakeGit) baseRoot() string {
	root := strings.TrimSpace(f.WorktreeRoot)
	if root == "" {
		return "/tmp"
	}
	return root
}
