package integrationqueue

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadQueueReturnsDefaultWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, queueFileName)
	q, err := LoadQueue(path)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}
	if q == nil {
		t.Fatal("expected queue, got nil")
	}
	if q.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", q.SchemaVersion, SchemaVersion)
	}
	if len(q.Entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(q.Entries))
	}
}

func TestSaveQueueUpdatesTimestampAndPersists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, queueFileName)

	queue := &Queue{
		Entries: []Entry{
			{
				Branch:        "gromit/ready",
				SessionID:     "session",
				OriginCommand: "refine",
				State:         StateReady,
				Lane:          string(CodeLane),
				BaseRef:       "main",
				HeadSHA:       "deadbeef",
			},
		},
	}
	if err := SaveQueue(path, queue); err != nil {
		t.Fatalf("SaveQueue() error = %v", err)
	}
	if queue.UpdatedAt.IsZero() {
		t.Fatalf("queue updated_at not set")
	}

	saved, err := LoadQueue(path)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}
	if !saved.UpdatedAt.Equal(queue.UpdatedAt) {
		t.Fatalf("saved updated_at = %v, want %v", saved.UpdatedAt, queue.UpdatedAt)
	}
	if len(saved.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(saved.Entries))
	}
}

func TestLoadQueueValidatesEntries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, queueFileName)

	payload := `{
  "schema_version": 1,
  "entries": [
    {
      "branch": "",
      "session_id": "session",
      "origin_command": "refine",
      "state": "ready",
      "lane": "code_lane",
      "base_ref": "main",
      "head_sha": "deadbeef"
    }
  ]
}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write invalid queue: %v", err)
	}
	if _, err := LoadQueue(path); err == nil {
		t.Fatal("LoadQueue() succeeded for invalid queue")
	}
}

func TestSaveQueueValidatesEntries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, queueFileName)

	queue := &Queue{
		Entries: []Entry{
			{
				Branch:        "",
				SessionID:     "session",
				OriginCommand: "refine",
				State:         StateReady,
				Lane:          string(CodeLane),
				BaseRef:       "main",
				HeadSHA:       "deadbeef",
			},
		},
	}
	if err := SaveQueue(path, queue); err == nil {
		t.Fatal("SaveQueue() succeeded for invalid queue")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("queue file exists after failed save: %v", err)
	}
}

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

	first := readQueueSnapshot(t, tmpDir)
	if first.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", first.SchemaVersion, SchemaVersion)
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

	updated := readQueueSnapshot(t, tmpDir)
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

func readQueueSnapshot(t *testing.T, gromitDir string) Snapshot {
	t.Helper()
	path := filepath.Join(gromitDir, queueFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read queue file: %v", err)
	}
	var payload Snapshot
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal queue file: %v", err)
	}
	return payload
}

func TestStoreSaveSortsChangedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	entry := Entry{
		Branch:        "gromit/changed-files",
		SessionID:     "session",
		OriginCommand: "ready",
		State:         "ready",
		Lane:          "code_lane",
		BaseRef:       "main",
		HeadSHA:       "abc123",
		ChangedFiles:  []string{"z.txt", "a.txt"},
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	payload := readQueueSnapshot(t, tmpDir)
	if len(payload.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(payload.Entries))
	}

	got := payload.Entries[0].ChangedFiles
	want := []string{"a.txt", "z.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changed files = %v, want %v", got, want)
	}
}

func TestTrimJSONPrefix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "clean JSON unchanged",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "strips text before opening brace",
			input: `Removed: [branch1, branch2]` + "\n" + `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "strips whitespace-only prefix",
			input: "  \n\t" + `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "no brace returns original",
			input: "not json at all",
			want:  "not json at all",
		},
		{
			name:  "empty input returns empty",
			input: "",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(trimJSONPrefix([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("trimJSONPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadQueueToleratesPrefixGarbage(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, queueFileName)

	validJSON := fmt.Sprintf(`{"schema_version": %d, "entries": []}`, SchemaVersion)
	garbage := "Removed: [branch1, branch2]\n" + validJSON

	if err := os.WriteFile(path, []byte(garbage), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	q, err := LoadQueue(path)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v, want nil", err)
	}
	if q.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", q.SchemaVersion, SchemaVersion)
	}
}

func TestStoreLoadToleratesPrefixGarbage(t *testing.T) {
	tmpDir := t.TempDir()

	// First save a valid entry so the file exists
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	entry := Entry{
		Branch:        "gromit/test",
		SessionID:     "session",
		OriginCommand: "refine",
		State:         StateReady,
		Lane:          string(CodeLane),
		BaseRef:       "main",
		HeadSHA:       "abc123",
	}
	if err := store.Save(entry); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Now prepend garbage to the file
	path := filepath.Join(tmpDir, queueFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	corrupted := append([]byte("Removed: [stale-branch]\n"), data...)
	if err := os.WriteFile(path, corrupted, 0o644); err != nil {
		t.Fatalf("write corrupted: %v", err)
	}

	// Snapshot (via load()) should still work
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v, want nil", err)
	}
	if len(snap.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(snap.Entries))
	}
	if snap.Entries[0].Branch != "gromit/test" {
		t.Fatalf("branch = %q, want %q", snap.Entries[0].Branch, "gromit/test")
	}
}

func TestStoreSaveRunsValidationHooks(t *testing.T) {
	tmpDir := t.TempDir()
	customErr := fmt.Errorf("validation failure")
	store, err := NewStore(tmpDir, WithValidationHook(func(entry Entry) error {
		if entry.Branch == "reject" {
			return customErr
		}
		return nil
	}))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	entry := Entry{
		Branch:        "reject",
		SessionID:     "session",
		OriginCommand: "ready",
		State:         "ready",
		Lane:          "code_lane",
		BaseRef:       "main",
		HeadSHA:       "abc123",
	}
	if err := store.Save(entry); !errors.Is(err, customErr) {
		t.Fatalf("Save() error = %v, want %v", err, customErr)
	}
}
