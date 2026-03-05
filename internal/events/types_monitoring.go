package events

// DroppedEventsEvent is emitted when one or more events are dropped because a subscriber channel was full.
type DroppedEventsEvent struct {
	// Count reports how many events were dropped by the most recent Emit call.
	Count int64
	// Total reports the cumulative number of dropped events since the emitter was created.
	Total int64
	TimeMixin
}

// EventType returns the dropped events event identifier.
func (e *DroppedEventsEvent) EventType() string {
	return "dropped_events"
}
