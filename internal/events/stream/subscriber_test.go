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

	time.Sleep(25 * time.Millisecond)

	testEvent := &events.LogEvent{Level: "info", Message: "hello"}
	emitter.Emit(testEvent)

	waitForCondition(t, func() bool { return buf.Len() > 0 }, time.Second)

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

func waitForCondition(t *testing.T, cond func() bool, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met before timeout")
}

type testWriteCloser struct {
	io.Writer
}

func (testWriteCloser) Close() error { return nil }
