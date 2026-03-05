package adapter

import "context"

// GitAdapter performs git operations required by the run loop.
type GitAdapter interface {
	Checkout(ctx context.Context, specID string) (worktree string, err error)
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
	PresentSummary(ctx context.Context, specID, summary string) error
}

// AdapterSet aggregates the adapters consumed by the run loop.
type AdapterSet struct {
	Git         GitAdapter
	LLM         LLMAdapter
	TaskTracker TaskTrackerAdapter
	Presenter   PresenterAdapter
}
