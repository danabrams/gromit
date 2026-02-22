package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

// TestPrintStatus_IncludesPipelineSection verifies that PrintStatus outputs
// the Pipeline section alongside the Run section. The current implementation
// only formats the Run section; this test drives adding pipeline.ReadStatus
// integration so the output includes "Pipeline:" with bead/spec/plan counts.
func TestPrintStatus_IncludesPipelineSection(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write a status.json so PrintStatus doesn't take the "no status file" path.
	sw, err := NewStatusWriter(gromitDir)
	if err != nil {
		t.Fatalf("NewStatusWriter: %v", err)
	}
	if err := sw.Write(1, "bead-pipe", "Pipeline test", "haiku", true, 0, 0); err != nil {
		t.Fatalf("Write: %v", err)
	}

	cfg := &config.Config{}
	cfg.Paths.Specs = filepath.Join(tmpDir, "specs")
	cfg.Paths.Plans = filepath.Join(tmpDir, "plans")

	var buf strings.Builder
	if err := PrintStatus(gromitDir, cfg, &buf, func(int) bool { return true }); err != nil {
		t.Fatalf("PrintStatus: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Pipeline:") {
		t.Errorf("PrintStatus output missing Pipeline section; got:\n%s", output)
	}
}

func TestStatusWriter_ScopeLabelRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	sw, err := NewStatusWriter(gromitDir)
	if err != nil {
		t.Fatalf("NewStatusWriter: %v", err)
	}
	sw.SetScopeLabel("spec:auth")
	if err := sw.Write(2, "bead-123", "Scoped bead", "sonnet", true, 0, 0); err != nil {
		t.Fatalf("Write: %v", err)
	}

	status, err := ReadStatus(gromitDir)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if status.ScopeLabel != "spec:auth" {
		t.Fatalf("ScopeLabel = %q, want %q", status.ScopeLabel, "spec:auth")
	}
}

func TestPrintStatus_RecomputesScopedIterationTotalOnEachCall(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	sw, err := NewStatusWriter(gromitDir)
	if err != nil {
		t.Fatalf("NewStatusWriter: %v", err)
	}
	sw.SetScopeLabel("spec:auth")
	sw.SetIterationTotal(1) // intentionally stale to verify live recomputation
	if err := sw.Write(1, "bead-scoped", "Scoped run", "haiku", true, 0, 0); err != nil {
		t.Fatalf("Write: %v", err)
	}

	origFactory := newBeadClientForStatus
	t.Cleanup(func() { newBeadClientForStatus = origFactory })
	newBeadClientForStatus = func() (*bead.Client, error) {
		return &bead.Client{
			RunFn: func(args ...string) (string, error) {
				return `[
					{"id":"task-1","title":"A","issue_type":"task","status":"open"},
					{"id":"task-2","title":"B","issue_type":"task","status":"open"},
					{"id":"task-3","title":"C","issue_type":"task","status":"open"},
					{"id":"task-4","title":"D","issue_type":"task","status":"closed"},
					{"id":"epic-1","title":"E","issue_type":"epic","status":"open"}
				]`, nil
			},
		}, nil
	}

	cfg := &config.Config{}
	cfg.Paths.Specs = filepath.Join(tmpDir, "specs")
	cfg.Paths.Plans = filepath.Join(tmpDir, "plans")

	var buf strings.Builder
	if err := PrintStatus(gromitDir, cfg, &buf, func(int) bool { return true }); err != nil {
		t.Fatalf("PrintStatus: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Run: iteration 1 of 3") {
		t.Fatalf("expected recomputed scoped total in run line, got:\n%s", output)
	}
}

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
	printStatus := PrintStatus
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

// TestPrintStatus_IncludesSPCSection verifies that PrintStatus reads process
// trend data from the logs directory and includes an SPC section in the output.
// Even with no log data, the output should contain "SPC:" with a "(no data)"
// indicator. The current implementation only outputs Run, Pipeline, and Health
// sections — this test drives adding logger.ReadProcessTrend integration.
func TestPrintStatus_IncludesSPCSection(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write a status.json so PrintStatus takes the active-run path.
	sw, err := NewStatusWriter(gromitDir)
	if err != nil {
		t.Fatalf("NewStatusWriter: %v", err)
	}
	if err := sw.Write(3, "bead-spc", "SPC test", "sonnet", true, 0, 0); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Create an empty logs directory — no iteration logs means SPC has no data.
	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll logs: %v", err)
	}

	cfg := &config.Config{}
	cfg.Paths.Specs = filepath.Join(tmpDir, "specs")
	cfg.Paths.Plans = filepath.Join(tmpDir, "plans")
	cfg.Paths.Logs = logsDir

	var buf strings.Builder
	if err := PrintStatus(gromitDir, cfg, &buf, func(int) bool { return true }); err != nil {
		t.Fatalf("PrintStatus: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "SPC:") {
		t.Errorf("PrintStatus output missing SPC section; got:\n%s", output)
	}
}

// TestPrintStatus_StalePIDWarnsAndDeletesFile verifies that when status.json
// says running:true but the processChecker reports the PID as dead, PrintStatus
// outputs a stale-run warning with bead details, prints a removal message, and
// deletes the status.json file from disk.
func TestPrintStatus_StalePIDWarnsAndDeletesFile(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write a status.json that claims to be running.
	sw, err := NewStatusWriter(gromitDir)
	if err != nil {
		t.Fatalf("NewStatusWriter: %v", err)
	}
	if err := sw.Write(4, "bead-stale-1", "Stale bead title", "sonnet", true, 0, 0); err != nil {
		t.Fatalf("Write: %v", err)
	}

	statusPath := filepath.Join(gromitDir, "status.json")

	cfg := &config.Config{}
	cfg.Paths.Specs = filepath.Join(tmpDir, "specs")
	cfg.Paths.Plans = filepath.Join(tmpDir, "plans")

	var buf strings.Builder
	// processChecker returns false → PID is dead → stale detection should trigger.
	if err := PrintStatus(gromitDir, cfg, &buf, func(int) bool { return false }); err != nil {
		t.Fatalf("PrintStatus: %v", err)
	}

	output := buf.String()

	// Should warn about the stale run with bead details.
	if !strings.Contains(output, "Warning: stale run detected") {
		t.Errorf("expected stale-run warning; got:\n%s", output)
	}
	if !strings.Contains(output, "bead-stale-1") {
		t.Errorf("expected bead ID in stale warning; got:\n%s", output)
	}
	if !strings.Contains(output, "Stale bead title") {
		t.Errorf("expected bead title in stale warning; got:\n%s", output)
	}

	// Should announce file removal.
	if !strings.Contains(output, "Removing stale status file") {
		t.Errorf("expected removal message; got:\n%s", output)
	}

	// Should show "not running" after cleanup.
	if !strings.Contains(output, "Run: not running") {
		t.Errorf("expected 'Run: not running' after stale cleanup; got:\n%s", output)
	}

	// The status.json file must have been deleted from disk.
	if _, err := os.Stat(statusPath); !os.IsNotExist(err) {
		t.Errorf("expected status.json to be deleted; stat returned: %v", err)
	}
}

// TestPrintStatus_ReadsStateFilesForHealthSection verifies that PrintStatus
// reads state.json (for IterationsSinceReview) and interactive-state.json
// (for LastRetro) and includes a Health section in the output. The current
// implementation only reads status.json and pipeline — this test drives
// adding state file reads so the output includes "Health:" with review and
// retro data.
func TestPrintStatus_ReadsStateFilesForHealthSection(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write a status.json so PrintStatus takes the active-run path.
	sw, err := NewStatusWriter(gromitDir)
	if err != nil {
		t.Fatalf("NewStatusWriter: %v", err)
	}
	if err := sw.Write(2, "bead-health", "Health test", "sonnet", true, 0, 0); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Write state.json with iterations_since_review.
	stateJSON := `{"iterations_since_review": 7}`
	if err := os.WriteFile(filepath.Join(gromitDir, "state.json"), []byte(stateJSON), 0644); err != nil {
		t.Fatalf("WriteFile state.json: %v", err)
	}

	// Write interactive-state.json with last_retro timestamp.
	retroTime := time.Now().Add(-3 * time.Hour).Format(time.RFC3339)
	interactiveJSON := `{"last_retro": "` + retroTime + `"}`
	if err := os.WriteFile(filepath.Join(gromitDir, "interactive-state.json"), []byte(interactiveJSON), 0644); err != nil {
		t.Fatalf("WriteFile interactive-state.json: %v", err)
	}

	cfg := &config.Config{}
	cfg.Paths.Specs = filepath.Join(tmpDir, "specs")
	cfg.Paths.Plans = filepath.Join(tmpDir, "plans")

	var buf strings.Builder
	if err := PrintStatus(gromitDir, cfg, &buf, func(int) bool { return true }); err != nil {
		t.Fatalf("PrintStatus: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Health:") {
		t.Errorf("PrintStatus output missing Health section; got:\n%s", output)
	}
	if !strings.Contains(output, "Last review: 7 iterations ago") {
		t.Errorf("PrintStatus output missing review count; got:\n%s", output)
	}
	if !strings.Contains(output, "Last retro:") {
		t.Errorf("PrintStatus output missing retro info; got:\n%s", output)
	}
}

// TestPrintStatus_NoStatusFile_ShowsStructuredOutput verifies that when no
// status.json exists, PrintStatus still shows all structured sections (Run,
// Pipeline, Health) instead of the old "No status file found" early return.
func TestPrintStatus_NoStatusFile_ShowsStructuredOutput(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// No status.json created — gromitDir exists but is empty.

	cfg := &config.Config{}
	cfg.Paths.Specs = filepath.Join(tmpDir, "specs")
	cfg.Paths.Plans = filepath.Join(tmpDir, "plans")

	var buf strings.Builder
	if err := PrintStatus(gromitDir, cfg, &buf, nil); err != nil {
		t.Fatalf("PrintStatus: %v", err)
	}

	output := buf.String()

	// Must show structured sections, not the old early-return message.
	if strings.Contains(output, "No status file found") {
		t.Errorf("output should not contain old 'No status file found' message; got:\n%s", output)
	}
	if !strings.Contains(output, "Run: not running") {
		t.Errorf("expected 'Run: not running'; got:\n%s", output)
	}
	if !strings.Contains(output, "Pipeline:") {
		t.Errorf("expected Pipeline section; got:\n%s", output)
	}
	if !strings.Contains(output, "Health:") {
		t.Errorf("expected Health section; got:\n%s", output)
	}
}

// TestPrintStatus_IncludesModelPerformanceSection verifies that PrintStatus
// reads model stats from iteration logs and includes a "Model Performance:"
// section in the output. The current implementation only outputs Run, Pipeline,
// Health, and SPC sections — this test drives adding logger.ReadModelStats
// integration so the output includes per-model success rates and costs.
func TestPrintStatus_IncludesModelPerformanceSection(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write a status.json so PrintStatus takes the active-run path.
	sw, err := NewStatusWriter(gromitDir)
	if err != nil {
		t.Fatalf("NewStatusWriter: %v", err)
	}
	if err := sw.Write(3, "bead-model", "Model perf test", "sonnet", true, 0, 0); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Create a logs directory with a JSONL file containing iteration entries.
	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll logs: %v", err)
	}
	logLine := `{"timestamp":"2026-02-22T10:00:00Z","iteration":1,"bead_id":"bead-1","model":"sonnet","success":true,"duration_ms":5000,"cost_usd":0.05,"input_tokens":1000,"output_tokens":500}` + "\n" +
		`{"timestamp":"2026-02-22T10:01:00Z","iteration":2,"bead_id":"bead-2","model":"haiku","success":false,"duration_ms":3000,"cost_usd":0.01,"input_tokens":800,"output_tokens":300}` + "\n"
	if err := os.WriteFile(filepath.Join(logsDir, "run-2026-02-22.jsonl"), []byte(logLine), 0644); err != nil {
		t.Fatalf("WriteFile log: %v", err)
	}

	cfg := &config.Config{}
	cfg.Paths.Specs = filepath.Join(tmpDir, "specs")
	cfg.Paths.Plans = filepath.Join(tmpDir, "plans")
	cfg.Paths.Logs = logsDir

	var buf strings.Builder
	if err := PrintStatus(gromitDir, cfg, &buf, func(int) bool { return true }); err != nil {
		t.Fatalf("PrintStatus: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Model Performance:") {
		t.Errorf("PrintStatus output missing Model Performance section; got:\n%s", output)
	}
}

// TestPrintStatus_IncludesNextActionSection verifies that PrintStatus includes
// the "Next action:" recommendation section derived from pipeline.ReadStatus().
// The current implementation reads the pipeline status but never calls
// formatRecommendation — this test drives adding that call so the output
// includes the recommendation with a command hint.
func TestPrintStatus_IncludesNextActionSection(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// No status.json — idle state. Pipeline should recommend based on backlog.
	// Write a backlog item so the pipeline produces a non-empty recommendation.
	if err := os.WriteFile(
		filepath.Join(gromitDir, "backlog.jsonl"),
		[]byte(`{"id":"idea-1","text":"Add rate limiting"}`+"\n"),
		0644,
	); err != nil {
		t.Fatalf("WriteFile backlog: %v", err)
	}

	cfg := &config.Config{}
	cfg.Paths.Specs = filepath.Join(tmpDir, "specs")
	cfg.Paths.Plans = filepath.Join(tmpDir, "plans")

	var buf strings.Builder
	if err := PrintStatus(gromitDir, cfg, &buf, nil); err != nil {
		t.Fatalf("PrintStatus: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Next action:") {
		t.Errorf("PrintStatus output missing Next action section; got:\n%s", output)
	}
}
