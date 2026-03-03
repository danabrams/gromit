package events

// GateScopeEvent is emitted when the gate detects a scope issue.
type GateScopeEvent struct {
	BeadID    string
	FileCount int
	MaxFiles  int
	Action    string // "block" or "decompose"
	TimeMixin
}

func (e *GateScopeEvent) EventType() string {
	return "gate_scope"
}

// GateStuckEvent is emitted when the gate detects a stuck bead.
type GateStuckEvent struct {
	BeadID string
	Reason string
	TimeMixin
}

func (e *GateStuckEvent) EventType() string {
	return "gate_stuck"
}

// GateSkipEvent is emitted when the gate decides to skip a bead.
type GateSkipEvent struct {
	BeadID string
	Reason string // "precheck_passed", "scope_decomposed", etc.
	TimeMixin
}

func (e *GateSkipEvent) EventType() string {
	return "gate_skip"
}

// GateBlockEvent is emitted when the gate decides to block a bead.
type GateBlockEvent struct {
	BeadID string
	Reason string // "stuck", "scope", "data_quality", etc.
	TimeMixin
}

func (e *GateBlockEvent) EventType() string {
	return "gate_block"
}

// GateReadinessBlockEvent is emitted when the gate blocks a bead for readiness reasons.
type GateReadinessBlockEvent struct {
	BeadID string
	Reason string
	TimeMixin
}

func (e *GateReadinessBlockEvent) EventType() string {
	return "gate_readiness_block"
}
