package event

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewFileSubscriberStoresPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	fs := NewFileSubscriber(path)
	if fs.path != path {
		t.Fatalf("expected path %q, got %q", path, fs.path)
	}
}

func TestHandleAppendsJSONLineToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "events.jsonl")
	fs := NewFileSubscriber(path)

	evt := &StageStartedEvent{Event: Event{Type: EventTypeStageStarted}, StageName: "build"}
	fs.Handle(evt)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	line := string(data)
	if line == "" {
		t.Fatal("expected non-empty file after Handle")
	}
	// must end with newline (JSONL convention)
	if line[len(line)-1] != '\n' {
		t.Fatalf("expected line ending with newline, got %q", line)
	}
	// must contain event type
	if !strings.Contains(line, EventTypeStageStarted) {
		t.Fatalf("expected event type in output, got %q", line)
	}
}

func TestSubscribeToRegistersHandlerWithEmitter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	fs := NewFileSubscriber(path)
	emitter := NewEmitter()

	fs.SubscribeTo(emitter)

	emitter.Emit(&StageStartedEvent{Event: Event{Type: EventTypeStageStarted}, StageName: "test"})
	emitter.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if !strings.Contains(string(data), EventTypeStageStarted) {
		t.Fatalf("expected event in file after SubscribeTo, got %q", string(data))
	}
}

func TestHandleConcurrentWritesProduceValidJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.jsonl")
	fs := NewFileSubscriber(path)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			fs.Handle(&StageStartedEvent{Event: Event{Type: EventTypeStageStarted}, StageName: "concurrent"})
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != n {
		t.Fatalf("expected %d lines, got %d", n, len(lines))
	}
	for i, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("line %d is not valid JSON: %q", i, line)
		}
	}
}
