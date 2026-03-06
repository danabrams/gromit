package main

import (
    "testing"
    "time"

    "github.com/danabrams/gromit/internal/events"
)

func TestSpecLoopStageRecorderEmitsLogEvent(t *testing.T) {
    t.Parallel()

    emitter := events.NewEmitter()
    defer emitter.Close()

    ch := emitter.Subscribe()
    defer emitter.Unsubscribe(ch)

    recorder := newSpecLoopStageRecorder(emitter, "example-spec")
    recorder.RecordStage("plan")

    select {
    case evt := <-ch:
        logEvent, ok := evt.(*events.LogEvent)
        if !ok {
            t.Fatalf("expected LogEvent, got %T", evt)
        }
        expected := "spec example-spec: stage plan"
        if logEvent.Message != expected {
            t.Fatalf("message = %q, want %q", logEvent.Message, expected)
        }
        if logEvent.Level != "info" {
            t.Fatalf("level = %q, want %q", logEvent.Level, "info")
        }
    case <-time.After(100 * time.Millisecond):
        t.Fatal("timeout waiting for log event")
    }
}
