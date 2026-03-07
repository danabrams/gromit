package adapter

import (
	"context"

	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/presentation"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
)

// LogEntry represents a single commit in a git log.
type LogEntry struct {
	Hash    string
	Message string
}

// GitAdapter performs git operations required by the run loop.
type GitAdapter interface {
	Checkout(ctx context.Context, specID string) (worktree string, err error)
	Diff(ctx context.Context, worktree string) (string, error)
	Commit(ctx context.Context, worktree, message string) (string, error)
	RemoveWorktree(ctx context.Context, worktree string) error
	Status(ctx context.Context, worktree string) (string, error)
	Log(ctx context.Context, worktree string, n int) ([]LogEntry, error)
}

// LLMAdapter provides LLM operations for the run loop.
// It embeds llm.LLMProvider (Invoke, StreamInvoke) and adds GeneratePlan.
type LLMAdapter interface {
	llmtypes.LLMProvider
	GeneratePlan(ctx context.Context, specID string) (plan string, err error)
}

// TaskTrackerAdapter provides task-tracker operations for the run loop.
type TaskTrackerAdapter interface {
	trackertypes.TaskTracker
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
