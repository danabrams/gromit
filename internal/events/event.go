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
	TimeMixin
}

// EventType returns the event type identifier.
func (e *LogEvent) EventType() string {
	return "log"
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

// EmitterMixin is an embeddable struct that provides an Emitter field and SetEmitter method
// for stages that need to accept an Emitter for event emission.
// Parent types should implement their own WithEmitter method that returns the parent type
// for proper method chaining.
type EmitterMixin struct {
	Emitter *Emitter
}

// SetEmitter is used by orchestrator wiring to attach an emitter without chaining support.
func (em *EmitterMixin) SetEmitter(emitter *Emitter) {
	em.Emitter = emitter
}

// Log emits a LogEvent with the given level and formatted message via the embedded Emitter.
// It is safe to call when Emitter is nil (no-op).
func (em *EmitterMixin) Log(level, format string, args ...interface{}) {
	logger := &EmitterLogger{Emitter: em.Emitter}
	logger.Log(level, format, args...)
}
