//go:build acceptance

package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestWriteIterationLog_PropagatesATDDReviewFields verifies that when
// IterationResult has ATDDReviewVerdict and ATDDReviewRewrite fields,
// they propagate to IterationLog.
func TestWriteIterationLog_PropagatesATDDReviewFields(t *testing.T) {
	// Expected failure: ATDDReviewVerdict field does not exist on IterationResult yet
	// Expected failure: ATDDReviewRewrite field does not exist on IterationResult yet
	tmpDir := t.TempDir()
	l, err := logger.NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Create runner with logger
	r := &Runner{
		logger: l,
	}

	// Create result with ATDD review fields set
	result := &runtypes.IterationResult{
		BeadID:            "test-atdd-1",
		BeadTitle:         "Test ATDD task",
		Model:             "haiku",
		Success:           true,
		Duration:          2 * time.Second,
		ATDDReviewVerdict: "pass",
		ATDDReviewRewrite: false,
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

	// Expected failure: ATDDReviewVerdict field does not exist on IterationLog yet
	if logEntry.ATDDReviewVerdict != "pass" {
		t.Errorf("expected ATDDReviewVerdict='pass', got %q", logEntry.ATDDReviewVerdict)
	}

	// Expected failure: ATDDReviewRewrite field does not exist on IterationLog yet
	if logEntry.ATDDReviewRewrite {
		t.Error("expected ATDDReviewRewrite=false")
	}
}

// TestWriteIterationLog_ATDDReviewRewriteTrue verifies that when
// ATDDReviewRewrite=true, it propagates to IterationLog.
func TestWriteIterationLog_ATDDReviewRewriteTrue(t *testing.T) {
	// Expected failure: ATDDReviewVerdict field does not exist on IterationResult yet
	// Expected failure: ATDDReviewRewrite field does not exist on IterationResult yet
	tmpDir := t.TempDir()
	l, err := logger.NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	r := &Runner{
		logger: l,
	}

	// Create result with ATDDReviewRewrite=true
	result := &runtypes.IterationResult{
		BeadID:            "test-atdd-2",
		BeadTitle:         "Test ATDD task with rewrite",
		Model:             "haiku",
		Success:           true,
		Duration:          3 * time.Second,
		ATDDReviewVerdict: "fail",
		ATDDReviewRewrite: true,
	}

	r.writeIterationLog(2, result)

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
	lines := strings.Split(string(data), "\n")
	if len(lines) < 1 || lines[0] == "" {
		t.Fatal("expected at least 1 log line")
	}
	if err := json.Unmarshal([]byte(lines[0]), &logEntry); err != nil {
		t.Fatalf("unmarshaling log entry: %v", err)
	}

	// Expected failure: ATDDReviewVerdict field does not exist on IterationLog yet
	if logEntry.ATDDReviewVerdict != "fail" {
		t.Errorf("expected ATDDReviewVerdict='fail', got %q", logEntry.ATDDReviewVerdict)
	}

	// Expected failure: ATDDReviewRewrite field does not exist on IterationLog yet
	if !logEntry.ATDDReviewRewrite {
		t.Error("expected ATDDReviewRewrite=true")
	}
}

// TestWriteIterationLog_ATDDReviewFieldsOmittedWhenEmpty verifies that
// ATDDReviewVerdict is omitted when empty and ATDDReviewRewrite is omitted when false.
func TestWriteIterationLog_ATDDReviewFieldsOmittedWhenEmpty(t *testing.T) {
	// Expected failure: ATDDReviewVerdict field does not exist on IterationResult yet
	// Expected failure: ATDDReviewRewrite field does not exist on IterationResult yet
	tmpDir := t.TempDir()
	l, err := logger.NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	r := &Runner{
		logger: l,
	}

	// Create result without ATDD review fields (defaults)
	result := &runtypes.IterationResult{
		BeadID:            "test-atdd-3",
		BeadTitle:         "Test non-ATDD task",
		Model:             "sonnet",
		Success:           true,
		Duration:          1 * time.Second,
		ATDDReviewVerdict: "",
		ATDDReviewRewrite: false,
	}

	r.writeIterationLog(3, result)

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

	// Parse as map to check field presence
	var rawLog map[string]interface{}
	lines := strings.Split(string(data), "\n")
	if len(lines) < 1 || lines[0] == "" {
		t.Fatal("expected at least 1 log line")
	}
	if err := json.Unmarshal([]byte(lines[0]), &rawLog); err != nil {
		t.Fatalf("unmarshaling log entry: %v", err)
	}

	// Expected failure: ATDDReviewVerdict field does not exist on IterationLog yet
	// Verify that empty string verdict is omitted (omitempty)
	if _, exists := rawLog["atdd_review_verdict"]; exists {
		t.Error("expected atdd_review_verdict to be omitted when empty")
	}

	// Expected failure: ATDDReviewRewrite field does not exist on IterationLog yet
	// Verify that false rewrite is omitted (omitempty)
	if _, exists := rawLog["atdd_review_rewrite"]; exists {
		t.Error("expected atdd_review_rewrite to be omitted when false")
	}
}

// TestIterationResult_ATDDReviewFieldsHaveCorrectTypes verifies that
// ATDDReviewVerdict is a string and ATDDReviewRewrite is a bool.
func TestIterationResult_ATDDReviewFieldsHaveCorrectTypes(t *testing.T) {
	// Expected failure: ATDDReviewVerdict field does not exist on IterationResult yet
	// Expected failure: ATDDReviewRewrite field does not exist on IterationResult yet

	// This test verifies the fields exist and have the correct types
	result := &runtypes.IterationResult{
		BeadID:            "test-types",
		BeadTitle:         "Type test",
		Model:             "haiku",
		Success:           true,
		ATDDReviewVerdict: "pass",
		ATDDReviewRewrite: true,
	}

	// If the fields exist with correct types, these assertions will compile and pass
	var verdict string = result.ATDDReviewVerdict
	if verdict != "pass" {
		t.Errorf("expected ATDDReviewVerdict='pass', got %q", verdict)
	}

	var rewrite bool = result.ATDDReviewRewrite
	if !rewrite {
		t.Error("expected ATDDReviewRewrite=true")
	}
}

// TestIterationLog_ATDDReviewFieldsHaveCorrectTypes verifies that
// ATDDReviewVerdict is a string and ATDDReviewRewrite is a bool in IterationLog.
func TestIterationLog_ATDDReviewFieldsHaveCorrectTypes(t *testing.T) {
	// Expected failure: ATDDReviewVerdict field does not exist on IterationLog yet
	// Expected failure: ATDDReviewRewrite field does not exist on IterationLog yet

	log := &logger.IterationLog{
		Timestamp:         time.Now(),
		Iteration:         1,
		BeadID:            "test-log-types",
		BeadTitle:         "Log type test",
		Model:             "haiku",
		Success:           true,
		ATDDReviewVerdict: "fail",
		ATDDReviewRewrite: true,
	}

	// If the fields exist with correct types, these assertions will compile and pass
	var verdict string = log.ATDDReviewVerdict
	if verdict != "fail" {
		t.Errorf("expected ATDDReviewVerdict='fail', got %q", verdict)
	}

	var rewrite bool = log.ATDDReviewRewrite
	if !rewrite {
		t.Error("expected ATDDReviewRewrite=true")
	}
}
