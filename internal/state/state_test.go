package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFile(t *testing.T) {
	f, _ := NewFile("/tmp/test-ralph")
	if f.path != "/tmp/test-ralph/state.json" {
		t.Errorf("expected path /tmp/test-ralph/state.json, got %s", f.path)
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
