package integrationqueue

import (
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
