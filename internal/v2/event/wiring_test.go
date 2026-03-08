package event

import (
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
        Event: Event{SchemaVersion: SchemaVersion, Timestamp: time.Now(), Type: EventTypeSpecStarted},
        SpecID: "wire-test",
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
