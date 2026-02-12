package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestIterationLog_UsageLimitedField verifies that IterationLog struct has a
// UsageLimited field for tracking usage limit errors in log output.
func TestIterationLog_UsageLimitedField(t *testing.T) {
	// Expected failure: UsageLimited field does not exist on IterationLog struct yet
	log := &IterationLog{
		Timestamp:    time.Now(),
		Iteration:    1,
		BeadID:       "test-1",
		BeadTitle:    "Test task",
		Model:        "sonnet",
		Success:      false,
		UsageLimited: true,
	}

	if !log.UsageLimited {
		t.Errorf("expected UsageLimited=true, got %v", log.UsageLimited)
	}
}

// TestLogIteration_UsageLimitedTrue verifies that when UsageLimited is true,
// it is serialized to JSON correctly.
func TestLogIteration_UsageLimitedTrue(t *testing.T) {
	// Expected failure: UsageLimited field does not exist on IterationLog struct yet
	tmpDir := t.TempDir()
	l, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	log := &IterationLog{
		Timestamp:    time.Now(),
		Iteration:    1,
		BeadID:       "b1",
		BeadTitle:    "Test task with usage limit",
		Model:        "sonnet",
		Success:      false,
		Validated:    false,
		Escalated:    false,
		DurationMs:   1000,
		UsageLimited: true,
		Error:        "usage limit exceeded",
	}

	if err := l.LogIteration(log); err != nil {
		t.Fatal(err)
	}

	// Read back and verify
	data, err := os.ReadFile(l.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Verify UsageLimited field is present in JSON
	if !contains(content, `"usage_limited":true`) {
		t.Error("log should contain usage_limited field with value true")
	}

	// Verify error field is present
	if !contains(content, `"error":"usage limit exceeded"`) {
		t.Error("log should contain error field with usage limit message")
	}
}

// TestLogIteration_UsageLimitedFalse verifies that when UsageLimited is false,
// it is omitted from JSON output (omitempty).
func TestLogIteration_UsageLimitedFalse(t *testing.T) {
	// Expected failure: UsageLimited field does not exist on IterationLog struct yet
	tmpDir := t.TempDir()
	l, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	log := &IterationLog{
		Timestamp:    time.Now(),
		Iteration:    1,
		BeadID:       "b1",
		BeadTitle:    "Test task without usage limit",
		Model:        "sonnet",
		Success:      true,
		Validated:    true,
		Escalated:    false,
		DurationMs:   1000,
		UsageLimited: false,
	}

	if err := l.LogIteration(log); err != nil {
		t.Fatal(err)
	}

	// Read back and verify
	data, err := os.ReadFile(l.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Verify UsageLimited field is omitted when false (omitempty)
	if contains(content, `"usage_limited"`) {
		t.Error("log should not contain usage_limited field when false (omitempty)")
	}
}

// TestReadLogFile_UsageLimitedField verifies that log files with usage_limited
// field can be read back correctly.
func TestReadLogFile_UsageLimitedField(t *testing.T) {
	// Expected failure: UsageLimited field does not exist on IterationLog struct yet
	dir := t.TempDir()

	// Write a log file with usage_limited field
	logContent := `{"timestamp":"2026-02-12T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":1000,"usage_limited":true,"error":"usage limit exceeded"}
{"timestamp":"2026-02-12T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":2000}
{"timestamp":"2026-02-12T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"opus","success":false,"validated":false,"escalated":false,"duration_ms":3000,"usage_limited":true,"error":"rate limit exceeded"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260212-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Read back the entries
	entries, err := readLogFile(filepath.Join(dir, "run-20260212-120000.jsonl"))
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// First entry: usage limit detected
	if !entries[0].UsageLimited {
		t.Error("expected first entry to have UsageLimited=true")
	}
	if entries[0].Error != "usage limit exceeded" {
		t.Errorf("expected error 'usage limit exceeded', got %q", entries[0].Error)
	}

	// Second entry: successful, no usage limit
	if entries[1].UsageLimited {
		t.Error("expected second entry to have UsageLimited=false")
	}

	// Third entry: usage limit detected
	if !entries[2].UsageLimited {
		t.Error("expected third entry to have UsageLimited=true")
	}
	if entries[2].Error != "rate limit exceeded" {
		t.Errorf("expected error 'rate limit exceeded', got %q", entries[2].Error)
	}
}

// TestReadLogFile_BackwardCompatibility verifies that log files without
// usage_limited field can still be read (backward compatibility).
func TestReadLogFile_BackwardCompatibility(t *testing.T) {
	// Expected failure: UsageLimited field does not exist on IterationLog struct yet
	dir := t.TempDir()

	// Write a log file without usage_limited field (old format)
	logContent := `{"timestamp":"2026-02-12T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-12T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"error":"build failed"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260212-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Read back the entries
	entries, err := readLogFile(filepath.Join(dir, "run-20260212-120000.jsonl"))
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Both entries should have UsageLimited=false (zero value)
	if entries[0].UsageLimited {
		t.Error("expected first entry to have UsageLimited=false (zero value for backward compatibility)")
	}
	if entries[1].UsageLimited {
		t.Error("expected second entry to have UsageLimited=false (zero value for backward compatibility)")
	}
}

// TestWriteIterationLog_PropagatesUsageLimited verifies that when IterationResult
// has UsageLimited=true, it propagates to the IterationLog.
func TestWriteIterationLog_PropagatesUsageLimited(t *testing.T) {
	// Expected failure: writeIterationLog in runner package does not propagate UsageLimited field yet
	// This test documents the expected behavior but cannot be fully tested here since
	// writeIterationLog is in the runner package. The integration test would verify
	// that IterationResult.UsageLimited flows through to IterationLog.UsageLimited
	t.Skip("Cross-package integration test - documents expected behavior")
}

// TestReadPerBeadStats_UsageLimitedCounted verifies that beads with usage limit
// errors are counted correctly in per-bead statistics.
func TestReadPerBeadStats_UsageLimitedCounted(t *testing.T) {
	// Expected failure: UsageLimited field does not exist on IterationLog struct yet
	dir := t.TempDir()

	// Write a log file with usage limit failures
	logContent := `{"timestamp":"2026-02-12T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":1000,"usage_limited":true,"error":"usage limit exceeded"}
{"timestamp":"2026-02-12T12:01:00Z","iteration":2,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":2000}
{"timestamp":"2026-02-12T12:02:00Z","iteration":3,"bead_id":"b2","bead_title":"Task 2","model":"opus","success":false,"validated":false,"escalated":false,"duration_ms":3000,"usage_limited":true,"error":"rate limit exceeded"}
{"timestamp":"2026-02-12T12:03:00Z","iteration":4,"bead_id":"b2","bead_title":"Task 2","model":"opus","success":false,"validated":false,"escalated":false,"duration_ms":4000,"error":"build failed"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260212-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	// b1: 2 attempts (1 usage limit failure, 1 success)
	b1 := stats["b1"]
	if b1.TotalRuns != 2 {
		t.Errorf("b1: expected 2 total runs, got %d", b1.TotalRuns)
	}
	if b1.Failures != 1 {
		t.Errorf("b1: expected 1 failure, got %d", b1.Failures)
	}
	if b1.Successes != 1 {
		t.Errorf("b1: expected 1 success, got %d", b1.Successes)
	}

	// b2: 2 attempts (1 usage limit failure, 1 normal failure)
	b2 := stats["b2"]
	if b2.TotalRuns != 2 {
		t.Errorf("b2: expected 2 total runs, got %d", b2.TotalRuns)
	}
	if b2.Failures != 2 {
		t.Errorf("b2: expected 2 failures, got %d", b2.Failures)
	}
	if b2.Successes != 0 {
		t.Errorf("b2: expected 0 successes, got %d", b2.Successes)
	}
}

// TestReadAllLogs_UsageLimitedCounted verifies that iterations with usage limit
// errors are counted in overall failure statistics.
func TestReadAllLogs_UsageLimitedCounted(t *testing.T) {
	// Expected failure: UsageLimited field does not exist on IterationLog struct yet
	dir := t.TempDir()

	// Write a log file with mixed failures including usage limits
	logContent := `{"timestamp":"2026-02-12T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-12T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"usage_limited":true,"error":"usage limit exceeded"}
{"timestamp":"2026-02-12T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"opus","success":false,"validated":false,"escalated":false,"duration_ms":3000,"error":"build failed"}
{"timestamp":"2026-02-12T12:03:00Z","iteration":4,"bead_id":"b4","bead_title":"Task 4","model":"haiku","success":true,"validated":true,"escalated":false,"duration_ms":500}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260212-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadAllLogs(dir)
	if err != nil {
		t.Fatalf("reading all logs: %v", err)
	}

	// 4 total iterations: 2 succeeded, 2 failed (1 usage limit, 1 normal)
	if stats.Total != 4 {
		t.Errorf("expected 4 total, got %d", stats.Total)
	}
	if stats.Succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", stats.Succeeded)
	}
	if stats.Failed != 2 {
		t.Errorf("expected 2 failed (including usage limit), got %d", stats.Failed)
	}

	expectedFailureRate := 2.0 / 4.0
	if diff := stats.FailureRate() - expectedFailureRate; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected failure rate ~%.3f, got %.3f", expectedFailureRate, stats.FailureRate())
	}
}

// TestIterationLog_JSONTagForUsageLimited verifies that UsageLimited field
// uses snake_case JSON tag (usage_limited) with omitempty.
func TestIterationLog_JSONTagForUsageLimited(t *testing.T) {
	// Expected failure: UsageLimited field does not exist on IterationLog struct yet
	// This test verifies the JSON tag by checking serialization behavior
	tmpDir := t.TempDir()
	l, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Log with UsageLimited=true
	log1 := &IterationLog{
		Timestamp:    time.Now(),
		Iteration:    1,
		BeadID:       "b1",
		BeadTitle:    "Task 1",
		Model:        "sonnet",
		Success:      false,
		UsageLimited: true,
		DurationMs:   1000,
	}
	if err := l.LogIteration(log1); err != nil {
		t.Fatal(err)
	}

	// Log with UsageLimited=false (should be omitted)
	log2 := &IterationLog{
		Timestamp:    time.Now(),
		Iteration:    2,
		BeadID:       "b2",
		BeadTitle:    "Task 2",
		Model:        "sonnet",
		Success:      true,
		UsageLimited: false,
		DurationMs:   1000,
	}
	if err := l.LogIteration(log2); err != nil {
		t.Fatal(err)
	}

	// Read back and verify JSON format
	data, err := os.ReadFile(l.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Should use snake_case: usage_limited, not UsageLimited or usageLimited
	if contains(content, `"UsageLimited"`) {
		t.Error("JSON should use snake_case (usage_limited), not PascalCase (UsageLimited)")
	}
	if contains(content, `"usageLimited"`) {
		t.Error("JSON should use snake_case (usage_limited), not camelCase (usageLimited)")
	}

	// First log should have usage_limited:true
	lines := splitLines(content)
	if len(lines) < 2 {
		t.Fatal("expected at least 2 log lines")
	}
	if !contains(lines[0], `"usage_limited":true`) {
		t.Error("first log line should contain usage_limited:true")
	}

	// Second log should NOT have usage_limited field (omitempty)
	if contains(lines[1], `"usage_limited"`) {
		t.Error("second log line should not contain usage_limited field when false (omitempty)")
	}
}

// Helper function to split content by newlines
func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, ch := range s {
		if ch == '\n' {
			if current != "" {
				lines = append(lines, current)
			}
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
