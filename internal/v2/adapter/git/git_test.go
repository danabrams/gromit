package git

import "testing"

func TestDiffResponseIncludesSummary(t *testing.T) {
	resp := DiffResponse{Diff: "diff", Summary: "summary"}
	if resp.Summary != "summary" {
		t.Fatalf("expected summary 'summary', got %q", resp.Summary)
	}
}
