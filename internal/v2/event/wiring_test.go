package event

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWireWorktreeFileSubscriberWritesEvents(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	worktree := filepath.Join(tempDir, "worktree")

	emitter := NewEmitter()
	cleanup := WireWorktreeFileSubscriber(emitter, worktree)
	if cleanup == nil {
		t.Fatal("expected cleanup function")
	}

	emitter.Emit(&SpecStartedEvent{
		Event:    Event{SchemaVersion: SchemaVersion, Timestamp: time.Now(), Type: EventTypeSpecStarted},
		SpecID:   "wire-test",
		Worktree: worktree,
	})

	emitter.Close()
	cleanup()

	eventsPath := filepath.Join(worktree, ".gromit", "v2", "events.jsonl")
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events file: %v", err)
	}

	if !strings.Contains(string(data), EventTypeSpecStarted) {
		t.Fatalf("events log missing type: %s", string(data))
	}
}

func TestStartWorktreeEventLogSubscriberPreservesExistingEvents(t *testing.T) {
	t.Parallel()

	worktree := filepath.Join(t.TempDir(), "worktree")
	existingDir := filepath.Join(worktree, ".gromit", "v2")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatalf("mkdir events dir: %v", err)
	}
	eventsPath := filepath.Join(existingDir, "events.jsonl")
	initial := `{"type":"existing"}`
	if err := os.WriteFile(eventsPath, []byte(initial+"\n"), 0o644); err != nil {
		t.Fatalf("write initial events log: %v", err)
	}

	emitter := NewEmitter()
	cleanup := StartWorktreeEventLogSubscriber(emitter, worktree)
	if cleanup == nil {
		t.Fatal("expected cleanup function")
	}

	emitter.Emit(&StageStartedEvent{
		Event: Event{
			SchemaVersion: SchemaVersion,
			Type:          EventTypeStageStarted,
		},
		StageName: "preserve",
	})
	emitter.Close()
	cleanup()

	data, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events log: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines after append, got %d: %q", len(lines), string(data))
	}
	if strings.TrimSpace(lines[0]) != initial {
		t.Fatalf("initial line changed, got %q", lines[0])
	}
	if !strings.Contains(lines[1], EventTypeStageStarted) {
		t.Fatalf("appended line missing stage event: %q", lines[1])
	}
}

func TestStartLegacyEventSubscribersRequiresEmitter(t *testing.T) {
	t.Parallel()

	if _, err := StartLegacyEventSubscribers(context.Background(), nil, io.Discard, ""); err == nil {
		t.Fatal("expected error when emitter is nil")
	}
}
