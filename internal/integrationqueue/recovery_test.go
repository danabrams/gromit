package integrationqueue

import (
	"context"
	"errors"
	"fmt"
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
	if recovered.Entries[1].LastErrorMessage != schemaRecoveryMessage {
		t.Fatalf("entry 1: expected error message %q, got %q", schemaRecoveryMessage, recovered.Entries[1].LastErrorMessage)
	}

	// Entry 2 (integrating) should be reset to ready
	if recovered.Entries[2].State != StateReady {
		t.Fatalf("entry 2: expected state %s, got %s", StateReady, recovered.Entries[2].State)
	}
	if recovered.Entries[2].LastErrorCode != "queue_schema_invalid" {
		t.Fatalf("entry 2: expected error code queue_schema_invalid, got %s", recovered.Entries[2].LastErrorCode)
	}
	if recovered.Entries[2].LastErrorMessage != schemaRecoveryMessage {
		t.Fatalf("entry 2: expected error message %q, got %q", schemaRecoveryMessage, recovered.Entries[2].LastErrorMessage)
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
	if persisted.Entries[1].LastErrorMessage != schemaRecoveryMessage {
		t.Fatalf("persisted entry 1: expected error message %q, got %q", schemaRecoveryMessage, persisted.Entries[1].LastErrorMessage)
	}
	if persisted.Entries[2].LastErrorMessage != schemaRecoveryMessage {
		t.Fatalf("persisted entry 2: expected error message %q, got %q", schemaRecoveryMessage, persisted.Entries[2].LastErrorMessage)
	}
}

func TestRecoverFromMalformedQueue_PreservesLastErrorCode(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, queueFileName)

	const (
		originalCode    = "merge_conflict"
		originalMessage = "merge conflict happened"
	)

	queue := &Queue{
		SchemaVersion: SchemaVersion,
		Entries: []Entry{
			{
				Branch:           "feature/retain-error",
				SessionID:        "session-merge",
				OriginCommand:    "refine",
				State:            StateIntegrating,
				Lane:             string(CodeLane),
				BaseRef:          "main",
				HeadSHA:          "cafebabefaced",
				FifoSeq:          42,
				LastErrorCode:    originalCode,
				LastErrorMessage: originalMessage,
			},
		},
	}
	if err := SaveQueue(queuePath, queue); err != nil {
		t.Fatalf("SaveQueue: %v", err)
	}

	recovered, err := RecoverFromMalformedQueue(context.Background(), queuePath)
	if err != nil {
		t.Fatalf("RecoverFromMalformedQueue: %v", err)
	}
	if len(recovered.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(recovered.Entries))
	}
	recoveredEntry := recovered.Entries[0]
	if recoveredEntry.State != StateReady {
		t.Fatalf("expected recovered state %s, got %s", StateReady, recoveredEntry.State)
	}
	if recoveredEntry.LastErrorCode != originalCode {
		t.Fatalf("expected last error code %s, got %s", originalCode, recoveredEntry.LastErrorCode)
	}
	if recoveredEntry.LastErrorMessage != originalMessage {
		t.Fatalf("expected last error message %q, got %q", originalMessage, recoveredEntry.LastErrorMessage)
	}

	persisted, err := LoadQueue(queuePath)
	if err != nil {
		t.Fatalf("LoadQueue(persisted): %v", err)
	}
	if len(persisted.Entries) != 1 {
		t.Fatalf("persisted: expected 1 entry, got %d", len(persisted.Entries))
	}
	if persisted.Entries[0].LastErrorCode != originalCode {
		t.Fatalf("persisted: expected last error code %s, got %s", originalCode, persisted.Entries[0].LastErrorCode)
	}
}

func TestRecoverFromMalformedQueue_RetainsLegacyMergeConflictCode(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, queueFileName)

	const recoveryJSON = `{
  "schema_version": %d,
  "entries": [
    {
      "branch": "feature/legacy-merge",
      "session_id": "session-legacy",
      "origin_command": "refine",
      "state": "integrating",
      "lane": "code_lane",
      "base_ref": "main",
      "head_sha": "cafebabeface",
      "fifo_seq": 99,
      "last_error_code": "merge_conflict",
      "last_error_message": ""
    }
  ]
}`

	if err := os.WriteFile(queuePath, []byte(fmt.Sprintf(recoveryJSON, SchemaVersion)), 0o644); err != nil {
		t.Fatalf("write legacy queue: %v", err)
	}

	recovered, err := RecoverFromMalformedQueue(context.Background(), queuePath)
	if err != nil {
		t.Fatalf("RecoverFromMalformedQueue: %v", err)
	}
	if len(recovered.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(recovered.Entries))
	}
	entry := recovered.Entries[0]
	if entry.State != StateReady {
		t.Fatalf("expected recovered state %s, got %s", StateReady, entry.State)
	}
	if entry.LastErrorCode != "merge_conflict" {
		t.Fatalf("expected LastErrorCode merge_conflict, got %s", entry.LastErrorCode)
	}
	if entry.LastErrorMessage == "" {
		t.Fatalf("expected last error message to be populated, got empty string")
	}

	persisted, err := LoadQueue(queuePath)
	if err != nil {
		t.Fatalf("LoadQueue(persisted): %v", err)
	}
	if len(persisted.Entries) != 1 {
		t.Fatalf("persisted: expected 1 entry, got %d", len(persisted.Entries))
	}
	if persisted.Entries[0].LastErrorCode != "merge_conflict" {
		t.Fatalf("persisted: expected last error code merge_conflict, got %s", persisted.Entries[0].LastErrorCode)
	}
	if persisted.Entries[0].LastErrorMessage == "" {
		t.Fatal("persisted: expected last error message to be populated")
	}
}

func TestRecoverFromMalformedQueue_UnsupportedSchemaVersion(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, queueFileName)

	legacy := `{
	  "schema_version": 999,
	  "entries": [
	    {
	      "branch": "feature/recover-me",
	      "session_id": "session1",
	      "origin_command": "refine",
	      "state": "integrating",
	      "lane": "code_lane",
	      "base_ref": "main",
	      "head_sha": "deadbeef",
	      "fifo_seq": 1
	    }
	  ]
	}`
	if err := os.WriteFile(queuePath, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy queue: %v", err)
	}

	recovered, err := RecoverFromMalformedQueue(context.Background(), queuePath)
	if err != nil {
		t.Fatalf("RecoverFromMalformedQueue: %v", err)
	}
	if recovered == nil {
		t.Fatal("expected recovered queue, got nil")
	}
	if recovered.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema version %d, got %d", SchemaVersion, recovered.SchemaVersion)
	}
	if len(recovered.Entries) != 1 {
		t.Fatalf("expected 1 recovered entry, got %d", len(recovered.Entries))
	}
	if recovered.Entries[0].State != StateReady {
		t.Fatalf("expected recovered state %s, got %s", StateReady, recovered.Entries[0].State)
	}

	persisted, err := LoadQueue(queuePath)
	if err != nil {
		t.Fatalf("LoadQueue(persisted): %v", err)
	}
	if persisted.SchemaVersion != SchemaVersion {
		t.Fatalf("persisted schema version: expected %d, got %d", SchemaVersion, persisted.SchemaVersion)
	}
	if persisted.Entries[0].State != StateReady {
		t.Fatalf("persisted state: expected %s, got %s", StateReady, persisted.Entries[0].State)
	}
}
