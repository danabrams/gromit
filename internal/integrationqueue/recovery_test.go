package integrationqueue

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestStore_HandlesMalformedQueueFile verifies that Store.Snapshot() handles
// malformed queue files by returning ErrSchemaInvalid without crashing.
// This test ensures recovery handling works when a queue file is corrupt.
func TestStore_HandlesMalformedQueueFile(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a malformed queue file
	queuePath := filepath.Join(gromitDir, queueFileName)
	malformedData := []byte(`{ invalid json }`)
	if err := os.WriteFile(queuePath, malformedData, 0o644); err != nil {
		t.Fatalf("write malformed queue: %v", err)
	}

	store, err := NewStore(gromitDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Attempting to load a malformed queue should return ErrSchemaInvalid
	snapshot, err := store.Snapshot()
	if err == nil {
		t.Fatal("expected ErrSchemaInvalid, got nil")
	}
	if !errors.Is(err, ErrSchemaInvalid) {
		t.Fatalf("expected ErrSchemaInvalid, got %v", err)
	}
	if snapshot != nil {
		t.Fatalf("expected nil snapshot with error, got %v", snapshot)
	}
}

// TestRecoverFromMalformedQueue verifies that RecoverFromMalformedQueue()
// transitions integrating entries back to ready and persists the recovered queue.
func TestRecoverFromMalformedQueue(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, queueFileName)

	// Create a queue with entries in various states, including integrating
	queue := &Queue{
		SchemaVersion: SchemaVersion,
		Entries: []Entry{
			{
				Branch:        "feature/one",
				SessionID:     "session1",
				OriginCommand: "refine",
				State:         StateReady,
				Lane:          string(CodeLane),
				BaseRef:       "main",
				HeadSHA:       "deadbeef",
				FifoSeq:       1,
			},
			{
				Branch:        "feature/two",
				SessionID:     "session2",
				OriginCommand: "refine",
				State:         StateIntegrating,
				Lane:          string(CodeLane),
				BaseRef:       "main",
				HeadSHA:       "cafebabe",
				FifoSeq:       2,
			},
			{
				Branch:        "feature/three",
				SessionID:     "session3",
				OriginCommand: "refine",
				State:         StateIntegrating,
				Lane:          string(CodeLane),
				BaseRef:       "main",
				HeadSHA:       "beefdead",
				FifoSeq:       3,
			},
		},
	}
	if err := SaveQueue(queuePath, queue); err != nil {
		t.Fatalf("SaveQueue: %v", err)
	}

	// Recover from malformed queue
	recovered, err := RecoverFromMalformedQueue(context.Background(), queuePath)
	if err != nil {
		t.Fatalf("RecoverFromMalformedQueue: %v", err)
	}
	if recovered == nil {
		t.Fatal("expected recovered queue, got nil")
	}

	// Verify that integrating entries were transitioned back to ready
	if len(recovered.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(recovered.Entries))
	}

	// Entry 0 (ready) should remain ready
	if recovered.Entries[0].State != StateReady {
		t.Fatalf("entry 0: expected state %s, got %s", StateReady, recovered.Entries[0].State)
	}

	// Entry 1 (integrating) should be reset to ready
	if recovered.Entries[1].State != StateReady {
		t.Fatalf("entry 1: expected state %s, got %s", StateReady, recovered.Entries[1].State)
	}
	if recovered.Entries[1].LastErrorCode != "queue_schema_invalid" {
		t.Fatalf("entry 1: expected error code queue_schema_invalid, got %s", recovered.Entries[1].LastErrorCode)
	}

	// Entry 2 (integrating) should be reset to ready
	if recovered.Entries[2].State != StateReady {
		t.Fatalf("entry 2: expected state %s, got %s", StateReady, recovered.Entries[2].State)
	}
	if recovered.Entries[2].LastErrorCode != "queue_schema_invalid" {
		t.Fatalf("entry 2: expected error code queue_schema_invalid, got %s", recovered.Entries[2].LastErrorCode)
	}

	// Verify that recovery was persisted to disk.
	persisted, err := LoadQueue(queuePath)
	if err != nil {
		t.Fatalf("LoadQueue(persisted): %v", err)
	}
	if persisted.Entries[1].State != StateReady {
		t.Fatalf("persisted entry 1: expected state %s, got %s", StateReady, persisted.Entries[1].State)
	}
	if persisted.Entries[2].State != StateReady {
		t.Fatalf("persisted entry 2: expected state %s, got %s", StateReady, persisted.Entries[2].State)
	}
}
