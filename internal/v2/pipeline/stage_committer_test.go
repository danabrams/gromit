package pipeline

import (
	"context"
	"testing"
)

type fakeStageCommitterGit struct {
	statusOut  string
	statusErr  error
	commitMsg  string
	commitErr  error
	commitHash string
}

func (f *fakeStageCommitterGit) Status(_ context.Context, _ string) (string, error) {
	if f.statusErr != nil {
		return "", f.statusErr
	}
	return f.statusOut, nil
}

func (f *fakeStageCommitterGit) Commit(_ context.Context, _ string, message string) (string, error) {
	f.commitMsg = message
	if f.commitErr != nil {
		return "", f.commitErr
	}
	hash := f.commitHash
	if hash == "" {
		hash = "deadbeef"
	}
	return hash, nil
}

func TestStageCommitter_noOpWhenNoChanges(t *testing.T) {
	git := &fakeStageCommitterGit{statusOut: ""}
	sc := &StageCommitter{Git: git}

	err := sc.CommitStage(context.Background(), "/tmp/wt", "bead-1", "build", 1, "Proceed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if git.commitMsg != "" {
		t.Errorf("expected no commit, got message %q", git.commitMsg)
	}
}
