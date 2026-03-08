package loop

import "testing"

func TestNextStageIterationBuildsDistinctRetryCommitMessages(t *testing.T) {
	loop := &BeadLoop{}

	first := loop.nextStageIteration("bead-retry", "validate")
	second := loop.nextStageIteration("bead-retry", "validate")

	if first != 1 {
		t.Fatalf("first iteration = %d, want 1", first)
	}
	if second != 2 {
		t.Fatalf("second iteration = %d, want 2", second)
	}

	firstMsg := BuildCommitMessage(CommitMessage{
		BeadID:    "bead-retry",
		StageName: "validate",
		Iteration: first,
		Decision:  "fail",
	})
	secondMsg := BuildCommitMessage(CommitMessage{
		BeadID:    "bead-retry",
		StageName: "validate",
		Iteration: second,
		Decision:  "fail",
	})

	if firstMsg == secondMsg {
		t.Fatalf("retry commit messages must differ, got %q", firstMsg)
	}
	if firstMsg != "[bead:bead-retry/validate/iter:1] fail" {
		t.Fatalf("first retry commit = %q, want %q", firstMsg, "[bead:bead-retry/validate/iter:1] fail")
	}
	if secondMsg != "[bead:bead-retry/validate/iter:2] fail" {
		t.Fatalf("second retry commit = %q, want %q", secondMsg, "[bead:bead-retry/validate/iter:2] fail")
	}
}
