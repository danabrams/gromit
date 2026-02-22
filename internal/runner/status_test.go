package runner

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestPrintStatus_ShowsModelAndTimeBudget verifies that PrintStatus reads
// status.json and includes both model and time budget in the display output.
// The round-trip through the file is verified first, then the test fails
// because PrintStatus does not yet exist — the GREEN phase will implement it.
func TestPrintStatus_ShowsModelAndTimeBudget(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write status.json with model and time budget via the real StatusWriter.
	sw, err := NewStatusWriter(gromitDir)
	if err != nil {
		t.Fatalf("NewStatusWriter: %v", err)
	}
	if err := sw.Write(5, "bead-display", "Test display fields", "opus", true, 10, 45); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify round-trip: model and time budget survive Write → ReadStatus.
	status, err := ReadStatus(gromitDir)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if status.Model != "opus" {
		t.Fatalf("round-trip lost model: got %q, want %q", status.Model, "opus")
	}
	if status.TimeBudgetMinutes != 45 {
		t.Fatalf("round-trip lost time budget: got %d, want 45", status.TimeBudgetMinutes)
	}

	// Verify the display format includes both fields.
	display := formatRun(status)
	if !strings.Contains(display, "Model:    opus") {
		t.Errorf("formatRun missing model; got:\n%s", display)
	}
	if !strings.Contains(display, "of 45m elapsed") {
		t.Errorf("formatRun missing time budget; got:\n%s", display)
	}

	// PrintStatus must exist as the exported entry point for "gromit status" display.
	// This call site will be filled in during the GREEN phase.
	cfg := &config.Config{}
	var printStatus func(string, *config.Config, io.Writer, func(int) bool) error
	if printStatus == nil {
		t.Fatal("PrintStatus not yet implemented; need exported function " +
			"PrintStatus(gromitDir string, cfg *config.Config, w io.Writer, processChecker func(int) bool) error")
	}
	var buf strings.Builder
	if err := printStatus(gromitDir, cfg, &buf, func(int) bool { return true }); err != nil {
		t.Fatalf("PrintStatus: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Model:    opus") {
		t.Errorf("PrintStatus output missing model; got:\n%s", output)
	}
	if !strings.Contains(output, "of 45m elapsed") {
		t.Errorf("PrintStatus output missing time budget; got:\n%s", output)
	}
}
