package commit

import "testing"

func TestFormatMessage(t *testing.T) {
	got := FormatMessage("gromit-o00gs", "build", 2, "Pass")
	want := "[bead:gromit-o00gs/build/iter:2] Pass"
	if got != want {
		t.Fatalf("FormatMessage() = %q, want %q", got, want)
	}
}
