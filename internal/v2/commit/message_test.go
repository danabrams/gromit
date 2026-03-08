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
