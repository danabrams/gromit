package git

// GitCreateWorktreeRequest describes the inputs required to make a new worktree.
type GitCreateWorktreeRequest struct {
	SpecID       string
	Reference    string
	WorktreeRoot string
}

// GitCreateWorktreeResponse describes the created worktree path.
type GitCreateWorktreeResponse struct {
	Worktree string
}

// CreateWorktreeRequest is retained for backwards compatibility with existing callers.
type CreateWorktreeRequest = GitCreateWorktreeRequest

// CreateWorktreeResponse is retained for backwards compatibility with existing callers.
type CreateWorktreeResponse = GitCreateWorktreeResponse

// RemoveWorktreeRequest describes a request to remove an existing worktree.
type GitRemoveWorktreeRequest struct {
	Worktree string
	Force    bool
}

// GitRemoveWorktreeResponse communicates whether the removal happened.
type GitRemoveWorktreeResponse struct {
	Removed bool
}

// RemoveWorktreeRequest is retained for backwards compatibility with existing callers.
type RemoveWorktreeRequest = GitRemoveWorktreeRequest

// RemoveWorktreeResponse is retained for backwards compatibility with existing callers.
type RemoveWorktreeResponse = GitRemoveWorktreeResponse

// CommitRequest describes the inputs to commit work within a worktree.
// GitCommitRequest describes the inputs to commit work within a worktree.
type GitCommitRequest struct {
	Worktree string
	Message  string
	Amend    bool
}

// GitCommitResponse reports the resulting commit hash.
type GitCommitResponse struct {
	CommitHash string
}

// CommitRequest is retained for backwards compatibility with existing callers.
type CommitRequest = GitCommitRequest

// CommitResponse is retained for backwards compatibility with existing callers.
type CommitResponse = GitCommitResponse

// DiffRequest describes the worktree to diff.
// GitDiffRequest describes the worktree to diff.
type GitDiffRequest struct {
	Worktree string
	Base     string
}

// GitDiffResponse carries the diff output.
type GitDiffResponse struct {
	Diff    string
	Summary string
}

// DiffRequest is retained for backwards compatibility with existing callers.
type DiffRequest = GitDiffRequest

// DiffResponse is retained for backwards compatibility with existing callers.
type DiffResponse = GitDiffResponse
