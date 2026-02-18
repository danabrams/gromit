package runbook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewEntrySetsIDAndTimestamp(t *testing.T) {
	now := time.Date(2026, time.February, 18, 12, 0, 0, 0, time.FixedZone("test", -5*60*60))
	beadID := "beads-abc"

	entry := NewEntry(beadID, now)

	wantID := fmt.Sprintf("rb-%d-%s", now.Unix(), beadID)
	if entry.ID != wantID {
		t.Fatalf("expected id %q, got %q", wantID, entry.ID)
	}
	if !entry.Timestamp.Equal(now.UTC()) {
		t.Fatalf("expected timestamp %s, got %s", now.UTC(), entry.Timestamp)
	}
}

func TestTruncateOutputCapsAt5KB(t *testing.T) {
	input := strings.Repeat("a", 6000)
	output := truncateOutput(input)

	if len(output) != 5120 {
		t.Fatalf("expected 5120 bytes, got %d", len(output))
	}
	if output != input[len(input)-5120:] {
		t.Fatalf("expected tail of input to be returned")
	}
}

func TestAppendWritesJSONL(t *testing.T) {
	gromitDir := t.TempDir()
	entry := Entry{ID: "rb-1-beads-abc", BeadID: "beads-abc"}

	if err := Append(gromitDir, entry); err != nil {
		t.Fatalf("append failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(gromitDir, "runbooks.jsonl"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("expected newline-terminated jsonl line")
	}
	if !strings.Contains(string(data), "\"id\":\"rb-1-beads-abc\"") {
		t.Fatalf("expected entry json to contain id")
	}
}
