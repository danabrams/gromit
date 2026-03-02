package events

import "time"

// RunStartEvent is emitted when a run begins.
type RunStartEvent struct {
	MaxIterations int
	TimeBudget    time.Duration
	DryRun        bool
	Time          time.Time
}

func (e *RunStartEvent) EventType() string {
	return "run_start"
}

func (e *RunStartEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// RunCompleteEvent is emitted when a run completes.
type RunCompleteEvent struct {
	IterationsCompleted int
	Reason              string
	Time                time.Time
}

func (e *RunCompleteEvent) EventType() string {
	return "run_complete"
}

func (e *RunCompleteEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// IterationStartEvent is emitted at the start of each iteration.
type IterationStartEvent struct {
	Iteration int
	BeadID    string
	BeadTitle string
	Time      time.Time
}

func (e *IterationStartEvent) EventType() string {
	return "iteration_start"
}

func (e *IterationStartEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// IterationCompleteEvent is emitted when an iteration completes.
type IterationCompleteEvent struct {
	Iteration int
	BeadID    string
	Success   bool
	Duration  time.Duration
	Time      time.Time
}

func (e *IterationCompleteEvent) EventType() string {
	return "iteration_complete"
}

func (e *IterationCompleteEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// BeadCompleteEvent is emitted when a bead is closed as done.
type BeadCompleteEvent struct {
	BeadID    string
	BeadTitle string
	Duration  time.Duration
	Time      time.Time
}

func (e *BeadCompleteEvent) EventType() string {
	return "bead_complete"
}

func (e *BeadCompleteEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// BeadFailedEvent is emitted when a bead fails after exhausting retries.
type BeadFailedEvent struct {
	BeadID    string
	BeadTitle string
	Error     string
	Time      time.Time
}

func (e *BeadFailedEvent) EventType() string {
	return "bead_failed"
}

func (e *BeadFailedEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// BeadStuckEvent is emitted when a bead is marked as stuck.
type BeadStuckEvent struct {
	BeadID    string
	BeadTitle string
	Reason    string
	Time      time.Time
}

func (e *BeadStuckEvent) EventType() string {
	return "bead_stuck"
}

func (e *BeadStuckEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// BeadSkippedEvent is emitted when a bead is skipped.
type BeadSkippedEvent struct {
	BeadID string
	Reason string
	Time   time.Time
}

func (e *BeadSkippedEvent) EventType() string {
	return "bead_skipped"
}

func (e *BeadSkippedEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}
