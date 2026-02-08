package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFile(t *testing.T) {
	f, _ := NewFile("/tmp/test-gromit")
	if f.path != "/tmp/test-gromit/state.json" {
		t.Errorf("expected path /tmp/test-gromit/state.json, got %s", f.path)
	}
}

func TestLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	if err := f.Load(); err != nil {
		t.Errorf("loading non-existent state should not error: %v", err)
	}

	if !f.LastRetro().IsZero() {
		t.Error("last retro should be zero for new state")
	}
}

func TestRecordRetroAndLoad(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	before := time.Now()
	if err := f.RecordRetro(); err != nil {
		t.Fatalf("recording retro: %v", err)
	}
	after := time.Now()

	// Verify file was created
	if _, err := os.Stat(filepath.Join(dir, "state.json")); err != nil {
		t.Fatalf("state.json should exist: %v", err)
	}

	// Verify the time was recorded
	if f.LastRetro().Before(before) || f.LastRetro().After(after) {
		t.Error("last retro time should be between before and after")
	}

	// Load in a new File and verify persistence
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	// Allow 1 second tolerance for JSON serialization rounding
	diff := f.LastRetro().Sub(f2.LastRetro())
	if diff < -time.Second || diff > time.Second {
		t.Errorf("loaded retro time should match: got %v, want %v", f2.LastRetro(), f.LastRetro())
	}
}

func TestLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	f, _ := NewFile(dir)
	if err := f.Load(); err == nil {
		t.Error("loading corrupt state should return error")
	}
}

func TestReviewState(t *testing.T) {
	tmpDir := t.TempDir()
	sf, _ := NewFile(tmpDir)

	// Initial state has no review info
	if err := sf.Load(); err != nil {
		t.Fatal(err)
	}
	if sf.LastReviewCommit() != "" {
		t.Errorf("expected empty last review commit, got %q", sf.LastReviewCommit())
	}
	if sf.LastReviewIteration() != 0 {
		t.Errorf("expected 0 last review iteration, got %d", sf.LastReviewIteration())
	}
	if sf.IterationsSinceReview() != 0 {
		t.Errorf("expected 0 iterations since review, got %d", sf.IterationsSinceReview())
	}

	// Record a review
	if err := sf.RecordReview("abc123", 5); err != nil {
		t.Fatal(err)
	}

	// Reload and check
	sf2, _ := NewFile(tmpDir)
	if err := sf2.Load(); err != nil {
		t.Fatal(err)
	}
	if sf2.LastReviewCommit() != "abc123" {
		t.Errorf("expected 'abc123', got %q", sf2.LastReviewCommit())
	}
	if sf2.LastReviewIteration() != 5 {
		t.Errorf("expected 5, got %d", sf2.LastReviewIteration())
	}
}

func TestIncrementIterationsSinceReview(t *testing.T) {
	tmpDir := t.TempDir()
	sf, _ := NewFile(tmpDir)
	if err := sf.Load(); err != nil {
		t.Fatal(err)
	}

	sf.IncrementIterationsSinceReview()
	if sf.IterationsSinceReview() != 1 {
		t.Errorf("expected 1, got %d", sf.IterationsSinceReview())
	}

	sf.IncrementIterationsSinceReview()
	sf.IncrementIterationsSinceReview()
	if sf.IterationsSinceReview() != 3 {
		t.Errorf("expected 3, got %d", sf.IterationsSinceReview())
	}

	// RecordReview resets counter
	if err := sf.RecordReview("def456", 8); err != nil {
		t.Fatal(err)
	}
	if sf.IterationsSinceReview() != 0 {
		t.Errorf("expected 0 after RecordReview, got %d", sf.IterationsSinceReview())
	}
}

func TestSaveAutoStampsUpdatedAt(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	before := time.Now()
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}
	after := time.Now()

	// Check UpdatedAt was stamped
	if f.state.UpdatedAt.Before(before) || f.state.UpdatedAt.After(after) {
		t.Error("UpdatedAt should be between before and after")
	}

	// Load in a new File and verify UpdatedAt persisted
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	// Allow 1 second tolerance for JSON serialization rounding
	diff := f.state.UpdatedAt.Sub(f2.state.UpdatedAt)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("loaded UpdatedAt should match: got %v, want %v", f2.state.UpdatedAt, f.state.UpdatedAt)
	}
}

func TestCleanExitPersistence(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Set CleanExit to true and save
	f.state.CleanExit = true
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Load in a new File and verify CleanExit persisted
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	if !f2.state.CleanExit {
		t.Error("CleanExit should be true after load")
	}

	// Set to false and verify it persists
	f2.state.CleanExit = false
	if err := f2.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	f3, _ := NewFile(dir)
	if err := f3.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	if f3.state.CleanExit {
		t.Error("CleanExit should be false after load")
	}
}
