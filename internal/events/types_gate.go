package events

import "time"

// GateScopeEvent is emitted when the gate detects a scope issue.
type GateScopeEvent struct {
	BeadID    string
	FileCount int
	MaxFiles  int
	Action    string // "block" or "decompose"
	Time      time.Time
}

func (e *GateScopeEvent) EventType() string {
	return "gate_scope"
}

func (e *GateScopeEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// GateStuckEvent is emitted when the gate detects a stuck bead.
type GateStuckEvent struct {
	BeadID string
	Reason string
	Time   time.Time
}

func (e *GateStuckEvent) EventType() string {
	return "gate_stuck"
}

func (e *GateStuckEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// GateSkipEvent is emitted when the gate decides to skip a bead.
type GateSkipEvent struct {
	BeadID string
	Reason string // "precheck_passed", "scope_decomposed", etc.
	Time   time.Time
}

func (e *GateSkipEvent) EventType() string {
	return "gate_skip"
}

func (e *GateSkipEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// GateBlockEvent is emitted when the gate decides to block a bead.
type GateBlockEvent struct {
	BeadID string
	Reason string // "stuck", "scope", "data_quality", etc.
	Time   time.Time
}

func (e *GateBlockEvent) EventType() string {
	return "gate_block"
}

func (e *GateBlockEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// GateReadinessBlockEvent is emitted when the gate blocks a bead for readiness reasons.
type GateReadinessBlockEvent struct {
	BeadID string
	Reason string
	Time   time.Time
}

func (e *GateReadinessBlockEvent) EventType() string {
	return "gate_readiness_block"
}

func (e *GateReadinessBlockEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}
