package present

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
)

type fakePresentSquashGit struct {
	logEntries  []adapter.LogEntry
	logErr      error
	squashCalls []int
	squashErr   error
	commitCalls []string
	commitErr   error
}

func (f *fakePresentSquashGit) Log(_ context.Context, _ string, _ int) ([]adapter.LogEntry, error) {
	if f.logErr != nil {
		return nil, f.logErr
	}
	return f.logEntries, nil
}

func (f *fakePresentSquashGit) SquashCommits(_ context.Context, _ string, count int) error {
	f.squashCalls = append(f.squashCalls, count)
	return f.squashErr
}

func (f *fakePresentSquashGit) Commit(_ context.Context, _ string, message string) (string, error) {
	f.commitCalls = append(f.commitCalls, message)
	if f.commitErr != nil {
		return "", f.commitErr
	}
	return "squash-commit", nil
}

func TestSquashPerBead_squashesEachBeadBoundary(t *testing.T) {
	fake := &fakePresentSquashGit{
		logEntries: []adapter.LogEntry{
			{Hash: "h4", Message: "[bead:002/review/iter:1] Proceed"},
			{Hash: "h3", Message: "[bead:002/validate/iter:1] Proceed"},
			{Hash: "h2", Message: "[bead:001/review/iter:1] Proceed"},
			{Hash: "h1", Message: "[bead:001/build/iter:1] Proceed"},
			{Hash: "h0", Message: "initial commit"},
		},
	}
	beads := []presentation.BeadSummary{
		{ID: "001", Title: "First Bead"},
		{ID: "002", Title: "Second Bead"},
	}

	err := squashPerBead(context.Background(), fake, "/tmp/wt", beads)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.squashCalls) != 2 || fake.squashCalls[0] != 2 || fake.squashCalls[1] != 2 {
		t.Errorf("squashCalls = %v, want [2 2]", fake.squashCalls)
	}
	wantMessages := []string{"bead 002: Second Bead", "bead 001: First Bead"}
	if len(fake.commitCalls) != len(wantMessages) {
		t.Fatalf("commitCalls = %v, want %v", fake.commitCalls, wantMessages)
	}
	for i := range wantMessages {
		if fake.commitCalls[i] != wantMessages[i] {
			t.Fatalf("commitCalls[%d] = %q, want %q", i, fake.commitCalls[i], wantMessages[i])
		}
	}
}
