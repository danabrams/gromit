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
	"github.com/danabrams/gromit/internal/prompt"
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

func newTestRunnerWithLogger(t *testing.T, includeLogsPathConfig bool) (*Runner, string) {
	t.Helper()

	tmpDir := t.TempDir()
	l, err := logger.NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := l.Close(); err != nil {
			t.Fatalf("failed to close logger: %v", err)
		}
	})

	r := &Runner{logger: l}
	if includeLogsPathConfig {
		r.cfg = &config.Config{
			Paths: config.PathsConfig{Logs: tmpDir},
		}
	}

	return r, tmpDir
}

func readSingleIterationLogLine(t *testing.T, logsDir string) string {
	t.Helper()

	logFiles, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
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

	line := strings.Split(strings.TrimSpace(string(data)), "\n")[0]
	if line == "" {
		t.Fatal("expected at least 1 log line")
	}

	return line
}

func unmarshalSingleIterationLogEntry(t *testing.T, logsDir string) map[string]any {
	t.Helper()

	var entry map[string]any
	if err := json.Unmarshal([]byte(readSingleIterationLogLine(t, logsDir)), &entry); err != nil {
		t.Fatalf("unmarshaling log line: %v", err)
	}

	return entry
}

func unmarshalSingleIterationLogInto(t *testing.T, logsDir string, target any) {
	t.Helper()

	if err := json.Unmarshal([]byte(readSingleIterationLogLine(t, logsDir)), target); err != nil {
		t.Fatalf("unmarshaling log entry: %v", err)
	}
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

// TestWriteIterationLog_UsageLimitedFalseNotPropagated verifies that when
// UsageLimited=false, it's omitted from the log (omitempty).
func TestWriteIterationLog_UsageLimitedFalseNotPropagated(t *testing.T) {
	r, tmpDir := newTestRunnerWithLogger(t, false)

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

	var logEntry logger.IterationLog
	unmarshalSingleIterationLogInto(t, tmpDir, &logEntry)

	if logEntry.UsageLimited {
		t.Error("expected UsageLimited to be false (default)")
	}
	if !logEntry.Success {
		t.Error("expected Success=true")
	}
}

// TestWriteIterationLog_PropagatesValidationDurationMs verifies that when
// IterationResult has ValidationDurationMs set, it propagates to IterationLog.
func TestWriteIterationLog_PropagatesValidationDurationMs(t *testing.T) {
	r, tmpDir := newTestRunnerWithLogger(t, false)

	result := &IterationResult{
		BeadID:               "test-2",
		BeadTitle:            "Validation timing",
		Model:                "sonnet",
		Success:              true,
		ValidationDurationMs: 1450,
		ValidationTimeouts:   2,
		Duration:             1 * time.Second,
	}

	r.writeIterationLog(2, result)

	var logEntry logger.IterationLog
	unmarshalSingleIterationLogInto(t, tmpDir, &logEntry)

	if logEntry.ValidationDurationMs != 1450 {
		t.Errorf("expected ValidationDurationMs=1450, got %d", logEntry.ValidationDurationMs)
	}
	if logEntry.ValidationTimeouts != 2 {
		t.Errorf("expected ValidationTimeouts=2, got %d", logEntry.ValidationTimeouts)
	}
}

func TestWriteIterationLog_PropagatesFallbackCounters(t *testing.T) {
	r, tmpDir := newTestRunnerWithLogger(t, false)

	result := &IterationResult{
		BeadID:            "test-fallback-counters",
		BeadTitle:         "Fallback counters",
		Model:             "sonnet",
		Success:           false,
		FallbackAttempts:  3,
		FallbackSuccesses: 1,
		FallbackFailures:  2,
		Duration:          1 * time.Second,
	}

	r.writeIterationLog(2, result)

	var logEntry logger.IterationLog
	unmarshalSingleIterationLogInto(t, tmpDir, &logEntry)

	if logEntry.FallbackAttempts != 3 {
		t.Errorf("expected FallbackAttempts=3, got %d", logEntry.FallbackAttempts)
	}
	if logEntry.FallbackSuccesses != 1 {
		t.Errorf("expected FallbackSuccesses=1, got %d", logEntry.FallbackSuccesses)
	}
	if logEntry.FallbackFailures != 2 {
		t.Errorf("expected FallbackFailures=2, got %d", logEntry.FallbackFailures)
	}
}

func TestWriteIterationLog_PropagatesPromptDiagnostics(t *testing.T) {
	r, tmpDir := newTestRunnerWithLogger(t, false)

	result := &IterationResult{
		BeadID:    "test-prompt-diag",
		BeadTitle: "Prompt diagnostics propagation",
		Model:     "sonnet",
		Success:   true,
		PromptDiagnostics: &prompt.PromptDiagnostics{
			PromptType:      "build",
			EstimatedTokens: 777,
			SectionTokens: map[string]int{
				prompt.SectionRules: 101,
			},
		},
		Duration: 1 * time.Second,
	}

	r.writeIterationLog(5, result)

	var logEntry logger.IterationLog
	unmarshalSingleIterationLogInto(t, tmpDir, &logEntry)

	if logEntry.PromptDiagnostics == nil {
		t.Fatal("expected PromptDiagnostics to be propagated to iteration log")
	}
	if logEntry.PromptDiagnostics.PromptType != "build" {
		t.Errorf("PromptType = %q, want %q", logEntry.PromptDiagnostics.PromptType, "build")
	}
	if logEntry.PromptDiagnostics.EstimatedTokens != 777 {
		t.Errorf("EstimatedTokens = %d, want 777", logEntry.PromptDiagnostics.EstimatedTokens)
	}
	if got := logEntry.PromptDiagnostics.SectionTokens[prompt.SectionRules]; got != 101 {
		t.Errorf("SectionTokens[%q] = %d, want 101", prompt.SectionRules, got)
	}
}

func TestWriteIterationLog_PropagatesProviderAndFailureClassificationFields(t *testing.T) {
	r, tmpDir := newTestRunnerWithLogger(t, false)
	result := &IterationResult{
		BeadID:          "test-provider-category",
		BeadTitle:       "Provider category propagation",
		Model:           "high",
		ReasoningEffort: "low",
		Provider:        "codex",
		FailureCategory: "transport_disconnect",
		FailureLayer:    "execution",
		FailureSubCat:   "provider_transport",
		Success:         false,
		Duration:        1 * time.Second,
		Error:           fmt.Errorf("invoke failed"),
	}

	r.writeIterationLog(3, result)

	entry := unmarshalSingleIterationLogEntry(t, tmpDir)

	if got := entry["provider"]; got != "codex" {
		t.Fatalf("provider = %v, want %q", got, "codex")
	}
	if got := entry["reasoning_effort"]; got != "low" {
		t.Fatalf("reasoning_effort = %v, want %q", got, "low")
	}
	if got := entry["failure_category"]; got != "transport_disconnect" {
		t.Fatalf("failure_category = %v, want %q", got, "transport_disconnect")
	}
	if got := entry["failure_layer"]; got != "execution" {
		t.Fatalf("failure_layer = %v, want %q", got, "execution")
	}
	if got := entry["failure_sub_cat"]; got != "provider_transport" {
		t.Fatalf("failure_sub_cat = %v, want %q", got, "provider_transport")
	}
}

func TestWriteIterationLog_WritesAcceptanceFailureArtifact(t *testing.T) {
	r, tmpDir := newTestRunnerWithLogger(t, true)

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

	var logEntry logger.IterationLog
	unmarshalSingleIterationLogInto(t, tmpDir, &logEntry)
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

func TestWriteIterationLog_RecordsAcceptanceFailureExitCode(t *testing.T) {
	r, tmpDir := newTestRunnerWithLogger(t, true)

	result := &IterationResult{
		BeadID:                    "bead-accept-exitcode",
		BeadTitle:                 "ATDD verify",
		Model:                     "sonnet",
		Success:                   false,
		Duration:                  2 * time.Second,
		Error:                     fmt.Errorf("post-build acceptance verification failed"),
		AcceptanceFailureSummary:  "acceptance tests failed after implementation",
		AcceptanceFailureOutput:   "VALIDATION_FAILED\n--- FAIL: TestSomething",
		AcceptanceFailureExitCode: 2,
	}

	r.writeIterationLog(4, result)

	entry := unmarshalSingleIterationLogEntry(t, tmpDir)

	if got := entry["acceptance_failure_exit_code"]; got != float64(2) {
		t.Fatalf("acceptance_failure_exit_code = %v, want 2", got)
	}
	if got := entry["acceptance_failure_artifact"]; got == "" || got == nil {
		t.Fatal("expected acceptance_failure_artifact to be set")
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
		Provider:         "codex",
		FailureCategory:  "rate_limited",
		Success:          false,
		Duration:         2 * time.Second,
		Error:            fmt.Errorf("quality gate failed"),
		FailureClass:     string(andon.FailureClassQuality),
		AndonLevel:       string(andon.LevelL2),
		TrimDecision:     "middle_ellipsis",
		AutonomyEligible: true,
		AutonomySuccess:  false,
		FirstPassSuccess: false,
		MTTRProxyMs:      42000,
		EscalationClass:  string(andon.FailureClassQuality),
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
	if got := entry["provider"]; got != "codex" {
		t.Fatalf("provider = %v, want %q", got, "codex")
	}
	if got := entry["failure_category"]; got != "rate_limited" {
		t.Fatalf("failure_category = %v, want %q", got, "rate_limited")
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

func TestWriteIterationLog_FailuresAlwaysEmitStructuredAndonEnvelope(t *testing.T) {
	// Expected failure: writeIterationLog does not call ensureFailureAndonEnvelope,
	// and the helper does not exist yet.
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
		BeadID:       "gromit-o27x",
		BeadTitle:    "Add reliability metrics and structured Andon logging",
		Model:        "sonnet",
		Success:      false,
		Duration:     2 * time.Second,
		Error:        fmt.Errorf("quality gate failed"),
		FailureClass: string(andon.FailureClassQuality),
		AndonLevel:   string(andon.LevelL2),
	}

	ensureFailureAndonEnvelope(result) // compile-time acceptance guard
	r.writeIterationLog(9, result)

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
}

func TestWriteIterationLog_FailureWithoutClassificationGetsDefaultAndonEnvelope(t *testing.T) {
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
		BeadID:    "gromit-o27x-defaults",
		BeadTitle: "Unclassified failure",
		Model:     "haiku",
		Success:   false,
		Duration:  1 * time.Second,
		Error:     fmt.Errorf("unclassified failure"),
	}

	r.writeIterationLog(10, result)

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

	if got := entry["failure_class"]; got != string(andon.FailureClassWorkflow) {
		t.Fatalf("failure_class = %v, want %q", got, andon.FailureClassWorkflow)
	}
	if got := entry["andon_level"]; got != string(andon.LevelL1) {
		t.Fatalf("andon_level = %v, want %q", got, andon.LevelL1)
	}
}

func TestWriteIterationLog_FilesTouched(t *testing.T) {
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
		BeadID:       "ft-test",
		Model:        "sonnet",
		Success:      true,
		Duration:     time.Second,
		FilesTouched: 5,
	}

	r.writeIterationLog(1, result)

	// Read back the log and verify FilesTouched
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

	if got := entry["files_touched"]; got != float64(5) {
		t.Fatalf("files_touched = %v, want 5", got)
	}
}

func TestWriteIterationLog_TouchedPackages(t *testing.T) {
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
		BeadID:          "tp-test",
		Model:           "sonnet",
		Success:         true,
		Duration:        time.Second,
		TouchedPackages: []string{"internal/runner", "internal/logger"},
	}

	r.writeIterationLog(1, result)

	var logEntry logger.IterationLog
	unmarshalSingleIterationLogInto(t, tmpDir, &logEntry)

	if len(logEntry.TouchedPackages) != 2 {
		t.Fatalf("TouchedPackages length = %d, want 2", len(logEntry.TouchedPackages))
	}
	if logEntry.TouchedPackages[0] != "internal/runner" {
		t.Fatalf("TouchedPackages[0] = %q, want %q", logEntry.TouchedPackages[0], "internal/runner")
	}
	if logEntry.TouchedPackages[1] != "internal/logger" {
		t.Fatalf("TouchedPackages[1] = %q, want %q", logEntry.TouchedPackages[1], "internal/logger")
	}
}

func TestWriteIterationLog_PropagatesTimeoutPhase(t *testing.T) {
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
		BeadID:       "bead-phase-1",
		BeadTitle:    "Phase attribution test",
		Model:        "sonnet",
		Success:      false,
		Duration:     1 * time.Second,
		Error:        fmt.Errorf("validation phase aborted due to timeout"),
		TimeoutType:  "bead",
		TimeoutPhase: "validation",
	}

	r.writeIterationLog(1, result)

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

	if got := entry["timeout_phase"]; got != "validation" {
		t.Fatalf("timeout_phase = %v, want %q", got, "validation")
	}
	if got := entry["timeout_type"]; got != "bead" {
		t.Fatalf("timeout_type = %v, want %q", got, "bead")
	}
}

func TestWriteIterationLog_ReliabilityMetricsDerivableFromStructuredEntries(t *testing.T) {
	// Expected failure: logger.ReadReliabilityMetrics does not exist yet; the
	// feature is expected to provide canonical derivation from emitted logs.
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
	r.writeIterationLog(1, &IterationResult{
		BeadID:           "gromit-o27x-a",
		BeadTitle:        "Attempt A",
		Model:            "haiku",
		Success:          false,
		Duration:         1 * time.Second,
		Error:            fmt.Errorf("failed"),
		AutonomyEligible: true,
		AutonomySuccess:  false,
		FirstPassSuccess: false,
		MTTRProxyMs:      30000,
		EscalationClass:  string(andon.FailureClassQuality),
		RecurrenceCount:  2,
	})
	r.writeIterationLog(2, &IterationResult{
		BeadID:           "gromit-o27x-b",
		BeadTitle:        "Attempt B",
		Model:            "sonnet",
		Success:          true,
		Validated:        true,
		Duration:         1 * time.Second,
		AutonomyEligible: true,
		AutonomySuccess:  true,
		FirstPassSuccess: false,
		MTTRProxyMs:      45000,
		EscalationClass:  string(andon.FailureClassQuality),
		RecurrenceCount:  2,
	})

	metrics, err := logger.ReadReliabilityMetrics(tmpDir) // compile-time acceptance guard
	if err != nil {
		t.Fatalf("ReadReliabilityMetrics() error = %v", err)
	}
	if metrics.AutonomyRate != 0.5 {
		t.Fatalf("AutonomyRate = %v, want 0.5", metrics.AutonomyRate)
	}
	if metrics.FirstPassSuccessRate != 0 {
		t.Fatalf("FirstPassSuccessRate = %v, want 0", metrics.FirstPassSuccessRate)
	}
	if metrics.MTTRProxyMs != 45000 {
		t.Fatalf("MTTRProxyMs = %d, want 45000", metrics.MTTRProxyMs)
	}
	if got := metrics.EscalationRatesByClass[string(andon.FailureClassQuality)]; got != 1 {
		t.Fatalf("EscalationRatesByClass[quality] = %v, want 1", got)
	}
	if got := metrics.RecurrenceCounters[string(andon.FailureClassQuality)]; got != 2 {
		t.Fatalf("RecurrenceCounters[quality] = %v, want 2", got)
	}
}

// TestWriteIterationLog_PropagatesCoverageFields verifies that coverage result
// fields are propagated from IterationResult to the JSONL log entry.
func TestWriteIterationLog_PropagatesCoverageFields(t *testing.T) {
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
		BeadID:             "bead-cov-wire",
		BeadTitle:          "Coverage wiring test",
		Model:              "sonnet",
		Success:            true,
		Duration:           1 * time.Second,
		CriteriaTotal:      8,
		CriteriaCovered:    6,
		CriteriaUntestable: 2,
		UncoveredCriteria:  []string{"criterion X", "criterion Y"},
	}

	r.writeIterationLog(1, result)

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

	if got := entry["criteria_total"]; got != float64(8) {
		t.Fatalf("criteria_total = %v, want 8", got)
	}
	if got := entry["criteria_covered"]; got != float64(6) {
		t.Fatalf("criteria_covered = %v, want 6", got)
	}
	if got := entry["criteria_untestable"]; got != float64(2) {
		t.Fatalf("criteria_untestable = %v, want 2", got)
	}
	uncovered, ok := entry["uncovered_criteria"].([]any)
	if !ok || len(uncovered) != 2 {
		t.Fatalf("uncovered_criteria = %v, want [criterion X criterion Y]", entry["uncovered_criteria"])
	}
	if uncovered[0] != "criterion X" || uncovered[1] != "criterion Y" {
		t.Fatalf("uncovered_criteria = %v, want [criterion X criterion Y]", uncovered)
	}
}

func TestLogResult_IncludesPhaseAttributionForTimeoutFailure(t *testing.T) {
	var buf bytes.Buffer
	r := &Runner{output: &buf}
	result := &IterationResult{
		BeadID:       "bead-phase-log",
		BeadTitle:    "Phase test",
		Model:        "sonnet",
		Success:      false,
		Duration:     5 * time.Second,
		Error:        fmt.Errorf("validation phase aborted due to timeout: context deadline exceeded"),
		TimeoutPhase: "validation",
	}

	r.logResult(result)
	output := buf.String()
	if !strings.Contains(output, "phase: validation") {
		t.Fatalf("expected log output to contain phase attribution, got:\n%s", output)
	}
}
