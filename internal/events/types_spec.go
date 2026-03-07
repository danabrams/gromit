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

// SpecFailedEvent is emitted when a spec run cannot be remediated.
type SpecFailedEvent struct {
	SpecID        string
	Worktree      string
	FailureReason string
	TimeMixin
}

func (e *SpecFailedEvent) EventType() string {
	return "spec_failed"
}

// PlanResumedEvent is emitted when an existing plan is reused on rerun.
type PlanResumedEvent struct {
	SpecID string
	Path   string
	TimeMixin
}

func (e *PlanResumedEvent) EventType() string {
	return "plan_resumed"
}

// DecomposeResumedEvent is emitted when existing beads are reused on rerun.
type DecomposeResumedEvent struct {
	SpecID    string
	BeadCount int
	TimeMixin
}

func (e *DecomposeResumedEvent) EventType() string {
	return "decompose_resumed"
}
