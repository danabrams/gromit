package events

// GenerationCapReachedEvent is emitted when the run loop stops because the generation cap is exceeded.
type GenerationCapReachedEvent struct {
	SpecID        string
	GenerationCap int
	TimeMixin
}

func (e *GenerationCapReachedEvent) EventType() string {
	return "generation_cap_reached"
}

// AndonTriggeredEvent signals that Andon escalation is required after remediation failed.
type AndonTriggeredEvent struct {
	SpecID       string
	Reason       string
	FindingCount int
	TimeMixin
}

func (e *AndonTriggeredEvent) EventType() string {
	return "andon_triggered"
}
