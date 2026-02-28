package integrationqueue

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLaneConstants(t *testing.T) {
	// Verify Lane constants are defined
	if SafeLane == "" {
		t.Fatal("SafeLane constant is not defined")
	}
	if CodeLane == "" {
		t.Fatal("CodeLane constant is not defined")
	}
	if SafeLane != "safe_lane" {
		t.Fatalf("SafeLane = %q, want %q", SafeLane, "safe_lane")
	}
	if CodeLane != "code_lane" {
		t.Fatalf("CodeLane = %q, want %q", CodeLane, "code_lane")
	}
}

func TestLaneEnumValidation(t *testing.T) {
	if !SafeLane.Valid() {
		t.Fatalf("SafeLane.Valid() = false, want true")
	}
	if !CodeLane.Valid() {
		t.Fatalf("CodeLane.Valid() = false, want true")
	}
	if Lane("").Valid() {
		t.Fatalf("empty Lane should not be valid")
	}
	if Lane("unknown_lane").Valid() {
		t.Fatalf("unknown Lane should not be valid")
	}
}

func TestErrorCodeConstants(t *testing.T) {
	// Verify ErrorCode constants are defined
	if ErrorCodeSessionCommitFailed == "" {
		t.Fatal("ErrorCodeSessionCommitFailed constant is not defined")
	}
	if ErrorCodeSessionCommitFailed != "session_commit_failed" {
		t.Fatalf("ErrorCodeSessionCommitFailed = %q, want %q", ErrorCodeSessionCommitFailed, "session_commit_failed")
	}
}

func TestSchemaVersionConstant(t *testing.T) {
	// Verify SchemaVersion constant is defined
	if SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want %d", SchemaVersion, 1)
	}
}

func TestQueueStruct(t *testing.T) {
	// Verify Queue struct exists with correct fields
	created := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	queue := Queue{
		SchemaVersion: 1,
		UpdatedAt:     created,
		Entries:       []Entry{},
	}
	if queue.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want %d", queue.SchemaVersion, 1)
	}
	if !queue.UpdatedAt.Equal(created) {
		t.Fatalf("UpdatedAt = %v, want %v", queue.UpdatedAt, created)
	}
	if queue.Entries == nil {
		t.Fatal("Entries should not be nil")
	}
}

func TestEntryJSONMarshalWithRequiredFields(t *testing.T) {
	created := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	entry := Entry{
		Branch:           "gromit/test",
		SessionID:        "session123",
		OriginCommand:    "review",
		State:            StateReady,
		Lane:             "code_lane",
		CreatedAt:        created,
		UpdatedAt:        updated,
		AttemptCount:     1,
		RetryCount:       0,
		FifoSeq:          5,
		BaseRef:          "main",
		HeadSHA:          "abc123def",
		ChangedFilesHash: "sha256:hash",
		LastErrorCode:    "",
		LastErrorMessage: "",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled Entry
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Branch != entry.Branch {
		t.Errorf("branch mismatch: got %q, want %q", unmarshaled.Branch, entry.Branch)
	}
	if unmarshaled.State != entry.State {
		t.Errorf("state mismatch: got %q, want %q", unmarshaled.State, entry.State)
	}
	if unmarshaled.Lane != entry.Lane {
		t.Errorf("lane mismatch: got %q, want %q", unmarshaled.Lane, entry.Lane)
	}
}

func TestEntryJSONMarshalWithChangedFiles(t *testing.T) {
	created := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	entry := Entry{
		Branch:           "gromit/test",
		SessionID:        "session123",
		OriginCommand:    "review",
		State:            StateReady,
		Lane:             "code_lane",
		CreatedAt:        created,
		UpdatedAt:        updated,
		AttemptCount:     1,
		RetryCount:       0,
		FifoSeq:          5,
		BaseRef:          "main",
		HeadSHA:          "abc123def",
		ChangedFiles:     []string{"cmd/gromit/review.go", "internal/pipeline/review.go"},
		ChangedFilesHash: "sha256:hash",
		LastErrorCode:    "",
		LastErrorMessage: "",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled Entry
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(unmarshaled.ChangedFiles) != len(entry.ChangedFiles) {
		t.Errorf("changed_files length: got %d, want %d", len(unmarshaled.ChangedFiles), len(entry.ChangedFiles))
	}
	for i, f := range entry.ChangedFiles {
		if i >= len(unmarshaled.ChangedFiles) {
			break
		}
		if unmarshaled.ChangedFiles[i] != f {
			t.Errorf("changed_files[%d]: got %q, want %q", i, unmarshaled.ChangedFiles[i], f)
		}
	}
}

func TestEntryJSONMarshalChangedFilesOmitted(t *testing.T) {
	created := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	entry := Entry{
		Branch:           "gromit/test",
		SessionID:        "session123",
		OriginCommand:    "review",
		State:            StateReady,
		Lane:             "code_lane",
		CreatedAt:        created,
		UpdatedAt:        updated,
		AttemptCount:     1,
		RetryCount:       0,
		FifoSeq:          5,
		BaseRef:          "main",
		HeadSHA:          "abc123def",
		ChangedFilesHash: "sha256:hash",
		LastErrorCode:    "",
		LastErrorMessage: "",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal to map failed: %v", err)
	}

	// changed_files should be omitted when empty (due to omitempty tag)
	if _, exists := result["changed_files"]; exists && entry.ChangedFiles == nil {
		t.Errorf("changed_files should be omitted when nil")
	}
}

func TestEntryDiagnosticsOmittedWhenEmpty(t *testing.T) {
	entry := Entry{
		Branch:           "gromit/test",
		SessionID:        "session123",
		OriginCommand:    "review",
		State:            StateReady,
		Lane:             "code_lane",
		CreatedAt:        time.Time{},
		UpdatedAt:        time.Time{},
		AttemptCount:     0,
		RetryCount:       0,
		FifoSeq:          1,
		BaseRef:          "main",
		HeadSHA:          "sha",
		ChangedFilesHash: "sha256:hash",
		LastErrorCode:    "",
		LastErrorMessage: "",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if _, exists := payload["last_error_code"]; exists {
		t.Fatalf("last_error_code should be omitted when empty")
	}
	if _, exists := payload["last_error_message"]; exists {
		t.Fatalf("last_error_message should be omitted when empty")
	}
}

func TestQueueJSONMarshalUnmarshal(t *testing.T) {
	created := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	queue := Queue{
		SchemaVersion: 1,
		UpdatedAt:     updated,
		Entries: []Entry{
			{
				Branch:           "gromit/test",
				SessionID:        "session123",
				OriginCommand:    "review",
				State:            StateReady,
				Lane:             "code_lane",
				CreatedAt:        created,
				UpdatedAt:        updated,
				AttemptCount:     1,
				RetryCount:       0,
				FifoSeq:          5,
				BaseRef:          "main",
				HeadSHA:          "abc123def",
				ChangedFiles:     []string{"cmd/gromit/review.go"},
				ChangedFilesHash: "sha256:hash",
				LastErrorCode:    "",
				LastErrorMessage: "",
			},
		},
	}

	data, err := json.Marshal(queue)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled Queue
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.SchemaVersion != queue.SchemaVersion {
		t.Errorf("schema_version: got %d, want %d", unmarshaled.SchemaVersion, queue.SchemaVersion)
	}
	if len(unmarshaled.Entries) != len(queue.Entries) {
		t.Errorf("entries count: got %d, want %d", len(unmarshaled.Entries), len(queue.Entries))
	}
	if len(unmarshaled.Entries) > 0 && unmarshaled.Entries[0].Branch != queue.Entries[0].Branch {
		t.Errorf("entries[0].branch: got %q, want %q", unmarshaled.Entries[0].Branch, queue.Entries[0].Branch)
	}
}

func TestEntryOrderingMetadata(t *testing.T) {
	created := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	entry := Entry{
		Branch:        "gromit/ordering",
		SessionID:     "session",
		OriginCommand: "ready",
		State:         "ready",
		Lane:          "code_lane",
		BaseRef:       "main",
		HeadSHA:       "abc123",
		FifoSeq:       7,
		CreatedAt:     created,
		UpdatedAt:     updated,
	}

	meta := entry.Ordering()
	if meta.Sequence != entry.FifoSeq {
		t.Fatalf("sequence = %d, want %d", meta.Sequence, entry.FifoSeq)
	}
	if !meta.CreatedAt.Equal(entry.CreatedAt) {
		t.Fatalf("created_at = %v, want %v", meta.CreatedAt, entry.CreatedAt)
	}
	if !meta.UpdatedAt.Equal(entry.UpdatedAt) {
		t.Fatalf("updated_at = %v, want %v", meta.UpdatedAt, entry.UpdatedAt)
	}
}
