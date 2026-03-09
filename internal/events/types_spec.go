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

// SpecVerdictEvent records the combined acceptance and spec review verdict for a spec run.
type SpecVerdictEvent struct {
	SpecID             string `json:"spec_id"`
	Worktree           string `json:"worktree"`
	AcceptDecision     string `json:"accept_decision"`
	SpecReviewDecision string `json:"spec_review_decision"`
	SpecReviewVerdict  string `json:"spec_review_verdict"`
	Success            bool   `json:"success"`
	TimeMixin
}

func (e *SpecVerdictEvent) EventType() string {
	return "spec_verdict"
}

// PlanResumedEvent is emitted when an existing plan is reused on rerun.
type PlanResumedEvent struct {
	SpecID string `json:"spec_id"`
	Path   string `json:"path"`
	TimeMixin
}

func (e *PlanResumedEvent) EventType() string {
	return "plan_resumed"
}

// DecomposeResumedEvent is emitted when existing beads are reused on rerun.
type DecomposeResumedEvent struct {
	SpecID    string `json:"spec_id"`
	BeadCount int    `json:"bead_count"`
	TimeMixin
}

func (e *DecomposeResumedEvent) EventType() string {
	return "decompose_resumed"
}
