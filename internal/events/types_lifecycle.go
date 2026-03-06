package events

import "time"

// RunStartEvent is emitted when a run begins.
type RunStartEvent struct {
	MaxIterations int
	TimeBudget    time.Duration
	DryRun        bool
	TimeMixin
}

func (e *RunStartEvent) EventType() string {
	return "run_start"
}

// RunCompleteEvent is emitted when a run completes.
type RunCompleteEvent struct {
	IterationsCompleted int
	Reason              string
	TimeMixin
}

func (e *RunCompleteEvent) EventType() string {
	return "run_complete"
}

// IterationStartEvent is emitted at the start of each iteration.
type IterationStartEvent struct {
	Iteration int
	BeadID    string
	BeadTitle string
	TimeMixin
}

func (e *IterationStartEvent) EventType() string {
	return "iteration_start"
}

// IterationCompleteEvent is emitted when an iteration completes.
type IterationCompleteEvent struct {
	Iteration int
	BeadID    string
	Success   bool
	Duration  time.Duration
	TimeMixin
}

func (e *IterationCompleteEvent) EventType() string {
	return "iteration_complete"
}

// BeadCompleteEvent is emitted when a bead is closed as done.
type BeadCompleteEvent struct {
	BeadID       string
	BeadTitle    string
	Duration     time.Duration
	Model        string
	CostUSD      float64
	InputTokens  int
	OutputTokens int
	TimeMixin
}

func (e *BeadCompleteEvent) EventType() string {
	return "bead_complete"
}

// BeadFailedEvent is emitted when a bead fails after exhausting retries.
type BeadFailedEvent struct {
	BeadID    string
	BeadTitle string
	Error     string
	TimeMixin
}

func (e *BeadFailedEvent) EventType() string {
	return "bead_failed"
}

// BeadStuckEvent is emitted when a bead is marked as stuck.
type BeadStuckEvent struct {
	BeadID    string
	BeadTitle string
	Reason    string
	TimeMixin
}

func (e *BeadStuckEvent) EventType() string {
	return "bead_stuck"
}

// BeadUnstickedEvent is emitted when a bead is manually or automatically unstuck.
type BeadUnstickedEvent struct {
	BeadID string
	Reason string
	TimeMixin
}

func (e *BeadUnstickedEvent) EventType() string {
	return "bead_unsticked"
}

// BeadSkippedEvent is emitted when a bead is skipped.
type BeadSkippedEvent struct {
	BeadID string
	Reason string
	TimeMixin
}

func (e *BeadSkippedEvent) EventType() string {
	return "bead_skipped"
}
