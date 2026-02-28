package stream_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/eventtest"
	"github.com/danabrams/gromit/internal/events/stream"
)

func TestStreamSubscriberSerializesEvents(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()
	buf := &bytes.Buffer{}
	writer := &testWriteCloser{Writer: buf}
	subscriber := stream.NewWriterSubscriber(writer, emitter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = subscriber.Start(ctx)
	}()

	// Wait for subscriber to start using polling
	startCtx, startCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer startCancel()
	if err := eventtest.WaitForSubscriberReady(startCtx, emitter); err != nil {
		t.Fatalf("WaitForSubscriberReady failed: %v", err)
	}

	testEvent := &events.LogEvent{Level: "info", Message: "hello"}
	emitter.Emit(testEvent)

	// Wait for event to be serialized
	processCtx, processCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer processCancel()
	if err := eventtest.WaitForCondition(processCtx, func() bool { return buf.Len() > 0 }); err != nil {
		t.Fatalf("WaitForCondition failed: %v", err)
	}

	cancel()
	<-done

	line := strings.TrimSpace(buf.String())
	var captured struct {
		Type    string          `json:"type"`
		Time    string          `json:"time"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(line), &captured); err != nil {
		t.Fatalf("unmarshal structured line: %v", err)
	}

	if captured.Type != testEvent.EventType() {
		t.Fatalf("type = %q, want %q", captured.Type, testEvent.EventType())
	}

	var payload map[string]any
	if err := json.Unmarshal(captured.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["Level"] != testEvent.Level {
		t.Fatalf("payload Level = %v, want %v", payload["Level"], testEvent.Level)
	}
	if payload["Message"] != testEvent.Message {
		t.Fatalf("payload Message = %v, want %v", payload["Message"], testEvent.Message)
	}
}

type testWriteCloser struct {
	io.Writer
}

func (testWriteCloser) Close() error { return nil }
