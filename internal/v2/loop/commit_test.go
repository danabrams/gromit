package loop

import "testing"

func TestBuildCommitMessage(t *testing.T) {
	tests := []struct {
		name string
		in   CommitMessage
		want string
	}{
		{
			name: "bead stage",
			in: CommitMessage{
				BeadID:    "gromit-mo15p",
				StageName: "build",
				Iteration: 2,
				Decision:  "Proceed",
			},
			want: "[bead:gromit-mo15p/build/iter:2] Proceed",
		},
		{
			name: "spec stage",
			in: CommitMessage{
				StageName: "plan",
				Iteration: 1,
				Decision:  "Proceed",
			},
			want: "[bead:spec/plan/iter:1] Proceed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCommitMessage(tt.in)
			if got != tt.want {
				t.Fatalf("BuildCommitMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

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

func TestParseCommitMessageRejectsInvalidBeadID(t *testing.T) {
	_, ok := ParseCommitMessage("[bead:gromit mo15p/review/iter:1] Retry")
	if ok {
		t.Fatal("ParseCommitMessage() ok = true, want false")
	}
}
