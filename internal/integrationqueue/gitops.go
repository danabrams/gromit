package integrationqueue

import "context"

// GitOps abstracts the git operations required by the integration coordinator.
type GitOps interface {
	// FetchAndRebase brings the candidate branch up to date with the base branch.
	FetchAndRebase(ctx context.Context, entry Entry) error

	// MergeToMain applies the candidate branch onto main.
	MergeToMain(ctx context.Context, entry Entry) error

	// Push syncs the updated main branch to the remote repository.
	Push(ctx context.Context) error

	// Cleanup removes any temporary metadata left behind after integration.
	Cleanup(ctx context.Context, entry Entry) error
}
