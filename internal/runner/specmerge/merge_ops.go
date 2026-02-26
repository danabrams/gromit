package specmerge

import (
	"context"
	"errors"
	"fmt"
)

// FinalizeDependencies holds the dependencies required by FinalizeSpecBranch.
type FinalizeDependencies struct {
	Git              GitOps
	ConflictResolver ConflictResolver
	MainBranch       string
}

// GitOps exposes the git operations needed to finalize a spec branch.
type GitOps interface {
	RebaseOnto(ctx context.Context, branch, onto string) error
	FastForwardMerge(ctx context.Context, branch string) error
	DeleteBranch(ctx context.Context, branch string) error
}

// ConflictResolver is invoked when FinalizeSpecBranch encounters a conflict.
type ConflictResolver interface {
	Resolve(ctx context.Context, branch string, cause error) error
}

// ConflictError represents a git conflict detected during rebase or merge.
type ConflictError struct {
	Operation string
	Err       error
}

func (e *ConflictError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s conflict: %v", e.Operation, e.Err)
}

func (e *ConflictError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

const defaultMainBranch = "main"

// FinalizeSpecBranch rebases the spec branch onto main, merges it, and deletes it.
func FinalizeSpecBranch(ctx context.Context, deps FinalizeDependencies, branch string) error {
	if deps.Git == nil {
		return fmt.Errorf("git operations are not configured")
	}
	if branch == "" {
		return fmt.Errorf("branch name is required")
	}

	mainBranch := deps.MainBranch
	if mainBranch == "" {
		mainBranch = defaultMainBranch
	}

	if err := executeWithConflictResolution(ctx, deps, branch, func() error {
		return deps.Git.RebaseOnto(ctx, branch, mainBranch)
	}); err != nil {
		return err
	}
	if err := executeWithConflictResolution(ctx, deps, branch, func() error {
		return deps.Git.FastForwardMerge(ctx, branch)
	}); err != nil {
		return err
	}
	if err := deps.Git.DeleteBranch(ctx, branch); err != nil {
		return err
	}
	return nil
}

func executeWithConflictResolution(ctx context.Context, deps FinalizeDependencies, branch string, op func() error) error {
	err := op()
	if err == nil {
		return nil
	}
	if deps.ConflictResolver == nil {
		return err
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		return err
	}
	if resolveErr := deps.ConflictResolver.Resolve(ctx, branch, err); resolveErr != nil {
		return fmt.Errorf("resolve conflict for branch %s: %w", branch, resolveErr)
	}
	return op()
}
