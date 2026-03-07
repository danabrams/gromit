package pipeline

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
)

type fakeSquashGit struct {
	logEntries  []adapter.LogEntry
	logErr      error
	squashCalls []int
	squashErr   error
	commitCalls []string
	commitErr   error
}

func (f *fakeSquashGit) Log(_ context.Context, _ string, _ int) ([]adapter.LogEntry, error) {
	if f.logErr != nil {
		return nil, f.logErr
	}
	return f.logEntries, nil
}

func (f *fakeSquashGit) SquashCommits(_ context.Context, _ string, count int) error {
	f.squashCalls = append(f.squashCalls, count)
	return f.squashErr
}

func (f *fakeSquashGit) Commit(_ context.Context, _ string, message string) (string, error) {
	f.commitCalls = append(f.commitCalls, message)
	if f.commitErr != nil {
		return "", f.commitErr
	}
	return "squash-commit", nil
}

func TestSquashPerBead_noOpWhenNoStructuredCommits(t *testing.T) {
	fake := &fakeSquashGit{
		logEntries: []adapter.LogEntry{
			{Hash: "abc123", Message: "initial commit"},
			{Hash: "def456", Message: "add readme"},
		},
	}

	err := SquashPerBead(context.Background(), fake, "/tmp/wt", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.squashCalls) != 0 {
		t.Errorf("expected no squash calls, got %v", fake.squashCalls)
	}
	if len(fake.commitCalls) != 0 {
		t.Errorf("expected no commit calls, got %v", fake.commitCalls)
	}
}

func TestSquashPerBead_emptyBeadsIsNoop(t *testing.T) {
	fake := &fakeSquashGit{
		logEntries: []adapter.LogEntry{
			{Hash: "ccc", Message: "[bead:001/validate/iter:1] Pass"},
			{Hash: "bbb", Message: "[bead:001/build/iter:1] Proceed"},
			{Hash: "aaa", Message: "initial commit"},
		},
	}

	err := SquashPerBead(context.Background(), fake, "/tmp/wt", []presentation.BeadSummary{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.squashCalls) != 0 {
		t.Errorf("expected no squash calls, got %v", fake.squashCalls)
	}
	if len(fake.commitCalls) != 0 {
		t.Errorf("expected no commit calls, got %v", fake.commitCalls)
	}
}

func TestSquashPerBead_squashesStructuredCommitsAndCommits(t *testing.T) {
	fake := &fakeSquashGit{
		logEntries: []adapter.LogEntry{
			{Hash: "ccc", Message: "[bead:001/validate/iter:1] Pass"},
			{Hash: "bbb", Message: "[bead:001/build/iter:1] Proceed"},
			{Hash: "aaa", Message: "initial commit"},
		},
	}
	beads := []presentation.BeadSummary{{ID: "001", Title: "My Feature"}}

	err := SquashPerBead(context.Background(), fake, "/tmp/wt", beads)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.squashCalls) != 1 || fake.squashCalls[0] != 2 {
		t.Errorf("squashCalls = %v, want [2]", fake.squashCalls)
	}
	if len(fake.commitCalls) != 1 || fake.commitCalls[0] != "bead 001: My Feature" {
		t.Errorf("commitCalls = %v, want [\"bead 001: My Feature\"]", fake.commitCalls)
	}
}
