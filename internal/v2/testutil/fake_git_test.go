package testutil

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestFakeGitTracksCheckoutAndDiff(t *testing.T) {
	t.Parallel()

	fake := NewFakeGit()
	fake.WorktreeRoot = "/tmp/worktrees"
	fake.DiffOutput = "diff-blob"

	ctx := context.Background()
	worktree, err := fake.Checkout(ctx, "spec-123")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}
	expected := filepath.Join(fake.WorktreeRoot, "spec-123")
	if worktree != expected {
		t.Fatalf("unexpected worktree: %q", worktree)
	}
	if len(fake.CheckoutCalls) != 1 || fake.CheckoutCalls[0] != "spec-123" {
		t.Fatalf("checkout calls = %v", fake.CheckoutCalls)
	}

	diff, err := fake.Diff(ctx, worktree)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff != fake.DiffOutput {
		t.Fatalf("unexpected diff: %q", diff)
	}
	if len(fake.DiffCalls) != 1 || fake.DiffCalls[0] != worktree {
		t.Fatalf("diff calls = %v", fake.DiffCalls)
	}

	isolated, err := fake.CreateIsolatedWorktree(ctx, "spec-xyz")
	if err != nil {
		t.Fatalf("CreateIsolatedWorktree failed: %v", err)
	}
	if isolated != filepath.Join(fake.WorktreeRoot, "spec-xyz") {
		t.Fatalf("unexpected isolated worktree: %q", isolated)
	}
	if len(fake.CreateWorktreeCalls) != 1 || fake.CreateWorktreeCalls[0] != "spec-xyz" {
		t.Fatalf("unexpected create calls: %v", fake.CreateWorktreeCalls)
	}
}

func TestFakeGitErrorInjection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("CheckoutErr", func(t *testing.T) {
		t.Parallel()
		injected := errors.New("checkout boom")
		fake := NewFakeGit()
		fake.CheckoutErr = injected

		_, err := fake.Checkout(ctx, "spec-1")
		if !errors.Is(err, injected) {
			t.Fatalf("Checkout error = %v, want %v", err, injected)
		}
		if len(fake.CheckoutCalls) != 1 {
			t.Fatalf("call should still be recorded, got %d", len(fake.CheckoutCalls))
		}
	})

	t.Run("DiffErr", func(t *testing.T) {
		t.Parallel()
		injected := errors.New("diff boom")
		fake := NewFakeGit()
		fake.DiffErr = injected

		_, err := fake.Diff(ctx, "/tmp/wt")
		if !errors.Is(err, injected) {
			t.Fatalf("Diff error = %v, want %v", err, injected)
		}
		if len(fake.DiffCalls) != 1 {
			t.Fatalf("call should still be recorded, got %d", len(fake.DiffCalls))
		}
	})

	t.Run("CreateWorktreeErr", func(t *testing.T) {
		t.Parallel()
		injected := errors.New("create-worktree boom")
		fake := NewFakeGit()
		fake.CreateWorktreeErr = injected

		_, err := fake.CreateIsolatedWorktree(ctx, "spec-2")
		if !errors.Is(err, injected) {
			t.Fatalf("CreateIsolatedWorktree error = %v, want %v", err, injected)
		}
		if len(fake.CreateWorktreeCalls) != 1 {
			t.Fatalf("call should still be recorded, got %d", len(fake.CreateWorktreeCalls))
		}
	})

	t.Run("CommitErr", func(t *testing.T) {
		t.Parallel()
		injected := errors.New("commit boom")
		fake := NewFakeGit()
		fake.CommitErr = injected

		_, err := fake.Commit(ctx, "/tmp/wt", "msg")
		if !errors.Is(err, injected) {
			t.Fatalf("Commit error = %v, want %v", err, injected)
		}
		if len(fake.CommitCalls) != 1 {
			t.Fatalf("call should still be recorded, got %d", len(fake.CommitCalls))
		}
	})

	t.Run("RemoveWorktreeErr", func(t *testing.T) {
		t.Parallel()
		injected := errors.New("remove-worktree boom")
		fake := NewFakeGit()
		fake.RemoveWorktreeErr = injected

		err := fake.RemoveWorktree(ctx, "/tmp/wt")
		if !errors.Is(err, injected) {
			t.Fatalf("RemoveWorktree error = %v, want %v", err, injected)
		}
		if len(fake.RemoveWorktreeCalls) != 1 {
			t.Fatalf("call should still be recorded, got %d", len(fake.RemoveWorktreeCalls))
		}
	})

	t.Run("StatusErr", func(t *testing.T) {
		t.Parallel()
		injected := errors.New("status boom")
		fake := NewFakeGit()
		fake.StatusErr = injected

		_, err := fake.Status(ctx, "/tmp/wt")
		if !errors.Is(err, injected) {
			t.Fatalf("Status error = %v, want %v", err, injected)
		}
		if len(fake.StatusCalls) != 1 {
			t.Fatalf("call should still be recorded, got %d", len(fake.StatusCalls))
		}
	})

	t.Run("NilErrorsPreserveDefaultBehavior", func(t *testing.T) {
		t.Parallel()
		fake := NewFakeGit()
		fake.WorktreeRoot = "/tmp/test"

		wt, err := fake.Checkout(ctx, "ok")
		if err != nil {
			t.Fatalf("Checkout should succeed: %v", err)
		}
		if wt != filepath.Join("/tmp/test", "ok") {
			t.Fatalf("unexpected worktree: %q", wt)
		}

		if err := fake.RemoveWorktree(ctx, wt); err != nil {
			t.Fatalf("RemoveWorktree should succeed: %v", err)
		}

		_, err = fake.Status(ctx, wt)
		if err != nil {
			t.Fatalf("Status should succeed: %v", err)
		}

		hash, err := fake.Commit(ctx, wt, "msg")
		if err != nil {
			t.Fatalf("Commit should succeed: %v", err)
		}
		if hash != "fake-commit" {
			t.Fatalf("unexpected hash: %q", hash)
		}
	})
}
