package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadRecurringReviewFixCategories_BySpec(t *testing.T) {
	dir := t.TempDir()
	content := `{"timestamp":"2026-02-21T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","spec_id":"spec:a","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-21T12:01:00Z","type":"review","review_type":"light","iteration":1,"bead_id":"b1","model":"sonnet","passed":true,"fixes_applied":2,"fix_categories":["error_handling","nil_checks"],"beads_created":0,"backlog_created":0,"duration_ms":100}
{"timestamp":"2026-02-21T12:02:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","spec_id":"spec:a","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-21T12:03:00Z","type":"review","review_type":"light","iteration":2,"bead_id":"b2","model":"sonnet","passed":true,"fixes_applied":1,"fix_categories":["error_handling"],"beads_created":0,"backlog_created":0,"duration_ms":100}
{"timestamp":"2026-02-21T12:04:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","spec_id":"spec:b","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-21T12:05:00Z","type":"review","review_type":"light","iteration":3,"bead_id":"b3","model":"sonnet","passed":true,"fixes_applied":1,"fix_categories":["test_quality"],"beads_created":0,"backlog_created":0,"duration_ms":100}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260221-120000.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadRecurringReviewFixCategories(dir, "b-current", "spec:a", 2, 3)
	if err != nil {
		t.Fatalf("ReadRecurringReviewFixCategories() error = %v", err)
	}

	if len(got) != 1 || got[0] != "error_handling" {
		t.Fatalf("ReadRecurringReviewFixCategories() = %v, want [error_handling]", got)
	}
}

func TestReadRecurringReviewFixCategories_AppliesCapsAndCurrentBeadFilter(t *testing.T) {
	dir := t.TempDir()
	content := `{"timestamp":"2026-02-21T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-21T12:01:00Z","type":"review","review_type":"light","iteration":1,"bead_id":"b1","model":"sonnet","passed":true,"fixes_applied":3,"fix_categories":["error_handling","nil_checks","test_quality"],"beads_created":0,"backlog_created":0,"duration_ms":100}
{"timestamp":"2026-02-21T12:02:00Z","iteration":2,"bead_id":"b-current","bead_title":"Task 2","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-21T12:03:00Z","type":"review","review_type":"light","iteration":2,"bead_id":"b-current","model":"sonnet","passed":true,"fixes_applied":2,"fix_categories":["error_handling","nil_checks"],"beads_created":0,"backlog_created":0,"duration_ms":100}
{"timestamp":"2026-02-21T12:04:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-21T12:05:00Z","type":"review","review_type":"light","iteration":3,"bead_id":"b3","model":"sonnet","passed":true,"fixes_applied":2,"fix_categories":["error_handling","nil_checks"],"beads_created":0,"backlog_created":0,"duration_ms":100}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260221-130000.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadRecurringReviewFixCategories(dir, "b-current", "", 1, 2)
	if err != nil {
		t.Fatalf("ReadRecurringReviewFixCategories() error = %v", err)
	}

	want := []string{"error_handling", "nil_checks"}
	if len(got) != len(want) {
		t.Fatalf("len(ReadRecurringReviewFixCategories()) = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ReadRecurringReviewFixCategories()[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}
