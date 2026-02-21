package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/state"
)

func TestSyncArchivedHashesFromState(t *testing.T) {
	tmpDir := t.TempDir()

	sf, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile(state) failed: %v", err)
	}
	if err := sf.Load(); err != nil {
		t.Fatalf("Load(state) failed: %v", err)
	}
	sf.AddArchivedHashes([]string{"hash-b", "hash-a"})
	if err := sf.Save(); err != nil {
		t.Fatalf("Save(state) failed: %v", err)
	}

	lf, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile(learnings) failed: %v", err)
	}

	r := &Runner{
		renderer: &mockPromptRenderer{LearningsFile: lf},
	}
	st := &runLoopState{sf: sf}

	r.syncArchivedHashesFromState(st)

	archived := lf.GetArchivedHashes()
	if len(archived) != 2 {
		t.Fatalf("expected 2 archived hashes in learnings, got %d", len(archived))
	}
	if !archived["hash-a"] || !archived["hash-b"] {
		t.Fatalf("expected synced archived hashes in learnings, got %#v", archived)
	}
}

func TestPersistArchivedHashesToState(t *testing.T) {
	tmpDir := t.TempDir()

	sf, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile(state) failed: %v", err)
	}
	if err := sf.Load(); err != nil {
		t.Fatalf("Load(state) failed: %v", err)
	}
	sf.AddArchivedHashes([]string{"hash-a"})
	if err := sf.Save(); err != nil {
		t.Fatalf("Save(state) failed: %v", err)
	}

	lf, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile(learnings) failed: %v", err)
	}
	lf.SetArchivedHashes([]string{"hash-a", "hash-b"})

	r := &Runner{
		renderer: &mockPromptRenderer{LearningsFile: lf},
	}
	st := &runLoopState{sf: sf}

	r.persistArchivedHashesToState(st)

	verifyState, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile(state verify) failed: %v", err)
	}
	if err := verifyState.Load(); err != nil {
		t.Fatalf("Load(state verify) failed: %v", err)
	}

	archived := verifyState.GetArchivedHashes()
	if len(archived) != 2 {
		t.Fatalf("expected 2 archived hashes in state, got %d", len(archived))
	}
	if !archived["hash-a"] || !archived["hash-b"] {
		t.Fatalf("expected merged archived hashes in state, got %#v", archived)
	}
}
