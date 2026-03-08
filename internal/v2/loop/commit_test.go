package loop

import "testing"

func TestCommitMessageBuild(t *testing.T) {
	msg := CommitMessage{
		BeadID:    "gromit-mo15p",
		StageName: "build",
		Iteration: 2,
		Decision:  "Proceed",
	}

	got := msg.Build()
	want := "[bead:gromit-mo15p/build/iter:2] Proceed"
	if got != want {
		t.Fatalf("Build() = %q, want %q", got, want)
	}
}
