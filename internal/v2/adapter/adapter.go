package adapter

import (
	"context"

	"github.com/danabrams/gromit/internal/v2/presentation"
)

// GitAdapter performs git operations required by the run loop.
type GitAdapter interface {
	Checkout(ctx context.Context, specID string) (worktree string, err error)
	Diff(ctx context.Context, worktree string) (string, error)
	Commit(ctx context.Context, worktree, message string) (string, error)
	RemoveWorktree(ctx context.Context, worktree string) error
	Status(ctx context.Context, worktree string) (string, error)
}

// LLMAdapter synthesizes the plan for a spec.
type LLMAdapter interface {
	GeneratePlan(ctx context.Context, specID string) (plan string, err error)
}

// TaskTrackerAdapter records the generated plan into the task tracker.
type TaskTrackerAdapter interface {
	RecordPlan(ctx context.Context, specID, plan string) error
}

// PresenterAdapter surfaces completed specs to product owners.
type PresenterAdapter interface {
	PresentSummary(ctx context.Context, specID string, summary presentation.PresentationSummary) error
}

// AdapterSet aggregates the adapters consumed by the run loop.
type AdapterSet struct {
	Git         GitAdapter
	LLM         LLMAdapter
	TaskTracker TaskTrackerAdapter
	Presenter   PresenterAdapter
}
