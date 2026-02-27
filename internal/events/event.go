package events

import (
	"fmt"
	"time"
)

// Event is the interface that all events must implement.
type Event interface {
	EventType() string
	EventTime() time.Time
}

// LogEvent is a transitional event type for generic log messages.
type LogEvent struct {
	Level   string
	Message string
	Time    time.Time
}

// EventType returns the event type identifier.
func (e *LogEvent) EventType() string {
	return "log"
}

// EventTime returns the event timestamp.
func (e *LogEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// EmitterLogger is a helper that logs formatted messages by emitting LogEvent to an Emitter.
type EmitterLogger struct {
	Emitter *Emitter
}

// Log emits a LogEvent with the given level and formatted message.
// It is safe to call when Emitter is nil (no-op).
func (el *EmitterLogger) Log(level, format string, args ...interface{}) {
	if el.Emitter == nil {
		return
	}
	el.Emitter.Emit(&LogEvent{
		Level:   level,
		Message: fmt.Sprintf(format, args...),
	})
}
