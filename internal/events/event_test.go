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

// TestTimeMixin_EventTime ensures the embeddable TimeMixin exposes the usual EventTime behavior.
func TestTimeMixin_EventTime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 2, 27, 10, 30, 0, 0, time.UTC)
	mixin := &TimeMixin{Time: now}

	if got := mixin.EventTime(); got != now {
		t.Fatalf("EventTime() = %v, want %v", got, now)
	}

	if zero := (&TimeMixin{}).EventTime(); zero.IsZero() {
		t.Errorf("EventTime() should not return zero for zero-value mixin")
	}
}

// TestEmitterLogger_Log_EmitsLogEvent tests that EmitterLogger.Log emits a LogEvent with correct level and message.
func TestEmitterLogger_Log_EmitsLogEvent(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	defer emitter.Close()

	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	logger := &EmitterLogger{Emitter: emitter}
	logger.Log("warning", "test %s", "message")

	select {
	case evt := <-ch:
		logEvent, ok := evt.(*LogEvent)
		if !ok {
			t.Fatalf("expected LogEvent, got %T", evt)
		}
		if logEvent.Level != "warning" {
			t.Errorf("Level = %q, want %q", logEvent.Level, "warning")
		}
		if logEvent.Message != "test message" {
			t.Errorf("Message = %q, want %q", logEvent.Message, "test message")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for LogEvent")
	}
}

// TestEmitterLogger_Log_DoesNothingWhenEmitterNil tests that Log is safe when emitter is nil.
func TestEmitterLogger_Log_DoesNothingWhenEmitterNil(t *testing.T) {
	t.Parallel()
	logger := &EmitterLogger{Emitter: nil}
	// Should not panic
	logger.Log("info", "test %s", "message")
}

// mockStageWithEmitterMixin is a test fixture that embeds EmitterMixin.
type mockStageWithEmitterMixin struct {
	EmitterMixin
	name string
}

// WithEmitter returns this stage for method chaining (required for embedded EmitterMixin).
func (m *mockStageWithEmitterMixin) WithEmitter(emitter *Emitter) *mockStageWithEmitterMixin {
	m.EmitterMixin.SetEmitter(emitter)
	return m
}

// TestEmitterMixin_SetEmitter_SetsEmitter tests that SetEmitter sets the emitter field.
func TestEmitterMixin_SetEmitter_SetsEmitter(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	defer emitter.Close()

	mixin := &EmitterMixin{}
	mixin.SetEmitter(emitter)

	if mixin.Emitter != emitter {
		t.Errorf("Emitter field not set correctly by SetEmitter")
	}
}

// TestEmitterMixin_Log_EmitsLogEvent tests that EmitterMixin.Log emits a LogEvent via the embedded Emitter.
func TestEmitterMixin_Log_EmitsLogEvent(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	defer emitter.Close()

	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	mixin := &EmitterMixin{}
	mixin.SetEmitter(emitter)
	mixin.Log("info", "hello %s", "world")

	select {
	case evt := <-ch:
		logEvent, ok := evt.(*LogEvent)
		if !ok {
			t.Fatalf("expected LogEvent, got %T", evt)
		}
		if logEvent.Level != "info" {
			t.Errorf("Level = %q, want %q", logEvent.Level, "info")
		}
		if logEvent.Message != "hello world" {
			t.Errorf("Message = %q, want %q", logEvent.Message, "hello world")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for LogEvent")
	}
}

// TestEmitterMixin_Log_DoesNothingWhenEmitterNil tests that Log is safe when embedded Emitter is nil.
func TestEmitterMixin_Log_DoesNothingWhenEmitterNil(t *testing.T) {
	t.Parallel()
	mixin := &EmitterMixin{}
	// Should not panic
	mixin.Log("warning", "test %s", "message")
}

// TestEmitterMixin_EmbeddedWithEmitter_WorksWithParent tests that WithEmitter works on a stage that embeds EmitterMixin.
func TestEmitterMixin_EmbeddedWithEmitter_WorksWithParent(t *testing.T) {
	t.Parallel()
	emitter := NewEmitter()
	defer emitter.Close()

	stage := &mockStageWithEmitterMixin{name: "test"}
	result := stage.WithEmitter(emitter)

	if result != stage {
		t.Errorf("WithEmitter should return the parent stage")
	}
	if stage.Emitter != emitter {
		t.Errorf("Emitter field not set correctly through embedded mixin")
	}
}
