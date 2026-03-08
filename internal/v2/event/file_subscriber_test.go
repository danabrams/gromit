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

func TestNewWorktreeFileSubscriberUsesDefaultEventsPath(t *testing.T) {
	worktree := t.TempDir()
	fs := NewWorktreeFileSubscriber(worktree)
	want := filepath.Join(worktree, ".gromit", "v2", "events.jsonl")
	if fs.path != want {
		t.Fatalf("expected path %q, got %q", want, fs.path)
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

func TestHandleSkipsNilEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	fs := NewFileSubscriber(path)
	defer fs.Close()

	fs.Handle(&StageStartedEvent{
		Event: Event{SchemaVersion: SchemaVersion, Type: EventTypeStageStarted},
	})

	fs.Handle(nil)

	var typedNil *StageStartedEvent
	fs.Handle(typedNil)

	if err := fs.Close(); err != nil {
		t.Fatalf("closing file subscriber: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected only 1 JSON object line, got %d: %q", len(lines), string(data))
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &payload); err != nil {
		t.Fatalf("line is not valid JSON object: %v", err)
	}
	if payload["type"] != EventTypeStageStarted {
		t.Fatalf("type = %v, want %q", payload["type"], EventTypeStageStarted)
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

func TestHandleAddsCorrelationFieldsFromPriorStageContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	fs := NewFileSubscriber(path)
	defer fs.Close()

	fs.Handle(&StageStartedEvent{
		Event: Event{
			SchemaVersion: SchemaVersion,
			Type:          EventTypeStageStarted,
		},
		StageName: "build",
		BeadID:    "gromit-123",
		Iteration: 2,
	})
	fs.Handle(&BuildInvocationStartEvent{
		Event: Event{
			SchemaVersion: SchemaVersion,
			Type:          EventTypeBuildInvocationStart,
		},
		BeadID: "gromit-123",
		Model:  "claude-haiku-4",
	})

	if err := fs.Close(); err != nil {
		t.Fatalf("closing file subscriber: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), string(data))
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &entry); err != nil {
		t.Fatalf("unmarshal second event: %v", err)
	}
	rawCorrelation, ok := entry["correlation"]
	if !ok {
		t.Fatalf("expected correlation field on second event, got: %v", entry)
	}
	correlation, ok := rawCorrelation.(map[string]interface{})
	if !ok {
		t.Fatalf("correlation type = %T, want object", rawCorrelation)
	}
	if got := correlation["bead_id"]; got != "gromit-123" {
		t.Fatalf("correlation bead_id = %v, want gromit-123", got)
	}
	if got := correlation["stage_name"]; got != "build" {
		t.Fatalf("correlation stage_name = %v, want build", got)
	}
	if got, ok := correlation["iteration"].(float64); !ok || int(got) != 2 {
		t.Fatalf("correlation iteration = %v, want 2", correlation["iteration"])
	}
}
