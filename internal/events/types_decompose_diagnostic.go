package events

// DecomposeStartEvent is emitted when bead decomposition begins.
type DecomposeStartEvent struct {
	BeadID    string
	BeadTitle string
	TimeMixin
}

func (e *DecomposeStartEvent) EventType() string {
	return "decompose_start"
}

// SubBeadCreatedEvent is emitted for each sub-bead created.
type SubBeadCreatedEvent struct {
	ParentBeadID string
	SubBeadID    string
	SubBeadTitle string
	Index        int
	Total        int
	TimeMixin
}

func (e *SubBeadCreatedEvent) EventType() string {
	return "subbead_created"
}

// DecomposeCompleteEvent is emitted when decomposition finishes.
type DecomposeCompleteEvent struct {
	BeadID          string
	SubBeadsCreated int
	TimeMixin
}

func (e *DecomposeCompleteEvent) EventType() string {
	return "decompose_complete"
}
