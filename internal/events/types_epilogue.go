package events

import "time"

// EpilogueStartEvent is emitted when the epilogue stage begins.
type EpilogueStartEvent struct {
	BeadID   string
	Iteration int
	Success  bool // Whether build succeeded
	Time     time.Time
}

func (e *EpilogueStartEvent) EventType() string {
	return "epilogue_start"
}

func (e *EpilogueStartEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// EpilogueCompleteEvent is emitted when the epilogue stage completes.
type EpilogueCompleteEvent struct {
	BeadID  string
	Success bool
	Time    time.Time
}

func (e *EpilogueCompleteEvent) EventType() string {
	return "epilogue_complete"
}

func (e *EpilogueCompleteEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// BeadCloseEvent is emitted when a bead is closed successfully.
type BeadCloseEvent struct {
	BeadID string
	Time   time.Time
}

func (e *BeadCloseEvent) EventType() string {
	return "bead_close"
}

func (e *BeadCloseEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// BeadCleanupEvent is emitted when epilogue cleanup operations complete.
type BeadCleanupEvent struct {
	BeadID string
	Action string // e.g., "sync", "merge", "worktree_cleanup"
	Time   time.Time
}

func (e *BeadCleanupEvent) EventType() string {
	return "bead_cleanup"
}

func (e *BeadCleanupEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}
