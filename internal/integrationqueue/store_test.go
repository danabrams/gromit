package integrationqueue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreSaveCreatesAndUpdatesEntry(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	entry := Entry{
		Branch:               "gromit/ready",
		SessionID:            "gromit/ready",
		OriginCommand:        "refine",
		State:                "ready",
		Lane:                 "code_lane",
		BaseRef:              "main",
		HeadSHA:              "abc123",
		ChangedFiles:         []string{"cmd/gromit/refine.go"},
		ChangedFilesHash:     "sha256:abc",
		LastTransitionReason: "session_committed",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	first := readQueueFile(t, tmpDir)
	if first.SchemaVersion != schemaVersionV {
		t.Fatalf("schema version = %d, want %d", first.SchemaVersion, schemaVersionV)
	}
	if len(first.Entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(first.Entries))
	}
	if first.Entries[0].FifoSeq != 1 {
		t.Fatalf("fifo_seq = %d, want 1", first.Entries[0].FifoSeq)
	}
	if first.Entries[0].State != "ready" {
		t.Fatalf("state = %s, want ready", first.Entries[0].State)
	}

	entry.State = "conflict"
	entry.LastErrorCode = "session_commit"
	entry.LastErrorMessage = "auto commit blocked"
	entry.LastTransitionReason = "session_commit_failed"
	if err := store.Save(entry); err != nil {
		t.Fatalf("update save: %v", err)
	}

	updated := readQueueFile(t, tmpDir)
	if len(updated.Entries) != 1 {
		t.Fatalf("entry count after update = %d, want 1", len(updated.Entries))
	}
	if updated.Entries[0].FifoSeq != 1 {
		t.Fatalf("fifo_seq after update = %d, want 1", updated.Entries[0].FifoSeq)
	}
	if updated.Entries[0].State != "conflict" {
		t.Fatalf("state after update = %s, want conflict", updated.Entries[0].State)
	}
	if updated.Entries[0].LastErrorMessage != "auto commit blocked" {
		t.Fatalf("last error message = %q, want %q", updated.Entries[0].LastErrorMessage, "auto commit blocked")
	}
}

func readQueueFile(t *testing.T, gromitDir string) queueFile {
	t.Helper()
	path := filepath.Join(gromitDir, queueFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read queue file: %v", err)
	}
	var payload queueFile
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal queue file: %v", err)
	}
	return payload
}
