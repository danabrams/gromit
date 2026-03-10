package provenance

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFSTracker_RecordAndCheck(t *testing.T) {
	dir := t.TempDir()
	tracker := NewFSTracker(filepath.Join(dir, "provenance.json"))

	rec := Record{
		FactID:    "arch-001",
		Artifact:  "architecture",
		Category:  "observed",
		GitSHA:    "abc123",
		Timestamp: time.Now(),
		Extractor: "go-module",
		InputHash: "sha256:deadbeef",
	}
	if err := tracker.Record(rec); err != nil {
		t.Fatalf("Record: %v", err)
	}

	got, err := tracker.Check("architecture")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got.GitSHA != "abc123" {
		t.Errorf("GitSHA = %q, want %q", got.GitSHA, "abc123")
	}
}

func TestFSTracker_IsFresh(t *testing.T) {
	dir := t.TempDir()
	tracker := NewFSTracker(filepath.Join(dir, "provenance.json"))

	tracker.Record(Record{
		Artifact: "architecture",
		GitSHA:   "abc123",
	})

	fresh, err := tracker.IsFresh("architecture", "abc123")
	if err != nil {
		t.Fatalf("IsFresh: %v", err)
	}
	if !fresh {
		t.Error("should be fresh with same SHA")
	}

	fresh, _ = tracker.IsFresh("architecture", "def456")
	if fresh {
		t.Error("should not be fresh with different SHA")
	}
}

func TestFSTracker_RecordWithCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "provenance.json")
	os.WriteFile(path, []byte("{corrupt json!!!"), 0o644)

	tracker := NewFSTracker(path)
	err := tracker.Record(Record{Artifact: "test", GitSHA: "abc"})
	if err == nil {
		t.Error("expected error for corrupt provenance file")
	}
}

func TestFSTracker_CheckMissing(t *testing.T) {
	dir := t.TempDir()
	tracker := NewFSTracker(filepath.Join(dir, "provenance.json"))

	_, err := tracker.Check("nonexistent")
	if err == nil {
		t.Error("expected error for missing artifact")
	}
}
