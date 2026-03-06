package events

// SpecStartedEvent is emitted when a spec loop begins execution.
type SpecStartedEvent struct {
	SpecID   string
	Worktree string
	TimeMixin
}

func (e *SpecStartedEvent) EventType() string {
	return "spec_started"
}

// SpecCompletedEvent is emitted when a spec loop finishes.
type SpecCompletedEvent struct {
	SpecID        string
	Worktree      string
	Success       bool
	FailureReason string
	TimeMixin
}

func (e *SpecCompletedEvent) EventType() string {
	return "spec_completed"
}
