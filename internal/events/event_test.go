//go:build !wasm

package events

import (
	"testing"
	"time"
)

// TestLogEvent_Implements_Event tests that LogEvent implements the Event interface.
func TestLogEvent_Implements_Event(t *testing.T) {
	t.Parallel()
	event := &LogEvent{
		Level:   "info",
		Message: "test message",
		Time:    time.Now(),
	}

	// Compile-time interface check is implicit; runtime checks verify methods work.
	if event.EventType() != "log" {
		t.Errorf("EventType() = %s, want %s", event.EventType(), "log")
	}

	if !event.EventTime().Before(time.Now().Add(time.Second)) {
		t.Error("EventTime() returned invalid time")
	}
}

// TestLogEvent_EventTime_UsesProvidedTime tests that EventTime returns the Time field.
func TestLogEvent_EventTime_UsesProvidedTime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 2, 27, 10, 30, 0, 0, time.UTC)
	event := &LogEvent{
		Level:   "info",
		Message: "test",
		Time:    now,
	}

	if event.EventTime() != now {
		t.Errorf("EventTime() = %v, want %v", event.EventTime(), now)
	}
}

// TestLogEvent_EventType_AlwaysReturnsLog tests that EventType always returns "log".
func TestLogEvent_EventType_AlwaysReturnsLog(t *testing.T) {
	t.Parallel()
	event := &LogEvent{Level: "error", Message: "error message"}
	if event.EventType() != "log" {
		t.Errorf("EventType() = %s, want log", event.EventType())
	}
}
