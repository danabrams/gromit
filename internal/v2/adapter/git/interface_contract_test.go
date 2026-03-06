package git

import (
	"context"
	"testing"
)

func TestGitContract(t *testing.T) {
	var _ interface {
		CreateWorktree(context.Context, GitCreateWorktreeRequest) (GitCreateWorktreeResponse, error)
		RemoveWorktree(context.Context, GitRemoveWorktreeRequest) (GitRemoveWorktreeResponse, error)
		Commit(context.Context, GitCommitRequest) (GitCommitResponse, error)
		Diff(context.Context, GitDiffRequest) (GitDiffResponse, error)
	} = (Git)(nil)
}
