package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadAllLogsEmpty(t *testing.T) {
	dir := t.TempDir()
	stats, err := ReadAllLogs(dir)
	if err != nil {
		t.Fatalf("reading empty logs dir: %v", err)
	}
	if stats.Total != 0 {
		t.Errorf("expected 0 total, got %d", stats.Total)
	}
	if stats.FailureRate() != 0 {
		t.Errorf("expected 0 failure rate, got %f", stats.FailureRate())
	}
}

func TestReadAllLogsWithEntries(t *testing.T) {
	dir := t.TempDir()

	// Write a log file with mixed results
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"error":"build failed"}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"opus","success":true,"validated":true,"escalated":true,"escalated_to":"opus","duration_ms":3000}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadAllLogs(dir)
	if err != nil {
		t.Fatalf("reading logs: %v", err)
	}

	if stats.Total != 3 {
		t.Errorf("expected 3 total, got %d", stats.Total)
	}
	if stats.Succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", stats.Succeeded)
	}
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", stats.Failed)
	}

	expectedRate := 1.0 / 3.0
	if diff := stats.FailureRate() - expectedRate; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected failure rate ~%.3f, got %.3f", expectedRate, stats.FailureRate())
	}
}

func TestReadAllLogsMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	log1 := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
`
	log2 := `{"timestamp":"2026-02-05T13:00:00Z","iteration":1,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"error":"failed"}
`

	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(log1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-130000.jsonl"), []byte(log2), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadAllLogs(dir)
	if err != nil {
		t.Fatalf("reading logs: %v", err)
	}

	if stats.Total != 2 {
		t.Errorf("expected 2 total, got %d", stats.Total)
	}
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", stats.Failed)
	}
}

func TestWriteValidationLog(t *testing.T) {
	dir := t.TempDir()
	output := "internal/foo/bar.go:42:15: undefined: SomeFunction\ninternal/foo/bar.go:58:3: too many arguments\n"

	path, err := WriteValidationLog(dir, output)
	if err != nil {
		t.Fatalf("writing validation log: %v", err)
	}

	// Verify file was created with correct prefix
	base := filepath.Base(path)
	if len(base) < len("validation-") || base[:len("validation-")] != "validation-" {
		t.Errorf("expected filename starting with 'validation-', got %s", base)
	}
	if filepath.Ext(path) != ".log" {
		t.Errorf("expected .log extension, got %s", filepath.Ext(path))
	}

	// Verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading validation log: %v", err)
	}
	if string(content) != output {
		t.Errorf("expected output %q, got %q", output, string(content))
	}
}

func TestWriteValidationLogCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "logs")
	output := "some error output"

	path, err := WriteValidationLog(dir, output)
	if err != nil {
		t.Fatalf("writing validation log: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading validation log: %v", err)
	}
	if string(content) != output {
		t.Errorf("expected output %q, got %q", output, string(content))
	}
}

func TestRunStatsFailureRate(t *testing.T) {
	tests := []struct {
		name     string
		stats    RunStats
		expected float64
	}{
		{"empty", RunStats{}, 0},
		{"all success", RunStats{Total: 5, Succeeded: 5, Failed: 0}, 0},
		{"all failed", RunStats{Total: 3, Succeeded: 0, Failed: 3}, 1.0},
		{"mixed", RunStats{Total: 10, Succeeded: 7, Failed: 3}, 0.3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stats.FailureRate()
			if diff := got - tt.expected; diff > 0.001 || diff < -0.001 {
				t.Errorf("expected failure rate %.3f, got %.3f", tt.expected, got)
			}
		})
	}
}

func TestReadPerBeadStatsEmpty(t *testing.T) {
	dir := t.TempDir()
	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading empty logs dir: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected 0 beads, got %d", len(stats))
	}
}

func TestReadPerBeadStatsWithEntries(t *testing.T) {
	dir := t.TempDir()

	// Write a log file with multiple attempts for same and different beads
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"error":"build failed"}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b1","bead_title":"Task 1","model":"opus","success":false,"validated":false,"escalated":true,"escalated_to":"opus","duration_ms":3000,"error":"validation failed"}
{"timestamp":"2026-02-05T12:03:00Z","iteration":4,"bead_id":"b3","bead_title":"Task 3","model":"haiku","success":true,"validated":true,"escalated":false,"duration_ms":500}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	if len(stats) != 3 {
		t.Errorf("expected 3 beads, got %d", len(stats))
	}

	// Check b1 - attempted twice (1 success, 1 failure)
	b1 := stats["b1"]
	if b1.BeadID != "b1" {
		t.Errorf("expected bead ID 'b1', got %q", b1.BeadID)
	}
	if b1.BeadTitle != "Task 1" {
		t.Errorf("expected bead title 'Task 1', got %q", b1.BeadTitle)
	}
	if b1.TotalRuns != 2 {
		t.Errorf("expected b1 total runs 2, got %d", b1.TotalRuns)
	}
	if b1.Successes != 1 {
		t.Errorf("expected b1 successes 1, got %d", b1.Successes)
	}
	if b1.Failures != 1 {
		t.Errorf("expected b1 failures 1, got %d", b1.Failures)
	}
	if b1.FailureRate() != 0.5 {
		t.Errorf("expected b1 failure rate 0.5, got %.3f", b1.FailureRate())
	}

	// Check b2 - attempted once (failed)
	b2 := stats["b2"]
	if b2.TotalRuns != 1 {
		t.Errorf("expected b2 total runs 1, got %d", b2.TotalRuns)
	}
	if b2.Failures != 1 {
		t.Errorf("expected b2 failures 1, got %d", b2.Failures)
	}
	if b2.FailureRate() != 1.0 {
		t.Errorf("expected b2 failure rate 1.0, got %.3f", b2.FailureRate())
	}

	// Check b3 - attempted once (succeeded)
	b3 := stats["b3"]
	if b3.TotalRuns != 1 {
		t.Errorf("expected b3 total runs 1, got %d", b3.TotalRuns)
	}
	if b3.Successes != 1 {
		t.Errorf("expected b3 successes 1, got %d", b3.Successes)
	}
	if b3.FailureRate() != 0.0 {
		t.Errorf("expected b3 failure rate 0.0, got %.3f", b3.FailureRate())
	}
}

func TestReadPerBeadStatsMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	// First run - b1 fails
	log1 := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Fix bug","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":1000,"error":"build failed"}
`
	// Second run - b1 succeeds, b2 fails
	log2 := `{"timestamp":"2026-02-05T13:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Fix bug","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":2000}
{"timestamp":"2026-02-05T13:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Add feature","model":"opus","success":false,"validated":false,"escalated":false,"duration_ms":3000,"error":"tests failed"}
`

	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(log1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-130000.jsonl"), []byte(log2), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("expected 2 beads, got %d", len(stats))
	}

	// b1 should have 2 total runs (1 failure, 1 success)
	b1 := stats["b1"]
	if b1.TotalRuns != 2 {
		t.Errorf("expected b1 total runs 2, got %d", b1.TotalRuns)
	}
	if b1.Failures != 1 {
		t.Errorf("expected b1 failures 1, got %d", b1.Failures)
	}
	if b1.Successes != 1 {
		t.Errorf("expected b1 successes 1, got %d", b1.Successes)
	}

	// b2 should have 1 total run (1 failure)
	b2 := stats["b2"]
	if b2.TotalRuns != 1 {
		t.Errorf("expected b2 total runs 1, got %d", b2.TotalRuns)
	}
	if b2.Failures != 1 {
		t.Errorf("expected b2 failures 1, got %d", b2.Failures)
	}
}

func TestBeadStatsFailureRate(t *testing.T) {
	tests := []struct {
		name     string
		stats    BeadStats
		expected float64
	}{
		{"empty", BeadStats{}, 0},
		{"all success", BeadStats{TotalRuns: 5, Successes: 5, Failures: 0}, 0},
		{"all failed", BeadStats{TotalRuns: 3, Successes: 0, Failures: 3}, 1.0},
		{"mixed", BeadStats{TotalRuns: 10, Successes: 6, Failures: 4}, 0.4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stats.FailureRate()
			if diff := got - tt.expected; diff > 0.001 || diff < -0.001 {
				t.Errorf("expected failure rate %.3f, got %.3f", tt.expected, got)
			}
		})
	}
}

func TestReadPerBeadStatsLastAttemptTime(t *testing.T) {
	dir := t.TempDir()

	// Single file with multiple attempts at different times
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T13:00:00Z","iteration":2,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000}
{"timestamp":"2026-02-05T14:00:00Z","iteration":3,"bead_id":"b1","bead_title":"Task 1","model":"opus","success":true,"validated":true,"escalated":false,"duration_ms":3000}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	b1 := stats["b1"]
	expectedTime, _ := time.Parse(time.RFC3339, "2026-02-05T14:00:00Z")
	if !b1.LastAttempt.Equal(expectedTime) {
		t.Errorf("expected last attempt %v, got %v", expectedTime, b1.LastAttempt)
	}
}

func TestReadPerBeadStatsLastAttemptTimeAcrossFiles(t *testing.T) {
	dir := t.TempDir()

	// First file - b1 at 12:00
	log1 := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
`
	// Second file - b1 at 11:00 (earlier)
	log2 := `{"timestamp":"2026-02-05T11:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000}
`

	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(log1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-110000.jsonl"), []byte(log2), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	b1 := stats["b1"]
	expectedTime, _ := time.Parse(time.RFC3339, "2026-02-05T12:00:00Z")
	if !b1.LastAttempt.Equal(expectedTime) {
		t.Errorf("expected last attempt %v (from 12:00), got %v", expectedTime, b1.LastAttempt)
	}
}

func TestReadPerBeadStatsInvalidJSON(t *testing.T) {
	dir := t.TempDir()

	// Log file with partial invalid JSON (some valid, some invalid)
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
this is not json
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	// Should read the first valid entry, stop at invalid JSON
	if len(stats) == 0 {
		t.Error("expected to parse at least one valid entry before error")
	}
	if _, exists := stats["b1"]; !exists {
		t.Error("expected b1 to be in stats")
	}
}

func TestReadPerBeadStatsEmptyLogFile(t *testing.T) {
	dir := t.TempDir()

	// Create an empty log file
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	if len(stats) != 0 {
		t.Errorf("expected 0 beads from empty file, got %d", len(stats))
	}
}

func TestReadPerBeadStatsNonexistentDirectory(t *testing.T) {
	dir := "/nonexistent/path/that/does/not/exist"

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading nonexistent directory: %v", err)
	}

	// Should return empty map without error
	if len(stats) != 0 {
		t.Errorf("expected 0 beads from nonexistent dir, got %d", len(stats))
	}
}

func TestReadPerBeadStatsSingleEntry(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Single Task","model":"haiku","success":true,"validated":true,"escalated":false,"duration_ms":500}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	if len(stats) != 1 {
		t.Errorf("expected 1 bead, got %d", len(stats))
	}

	b1 := stats["b1"]
	if b1.TotalRuns != 1 {
		t.Errorf("expected 1 total run, got %d", b1.TotalRuns)
	}
	if b1.Successes != 1 {
		t.Errorf("expected 1 success, got %d", b1.Successes)
	}
	if b1.Failures != 0 {
		t.Errorf("expected 0 failures, got %d", b1.Failures)
	}
	if b1.FailureRate() != 0.0 {
		t.Errorf("expected failure rate 0.0, got %f", b1.FailureRate())
	}
}

func TestReadPerBeadStatsMultipleBeadsMultipleAttempts(t *testing.T) {
	dir := t.TempDir()

	// Complex scenario: multiple beads with varying attempts
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:03:00Z","iteration":4,"bead_id":"b2","bead_title":"Task 2","model":"opus","success":false,"validated":false,"escalated":false,"duration_ms":2000}
{"timestamp":"2026-02-05T12:04:00Z","iteration":5,"bead_id":"b2","bead_title":"Task 2","model":"opus","success":false,"validated":false,"escalated":false,"duration_ms":2000}
{"timestamp":"2026-02-05T12:05:00Z","iteration":6,"bead_id":"b2","bead_title":"Task 2","model":"opus","success":true,"validated":true,"escalated":false,"duration_ms":2000}
{"timestamp":"2026-02-05T12:06:00Z","iteration":7,"bead_id":"b3","bead_title":"Task 3","model":"haiku","success":true,"validated":true,"escalated":false,"duration_ms":500}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	if len(stats) != 3 {
		t.Errorf("expected 3 beads, got %d", len(stats))
	}

	// b1: 3 attempts (2 success, 1 failure)
	b1 := stats["b1"]
	if b1.TotalRuns != 3 {
		t.Errorf("b1: expected 3 total runs, got %d", b1.TotalRuns)
	}
	if b1.Successes != 2 {
		t.Errorf("b1: expected 2 successes, got %d", b1.Successes)
	}
	if b1.Failures != 1 {
		t.Errorf("b1: expected 1 failure, got %d", b1.Failures)
	}

	// b2: 3 attempts (1 success, 2 failures)
	b2 := stats["b2"]
	if b2.TotalRuns != 3 {
		t.Errorf("b2: expected 3 total runs, got %d", b2.TotalRuns)
	}
	if b2.Successes != 1 {
		t.Errorf("b2: expected 1 success, got %d", b2.Successes)
	}
	if b2.Failures != 2 {
		t.Errorf("b2: expected 2 failures, got %d", b2.Failures)
	}

	// b3: 1 attempt (1 success)
	b3 := stats["b3"]
	if b3.TotalRuns != 1 {
		t.Errorf("b3: expected 1 total run, got %d", b3.TotalRuns)
	}
	if b3.Successes != 1 {
		t.Errorf("b3: expected 1 success, got %d", b3.Successes)
	}
	if b3.Failures != 0 {
		t.Errorf("b3: expected 0 failures, got %d", b3.Failures)
	}
}

func TestReadPerBeadStatsBeadTitleUpdate(t *testing.T) {
	dir := t.TempDir()

	// Same bead with different titles (should keep first title seen)
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Original Title","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b1","bead_title":"Updated Title","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	b1 := stats["b1"]
	if b1.BeadTitle != "Original Title" {
		t.Errorf("expected title 'Original Title', got %q", b1.BeadTitle)
	}
}

func TestBeadStatsNormalizeNilFields(t *testing.T) {
	// Test that nil Comments slice is normalized to empty slice
	stats := BeadStats{
		BeadID: "b1",
	}
	if stats.Comments != nil {
		t.Error("expected Comments to start as nil")
	}

	stats.normalizeNilFields()
	if stats.Comments == nil {
		t.Error("expected Comments to be non-nil after normalization")
	}
	if len(stats.Comments) != 0 {
		t.Errorf("expected empty Comments, got %d items", len(stats.Comments))
	}
}

func TestBeadStatsNormalizeNilFieldsNilReceiver(t *testing.T) {
	var stats *BeadStats
	// Should not panic
	stats.normalizeNilFields()
}

func TestBeadStatsNormalizeNilFieldsPreservesExisting(t *testing.T) {
	stats := BeadStats{
		BeadID:   "b1",
		Comments: []string{"comment1", "comment2"},
	}

	stats.normalizeNilFields()
	if len(stats.Comments) != 2 {
		t.Errorf("expected 2 comments preserved, got %d", len(stats.Comments))
	}
}

func TestReadPerBeadStatsCommentsNotNil(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	b1 := stats["b1"]
	if b1.Comments == nil {
		t.Error("expected Comments to be non-nil (empty slice) after ReadPerBeadStats")
	}
}

func TestReadPerBeadStatsIgnoreNonLogFiles(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file that doesn't match the pattern
	if err := os.WriteFile(filepath.Join(dir, "other-file.txt"), []byte("some content"), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStats(dir)
	if err != nil {
		t.Fatalf("reading per-bead stats: %v", err)
	}

	// Should only read the .jsonl file
	if len(stats) != 1 {
		t.Errorf("expected 1 bead, got %d", len(stats))
	}
	if _, exists := stats["b1"]; !exists {
		t.Error("expected b1 in stats")
	}
}

func TestLogReview(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	review := &ReviewLog{
		Timestamp:      time.Now(),
		Type:           "review",
		ReviewType:     "light",
		Iteration:      5,
		BeadID:         "abc-123",
		Model:          "sonnet",
		Passed:         true,
		FixesApplied:   1,
		BeadsCreated:   2,
		BacklogCreated: 0,
		DurationMs:     25000,
	}
	if err := l.LogReview(review); err != nil {
		t.Fatal(err)
	}

	// Read back and verify
	data, err := os.ReadFile(l.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !contains(content, `"type":"review"`) {
		t.Errorf("log should contain type field")
	}
	if !contains(content, `"review_type":"light"`) {
		t.Errorf("log should contain review_type field")
	}
	if !contains(content, `"fixes_applied":1`) {
		t.Errorf("log should contain fixes_applied field")
	}
	if !contains(content, `"beads_created":2`) {
		t.Errorf("log should contain beads_created field")
	}
}

func TestLogReviewNilLogger(t *testing.T) {
	var l *Logger
	review := &ReviewLog{
		Timestamp:  time.Now(),
		Type:       "review",
		ReviewType: "thorough",
		Iteration:  10,
		Model:      "opus",
		Passed:     false,
	}
	// Should not panic
	if err := l.LogReview(review); err != nil {
		t.Error("expected nil logger to return nil error")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
