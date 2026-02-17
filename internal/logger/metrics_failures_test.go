package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadConsecutiveFailureCountsFromMetrics(t *testing.T) {
	dir := t.TempDir()
	content := `{"bead_id":"bead-a","success":false}
{"bead_id":"bead-a","success":false}
{"bead_id":"bead-a","success":true}
{"bead_id":"bead-a","success":false}
{"bead_id":"bead-b","success":false}
{"bead_id":"bead-b","success":false}
`

	if err := os.WriteFile(filepath.Join(dir, "iteration_metrics.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("write metrics file: %v", err)
	}

	counts, err := ReadConsecutiveFailureCounts(dir)
	if err != nil {
		t.Fatalf("ReadConsecutiveFailureCounts error: %v", err)
	}

	if got := counts["bead-a"]; got != 1 {
		t.Fatalf("bead-a consecutive failures = %d, want 1", got)
	}
	if got := counts["bead-b"]; got != 2 {
		t.Fatalf("bead-b consecutive failures = %d, want 2", got)
	}
}
