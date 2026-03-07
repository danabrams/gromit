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
	defer fs.Close()

	evt := &StageStartedEvent{Event: Event{Type: EventTypeStageStarted}, StageName: "build"}
	fs.Handle(evt)

	// Close to flush before reading
	if err := fs.Close(); err != nil {
		t.Fatalf("closing file subscriber: %v", err)
	}

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
	defer fs.Close()
	emitter := NewEmitter()

	fs.SubscribeTo(emitter)

	emitter.Emit(&StageStartedEvent{Event: Event{Type: EventTypeStageStarted}, StageName: "test"})
	emitter.Close()

	if err := fs.Close(); err != nil {
		t.Fatalf("closing file subscriber: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if !strings.Contains(string(data), EventTypeStageStarted) {
		t.Fatalf("expected event in file after SubscribeTo, got %q", string(data))
	}
}

func TestHandleDoesNotPanicOnInvalidPath(t *testing.T) {
	// Use a path that cannot be written to (null byte in path is invalid on all OSes).
	fs := NewFileSubscriber("/dev/null/\x00/impossible/events.jsonl")

	// Must not panic — errors should be logged, not swallowed silently or cause crashes.
	evt := &StageStartedEvent{Event: Event{Type: EventTypeStageStarted}, StageName: "build"}
	fs.Handle(evt)
}

func TestHandleDoesNotPanicOnReadOnlyPath(t *testing.T) {
	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o555); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(readOnlyDir, "subdir", "events.jsonl")
	fs := NewFileSubscriber(path)

	// Must not panic when the parent directory cannot be created.
	evt := &StageStartedEvent{Event: Event{Type: EventTypeStageStarted}, StageName: "build"}
	fs.Handle(evt)
}

func TestHandleConcurrentWritesProduceValidJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.jsonl")
	fs := NewFileSubscriber(path)
	defer fs.Close()

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

	if err := fs.Close(); err != nil {
		t.Fatalf("closing file subscriber: %v", err)
	}

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

func TestCloseIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	fs := NewFileSubscriber(path)

	// Handle one event to open the file
	fs.Handle(&StageStartedEvent{Event: Event{Type: EventTypeStageStarted}, StageName: "x"})

	// First close should succeed
	if err := fs.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}
	// Second close should also succeed (no-op)
	if err := fs.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

func TestCloseWithoutHandleIsNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	fs := NewFileSubscriber(path)

	// Close without any Handle calls should succeed (file never opened)
	if err := fs.Close(); err != nil {
		t.Fatalf("Close without Handle returned error: %v", err)
	}
}

func TestHandleAfterCloseIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	fs := NewFileSubscriber(path)

	// Write one event to open the file, then close.
	fs.Handle(&StageStartedEvent{Event: Event{Type: EventTypeStageStarted}, StageName: "before"})
	if err := fs.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	// Handle after Close must not reopen the file or panic.
	fs.Handle(&StageStartedEvent{Event: Event{Type: EventTypeStageStarted}, StageName: "after"})

	// Verify the file handle was NOT reopened (still nil).
	fs.mu.Lock()
	reopened := fs.file != nil
	fs.mu.Unlock()
	if reopened {
		t.Fatal("expected file to remain nil after Handle called post-Close, but it was reopened")
	}

	// Verify the file still contains only the original event (no second line).
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

func TestHandleHoldsFileOpenAcrossCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	fs := NewFileSubscriber(path)
	defer fs.Close()

	// Write multiple events
	for i := 0; i < 5; i++ {
		fs.Handle(&StageStartedEvent{Event: Event{Type: EventTypeStageStarted}, StageName: "multi"})
	}

	// The file field should be non-nil (handle held open)
	fs.mu.Lock()
	fileOpen := fs.file != nil
	fs.mu.Unlock()
	if !fileOpen {
		t.Fatal("expected file handle to be held open after Handle calls")
	}

	// Close and verify all lines were written
	if err := fs.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
}
