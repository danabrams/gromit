package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/runner/andon"
)

type errIterationLogger struct {
	err error
}

func (e *errIterationLogger) LogIteration(log *logger.IterationLog) error {
	return e.err
}

func (e *errIterationLogger) LogReview(log *logger.ReviewLog) error {
	return nil
}

func (e *errIterationLogger) Close() error {
	return nil
}

func (e *errIterationLogger) FilePath() string {
	return ""
}

func (e *errIterationLogger) RunID() string {
	return ""
}

func TestLogIterationWithWarning_LogsError(t *testing.T) {
	buf := &bytes.Buffer{}
	r := &Runner{
		logger: &errIterationLogger{err: errors.New("write failed")},
		output: buf,
	}

	r.logIterationWithWarning(&logger.IterationLog{
		BeadID: "test-1",
	})

	if !strings.Contains(buf.String(), "Warning: failed to write iteration log: write failed") {
		t.Fatalf("expected warning in output, got %q", buf.String())
	}
}

// TestWriteIterationLog_PropagatesUsageLimited verifies that when
// IterationResult has UsageLimited=true, it propagates to IterationLog.
func TestWriteIterationLog_PropagatesUsageLimited(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := logger.NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			t.Fatalf("failed to close logger: %v", err)
		}
	}()

	// Create runner with logger
	r := &Runner{
		logger: l,
	}

	// Create result with UsageLimited=true
	result := &IterationResult{
		BeadID:       "test-1",
		BeadTitle:    "Test task",
		Model:        "sonnet",
		Success:      false,
		UsageLimited: true,
		Duration:     1 * time.Second,
		Error:        fmt.Errorf("usage limit exceeded"),
	}

	// Write the log
	r.writeIterationLog(1, result)

	// Read back the log file to verify
	logFiles, err := filepath.Glob(filepath.Join(tmpDir, "run-*.jsonl"))
	if err != nil {
		t.Fatalf("globbing log files: %v", err)
	}
	if len(logFiles) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(logFiles))
	}

	data, err := os.ReadFile(logFiles[0])
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	// Parse the JSON line
	var logEntry logger.IterationLog
	lines := strings.Split(string(data), "\n")
	if len(lines) < 1 || lines[0] == "" {
		t.Fatal("expected at least 1 log line")
	}
	if err := json.Unmarshal([]byte(lines[0]), &logEntry); err != nil {
		t.Fatalf("unmarshaling log entry: %v", err)
	}

	if !logEntry.UsageLimited {
		t.Error("expected UsageLimited to be propagated to log entry")
	}
	if logEntry.Error != "usage limit exceeded" {
		t.Errorf("expected error 'usage limit exceeded', got %q", logEntry.Error)
	}
	if logEntry.Success {
		t.Error("expected Success=false")
	}
}

// TestWriteIterationLog_UsageLimitedFalseNotPropagated verifies that when
// UsageLimited=false, it's omitted from the log (omitempty).
func TestWriteIterationLog_UsageLimitedFalseNotPropagated(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := logger.NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			t.Fatalf("failed to close logger: %v", err)
		}
	}()

	// Create runner with logger
	r := &Runner{
		logger: l,
	}

	// Create result with UsageLimited=false (default)
	result := &IterationResult{
		BeadID:       "test-2",
		BeadTitle:    "Test task",
		Model:        "sonnet",
		Success:      true,
		UsageLimited: false,
		Duration:     1 * time.Second,
	}

	// Write the log
	r.writeIterationLog(2, result)

	// Read back the log file to verify
	logFiles, err := filepath.Glob(filepath.Join(tmpDir, "run-*.jsonl"))
	if err != nil {
		t.Fatalf("globbing log files: %v", err)
	}
	if len(logFiles) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(logFiles))
	}

	data, err := os.ReadFile(logFiles[0])
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	// Parse the JSON line
	var logEntry logger.IterationLog
	lines := strings.Split(string(data), "\n")
	if len(lines) < 1 || lines[0] == "" {
		t.Fatal("expected at least 1 log line")
	}
	if err := json.Unmarshal([]byte(lines[0]), &logEntry); err != nil {
		t.Fatalf("unmarshaling log entry: %v", err)
	}

	if logEntry.UsageLimited {
		t.Error("expected UsageLimited to be false (default)")
	}
	if logEntry.Success != true {
		t.Error("expected Success=true")
	}
}

func TestWriteIterationLog_WritesAcceptanceFailureArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := logger.NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			t.Fatalf("failed to close logger: %v", err)
		}
	}()

	r := &Runner{
		logger: l,
		cfg: &config.Config{
			Paths: config.PathsConfig{Logs: tmpDir},
		},
	}

	result := &IterationResult{
		BeadID:                   "bead-accept-1",
		BeadTitle:                "ATDD verify",
		Model:                    "sonnet",
		Success:                  false,
		Duration:                 2 * time.Second,
		Error:                    fmt.Errorf("post-build acceptance verification failed"),
		AcceptanceFailureSummary: "acceptance tests failed after implementation",
		AcceptanceFailureOutput:  "VALIDATION_FAILED\n--- FAIL: TestSomething",
	}

	r.writeIterationLog(3, result)

	if result.AcceptanceFailureArtifact == "" {
		t.Fatal("expected acceptance failure artifact path to be set")
	}
	if _, err := os.Stat(result.AcceptanceFailureArtifact); err != nil {
		t.Fatalf("expected artifact file to exist: %v", err)
	}

	logFiles, err := filepath.Glob(filepath.Join(tmpDir, "run-*.jsonl"))
	if err != nil {
		t.Fatalf("globbing log files: %v", err)
	}
	if len(logFiles) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(logFiles))
	}

	data, err := os.ReadFile(logFiles[0])
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	var logEntry logger.IterationLog
	line := strings.Split(strings.TrimSpace(string(data)), "\n")[0]
	if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
		t.Fatalf("unmarshaling log entry: %v", err)
	}
	if logEntry.AcceptanceFailureSummary == "" {
		t.Fatal("expected acceptance_failure_summary in JSONL")
	}
	if logEntry.AcceptanceFailureOutput == "" {
		t.Fatal("expected acceptance_failure_output in JSONL")
	}
	if logEntry.AcceptanceFailureArtifact == "" {
		t.Fatal("expected acceptance_failure_artifact in JSONL")
	}
}

func TestWriteIterationLog_EmitsAndonClassificationAndReliabilitySignals(t *testing.T) {
	// Expected failure: IterationResult/IterationLog do not yet expose FailureClass,
	// AndonLevel, TrimDecision, AutonomyEligible, AutonomySuccess, FirstPassSuccess,
	// MTTRProxyMs, EscalationClass, or RecurrenceCount fields.
	tmpDir := t.TempDir()
	l, err := logger.NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			t.Fatalf("failed to close logger: %v", err)
		}
	}()

	r := &Runner{logger: l}
	result := &IterationResult{
		BeadID:           "gromit-o27x",
		BeadTitle:        "Add reliability metrics and structured Andon logging",
		Model:            "sonnet",
		Success:          false,
		Duration:         2 * time.Second,
		Error:            fmt.Errorf("quality gate failed"),
		FailureClass:     andon.FailureClassQuality,
		AndonLevel:       andon.LevelL2,
		TrimDecision:     "middle_ellipsis",
		AutonomyEligible: true,
		AutonomySuccess:  false,
		FirstPassSuccess: false,
		MTTRProxyMs:      42000,
		EscalationClass:  andon.FailureClassQuality,
		RecurrenceCount:  3,
	}

	r.writeIterationLog(7, result)

	logFiles, err := filepath.Glob(filepath.Join(tmpDir, "run-*.jsonl"))
	if err != nil {
		t.Fatalf("globbing log files: %v", err)
	}
	if len(logFiles) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(logFiles))
	}

	data, err := os.ReadFile(logFiles[0])
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	var entry map[string]any
	line := strings.Split(strings.TrimSpace(string(data)), "\n")[0]
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("unmarshaling log line: %v", err)
	}

	if got := entry["failure_class"]; got != string(andon.FailureClassQuality) {
		t.Fatalf("failure_class = %v, want %q", got, andon.FailureClassQuality)
	}
	if got := entry["andon_level"]; got != string(andon.LevelL2) {
		t.Fatalf("andon_level = %v, want %q", got, andon.LevelL2)
	}
	if got := entry["trim_decision"]; got != "middle_ellipsis" {
		t.Fatalf("trim_decision = %v, want %q", got, "middle_ellipsis")
	}
	if got, ok := entry["autonomy_eligible"].(bool); !ok || !got {
		t.Fatalf("autonomy_eligible = %v, want true", entry["autonomy_eligible"])
	}
	if got, ok := entry["autonomy_success"].(bool); !ok || got {
		t.Fatalf("autonomy_success = %v, want false", entry["autonomy_success"])
	}
	if got, ok := entry["first_pass_success"].(bool); !ok || got {
		t.Fatalf("first_pass_success = %v, want false", entry["first_pass_success"])
	}
	if got := entry["mttr_proxy_ms"]; got != float64(42000) {
		t.Fatalf("mttr_proxy_ms = %v, want 42000", got)
	}
	if got := entry["escalation_class"]; got != string(andon.FailureClassQuality) {
		t.Fatalf("escalation_class = %v, want %q", got, andon.FailureClassQuality)
	}
	if got := entry["recurrence_count"]; got != float64(3) {
		t.Fatalf("recurrence_count = %v, want 3", got)
	}
}
