package git

import (
	"context"
	"testing"
)

func TestGitInterface(t *testing.T) {
	var adapter Git = dummyGit{}
	if _, err := adapter.CreateWorktree(context.Background(), CreateWorktreeRequest{SpecID: "spec-test"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiffResponseIncludesSummary(t *testing.T) {
	resp := DiffResponse{Diff: "diff", Summary: "summary"}
	if resp.Summary != "summary" {
		t.Fatalf("expected summary 'summary', got %q", resp.Summary)
	}
}

type dummyGit struct{}

func (dummyGit) CreateWorktree(context.Context, CreateWorktreeRequest) (CreateWorktreeResponse, error) {
	return CreateWorktreeResponse{Worktree: "tmp/spec-test"}, nil
}

func (dummyGit) RemoveWorktree(context.Context, RemoveWorktreeRequest) (RemoveWorktreeResponse, error) {
	return RemoveWorktreeResponse{}, nil
}

func (dummyGit) Commit(context.Context, CommitRequest) (CommitResponse, error) {
	return CommitResponse{CommitHash: "abc123"}, nil
}

func (dummyGit) Diff(context.Context, DiffRequest) (DiffResponse, error) {
	return DiffResponse{Diff: "diff", Summary: "summary"}, nil
}
