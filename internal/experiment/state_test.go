package experiment

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStateReturnsZeroStateWhenMissing(t *testing.T) {
    dir := t.TempDir()

    state, err := LoadState(dir, "exp-1")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if state == nil {
        t.Fatalf("expected non-nil state")
    }
    if len(state.Arms) != 0 {
        t.Fatalf("expected zero arms for missing state, got %d", len(state.Arms))
    }
}

func TestSaveStateWritesStateFile(t *testing.T) {
	dir := t.TempDir()
	state := &BanditState{
		Arms: []ArmState{
			{ID: "control", Successes: 3, Failures: 1},
		},
	}

	if err := SaveState(dir, "exp-1", state); err != nil {
		t.Fatalf("SaveState error: %v", err)
	}

	path := filepath.Join(dir, "exp-1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved state: %v", err)
	}

	var loaded BanditState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal saved state: %v", err)
	}

	if len(loaded.Arms) != 1 {
		t.Fatalf("expected 1 arm, got %d", len(loaded.Arms))
	}
	if loaded.Arms[0].ID != "control" {
		t.Fatalf("expected arm ID control, got %q", loaded.Arms[0].ID)
	}
}
