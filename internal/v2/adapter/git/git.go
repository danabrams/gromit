package git

import "context"

// CreateWorktreeRequest describes the inputs required to make a new worktree.
type CreateWorktreeRequest struct {
	SpecID       string
	Reference    string
	WorktreeRoot string
}

// CreateWorktreeResponse describes the created worktree path.
type CreateWorktreeResponse struct {
	Worktree string
}

// RemoveWorktreeRequest describes a request to remove an existing worktree.
type RemoveWorktreeRequest struct {
	Worktree string
	Force    bool
}

// RemoveWorktreeResponse communicates whether the removal happened.
type RemoveWorktreeResponse struct {
	Removed bool
}

// CommitRequest describes the inputs to commit work within a worktree.
type CommitRequest struct {
	Worktree string
	Message  string
	Amend    bool
}

// CommitResponse reports the resulting commit hash.
type CommitResponse struct {
	CommitHash string
}

// DiffRequest describes the worktree to diff.
type DiffRequest struct {
	Worktree string
}

// DiffResponse carries the diff output.
type DiffResponse struct {
	Diff string
}

// Git describes higher-level git operations for the run loop.
type Git interface {
	CreateWorktree(ctx context.Context, req CreateWorktreeRequest) (CreateWorktreeResponse, error)
	RemoveWorktree(ctx context.Context, req RemoveWorktreeRequest) (RemoveWorktreeResponse, error)
	Commit(ctx context.Context, req CommitRequest) (CommitResponse, error)
	Diff(ctx context.Context, req DiffRequest) (DiffResponse, error)
}
