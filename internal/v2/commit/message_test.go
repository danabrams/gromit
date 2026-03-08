package commit

import "testing"

func TestFormatMessage(t *testing.T) {
	got := FormatMessage("gromit-o00gs", "build", 2, "Pass")
	want := "[bead:gromit-o00gs/build/iter:2] Pass"
	if got != want {
		t.Fatalf("FormatMessage() = %q, want %q", got, want)
	}
}

func TestParseMessage(t *testing.T) {
	parsed, err := ParseMessage("[bead:gromit-o00gs/review/iter:3] Retry")
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if parsed.BeadID != "gromit-o00gs" {
		t.Fatalf("BeadID = %q, want %q", parsed.BeadID, "gromit-o00gs")
	}
	if parsed.StageName != "review" {
		t.Fatalf("StageName = %q, want %q", parsed.StageName, "review")
	}
	if parsed.Iteration != 3 {
		t.Fatalf("Iteration = %d, want %d", parsed.Iteration, 3)
	}
	if parsed.Decision != "Retry" {
		t.Fatalf("Decision = %q, want %q", parsed.Decision, "Retry")
	}
}

func TestParseMessageValidatesRequiredFields(t *testing.T) {
	tests := []string{
		"[bead:gromit-o00gs/review/iter:0] Retry",
		"[bead:gromit-o00gs/review/iter:3]    ",
	}

	for _, input := range tests {
		if _, err := ParseMessage(input); err == nil {
			t.Fatalf("ParseMessage(%q) error = nil, want non-nil", input)
		}
	}
}

func TestFormatMessageSpecStage(t *testing.T) {
	got := FormatMessage("", "plan", 1, "Proceed")
	want := "[spec/plan/iter:1] Proceed"
	if got != want {
		t.Fatalf("FormatMessage() = %q, want %q", got, want)
	}
}

func TestParseMessageSpecStage(t *testing.T) {
	parsed, err := ParseMessage("[spec/plan/iter:1] Proceed")
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if parsed.BeadID != "" {
		t.Fatalf("BeadID = %q, want empty", parsed.BeadID)
	}
	if parsed.StageName != "plan" {
		t.Fatalf("StageName = %q, want %q", parsed.StageName, "plan")
	}
	if parsed.Iteration != 1 {
		t.Fatalf("Iteration = %d, want %d", parsed.Iteration, 1)
	}
	if parsed.Decision != "Proceed" {
		t.Fatalf("Decision = %q, want %q", parsed.Decision, "Proceed")
	}
}
