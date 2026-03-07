package event

import (
	"os"
	"path/filepath"
	"strings"
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
