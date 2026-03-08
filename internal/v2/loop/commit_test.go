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

func TestParseCommitMessage(t *testing.T) {
	got, ok := ParseCommitMessage("[bead:gromit-mo15p/review/iter:3] Retry")
	if !ok {
		t.Fatal("ParseCommitMessage() ok = false, want true")
	}
	if got.BeadID != "gromit-mo15p" {
		t.Fatalf("BeadID = %q, want %q", got.BeadID, "gromit-mo15p")
	}
	if got.StageName != "review" {
		t.Fatalf("StageName = %q, want %q", got.StageName, "review")
	}
	if got.Iteration != 3 {
		t.Fatalf("Iteration = %d, want %d", got.Iteration, 3)
	}
	if got.Decision != "Retry" {
		t.Fatalf("Decision = %q, want %q", got.Decision, "Retry")
	}
}

func TestParseCommitMessageRejectsNonPositiveIteration(t *testing.T) {
	_, ok := ParseCommitMessage("[bead:gromit-mo15p/review/iter:0] Retry")
	if ok {
		t.Fatal("ParseCommitMessage() ok = true, want false")
	}
}
