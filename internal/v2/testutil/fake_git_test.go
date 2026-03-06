package testutil

import (
    "context"
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
