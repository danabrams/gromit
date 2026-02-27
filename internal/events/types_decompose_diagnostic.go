package events

import "time"

// DecomposeStartEvent is emitted when bead decomposition begins.
type DecomposeStartEvent struct {
	BeadID    string
	BeadTitle string
	Time      time.Time
}

func (e *DecomposeStartEvent) EventType() string {
	return "decompose_start"
}

func (e *DecomposeStartEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// SubBeadCreatedEvent is emitted for each sub-bead created.
type SubBeadCreatedEvent struct {
	ParentBeadID  string
	SubBeadID     string
	SubBeadTitle  string
	Index         int
	Total         int
	Time          time.Time
}

func (e *SubBeadCreatedEvent) EventType() string {
	return "subbead_created"
}

func (e *SubBeadCreatedEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// DecomposeCompleteEvent is emitted when decomposition finishes.
type DecomposeCompleteEvent struct {
	BeadID           string
	SubBeadsCreated  int
	Time             time.Time
}

func (e *DecomposeCompleteEvent) EventType() string {
	return "decompose_complete"
}

func (e *DecomposeCompleteEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}
