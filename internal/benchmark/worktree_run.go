package benchmark

import "context"

type BaseCommitResolver interface {
	ResolveBaseCommit(ctx context.Context, baseCommitHint string) (string, error)
}

type ModeWorktreeRequest struct {
	Mode          string
	BaseCommit    string
	SelectedBeads []string
	Overlay       ModeOverlay
}

type ModeWorktreeRun struct {
	Mode    string
	Cleanup func() error
}

type ModeWorktreeRunner interface {
	RunMode(ctx context.Context, req ModeWorktreeRequest) (ModeWorktreeRun, error)
}
