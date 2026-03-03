package events

import "time"

// TimeMixin provides a shared EventTime implementation for timestamped events.
type TimeMixin struct {
	Time time.Time
}

// EventTime returns the stored time or the current time when zero.
func (m *TimeMixin) EventTime() time.Time {
	if m.Time.IsZero() {
		return time.Now()
	}
	return m.Time
}
