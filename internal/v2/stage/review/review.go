package review

import "context"

// GitDiffFn returns the git diff for the current worktree.
type GitDiffFn func(ctx context.Context, worktree string) (string, error)
