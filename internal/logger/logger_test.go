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

func TestIterationLogOutcomeField(t *testing.T) {
	dir := t.TempDir()

	// Write a log file with and without outcome field
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"haiku","success":true,"validated":false,"escalated":false,"duration_ms":500,"outcome":"precheck_skipped"}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"error":"build failed","outcome":"build_failed"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Read back the entries
	entries, err := readLogFile(filepath.Join(dir, "run-20260205-120000.jsonl"))
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// First entry should have empty outcome (backward compatibility)
	if entries[0].Outcome != "" {
		t.Errorf("expected empty outcome for entry without field, got %q", entries[0].Outcome)
	}

	// Second entry should have precheck_skipped outcome
	if entries[1].Outcome != "precheck_skipped" {
		t.Errorf("expected 'precheck_skipped' outcome, got %q", entries[1].Outcome)
	}

	// Third entry should have build_failed outcome
	if entries[2].Outcome != "build_failed" {
		t.Errorf("expected 'build_failed' outcome, got %q", entries[2].Outcome)
	}
}

func TestLogIterationWithOutcome(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Log an iteration with outcome field
	log := &IterationLog{
		Timestamp:  time.Now(),
		Iteration:  1,
		BeadID:     "b1",
		BeadTitle:  "Test bead",
		Model:      "haiku",
		Success:    true,
		Validated:  false,
		Escalated:  false,
		DurationMs: 500,
		Outcome:    "precheck_skipped",
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
	if !contains(content, `"outcome":"precheck_skipped"`) {
		t.Errorf("log should contain outcome field with value 'precheck_skipped'")
	}
}

func TestLogIterationWithoutOutcome(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Log an iteration without outcome field (empty string)
	log := &IterationLog{
		Timestamp:  time.Now(),
		Iteration:  1,
		BeadID:     "b1",
		BeadTitle:  "Test bead",
		Model:      "sonnet",
		Success:    true,
		Validated:  true,
		Escalated:  false,
		DurationMs: 1000,
		Outcome:    "", // Empty string should be omitted
	}
	if err := l.LogIteration(log); err != nil {
		t.Fatal(err)
	}

	// Read back and verify outcome field is omitted
	data, err := os.ReadFile(l.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if contains(content, `"outcome"`) {
		t.Errorf("log should not contain outcome field when empty (omitempty)")
	}
}

func TestLogIterationWithCostAndTokens(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Log an iteration with cost and token fields
	log := &IterationLog{
		Timestamp:    time.Now(),
		Iteration:    1,
		BeadID:       "b1",
		BeadTitle:    "Test bead",
		Model:        "sonnet",
		Success:      true,
		Validated:    true,
		Escalated:    false,
		DurationMs:   1000,
		CostUSD:      0.42,
		InputTokens:  12000,
		OutputTokens: 3000,
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
	if !contains(content, `"cost_usd":0.42`) {
		t.Errorf("log should contain cost_usd field with value 0.42")
	}
	if !contains(content, `"input_tokens":12000`) {
		t.Errorf("log should contain input_tokens field with value 12000")
	}
	if !contains(content, `"output_tokens":3000`) {
		t.Errorf("log should contain output_tokens field with value 3000")
	}
}

func TestReadLogFileWithCostAndTokens(t *testing.T) {
	dir := t.TempDir()

	// Write a log file with cost and token fields
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000,"cost_usd":0.25,"input_tokens":10000,"output_tokens":2500}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"opus","success":true,"validated":true,"escalated":false,"duration_ms":2000,"cost_usd":1.50,"input_tokens":50000,"output_tokens":8000}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Read back the entries
	entries, err := readLogFile(filepath.Join(dir, "run-20260205-120000.jsonl"))
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// First entry
	if entries[0].CostUSD != 0.25 {
		t.Errorf("expected cost_usd 0.25, got %f", entries[0].CostUSD)
	}
	if entries[0].InputTokens != 10000 {
		t.Errorf("expected input_tokens 10000, got %d", entries[0].InputTokens)
	}
	if entries[0].OutputTokens != 2500 {
		t.Errorf("expected output_tokens 2500, got %d", entries[0].OutputTokens)
	}

	// Second entry
	if entries[1].CostUSD != 1.50 {
		t.Errorf("expected cost_usd 1.50, got %f", entries[1].CostUSD)
	}
	if entries[1].InputTokens != 50000 {
		t.Errorf("expected input_tokens 50000, got %d", entries[1].InputTokens)
	}
	if entries[1].OutputTokens != 8000 {
		t.Errorf("expected output_tokens 8000, got %d", entries[1].OutputTokens)
	}
}

func TestReadLogFileBackwardCompatibility(t *testing.T) {
	dir := t.TempDir()

	// Write a log file without cost and token fields (backward compatibility)
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Read back the entries
	entries, err := readLogFile(filepath.Join(dir, "run-20260205-120000.jsonl"))
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Fields should have zero values when omitted
	if entries[0].CostUSD != 0 {
		t.Errorf("expected cost_usd 0 (zero value), got %f", entries[0].CostUSD)
	}
	if entries[0].InputTokens != 0 {
		t.Errorf("expected input_tokens 0 (zero value), got %d", entries[0].InputTokens)
	}
	if entries[0].OutputTokens != 0 {
		t.Errorf("expected output_tokens 0 (zero value), got %d", entries[0].OutputTokens)
	}
}

func TestReadLogFileSkipsNonIterationRecords(t *testing.T) {
	dir := t.TempDir()

	iteration := IterationLog{
		Timestamp:  time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC),
		Iteration:  1,
		BeadID:     "b1",
		BeadTitle:  "Task 1",
		Model:      "sonnet",
		Success:    true,
		Validated:  true,
		Escalated:  false,
		DurationMs: 1000,
	}
	review := ReviewLog{
		Timestamp:      time.Date(2026, 2, 5, 12, 1, 0, 0, time.UTC),
		Type:           "review",
		ReviewType:     "light",
		Iteration:      1,
		BeadID:         "b1",
		Model:          "sonnet",
		Passed:         true,
		FixesApplied:   1,
		BeadsCreated:   0,
		BacklogCreated: 0,
		DurationMs:     500,
	}
	phase := TDDPhaseRecord{
		Type:               "tdd_phase",
		Timestamp:          time.Date(2026, 2, 5, 12, 2, 0, 0, time.UTC),
		BeadID:             "b2",
		Phase:              "red",
		CycleNumber:        1,
		Model:              "haiku",
		Tier:               "low",
		InputTokens:        100,
		OutputTokens:       50,
		DurationMs:         250,
		Success:            false,
		Escalated:          false,
		CriteriaTotal:      2,
		CriteriaCovered:    1,
		CriteriaUntestable: 0,
	}
	summary := TDDSummaryRecord{
		Type:            "tdd_summary",
		Timestamp:       time.Date(2026, 2, 5, 12, 3, 0, 0, time.UTC),
		BeadID:          "b3",
		TotalCycles:     1,
		TotalPhases:     3,
		Success:         true,
		TotalDurationMs: 700,
	}

	var buf bytes.Buffer
	for _, record := range []any{iteration, review, phase, summary} {
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatalf("marshal record: %v", err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := readLogFile(filepath.Join(dir, "run-20260205-120000.jsonl"))
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 iteration entry, got %d", len(entries))
	}
	if entries[0].BeadID != "b1" {
		t.Errorf("expected bead_id b1, got %q", entries[0].BeadID)
	}
	if !entries[0].Success {
		t.Errorf("expected success true for iteration log")
	}
}

// TestTDDPhaseRecord_Fields verifies TDDPhaseRecord has a type discriminator
// and all per-phase metric fields for JSONL logging.
func TestTDDPhaseRecord_Fields(t *testing.T) {
	rec := TDDPhaseRecord{
		Type:               "tdd_phase",
		Timestamp:          time.Now(),
		BeadID:             "bead-xyz",
		Phase:              "red",
		CycleNumber:        1,
		Model:              "haiku",
		Tier:               "low",
		InputTokens:        1000,
		OutputTokens:       500,
		DurationMs:         2500,
		Success:            false,
		Escalated:          false,
		EscalatedFrom:      "",
		CriteriaTotal:      4,
		CriteriaCovered:    2,
		CriteriaUntestable: 0,
	}

	if rec.Type != "tdd_phase" {
		t.Errorf("Type = %q, want %q", rec.Type, "tdd_phase")
	}
	if rec.BeadID != "bead-xyz" {
		t.Errorf("BeadID = %q, want %q", rec.BeadID, "bead-xyz")
	}
	if rec.Phase != "red" {
		t.Errorf("Phase = %q, want %q", rec.Phase, "red")
	}
	if rec.CycleNumber != 1 {
		t.Errorf("CycleNumber = %d, want 1", rec.CycleNumber)
	}
	if rec.Model != "haiku" {
		t.Errorf("Model = %q, want %q", rec.Model, "haiku")
	}
	if rec.Tier != "low" {
		t.Errorf("Tier = %q, want %q", rec.Tier, "low")
	}
	if rec.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", rec.InputTokens)
	}
	if rec.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", rec.OutputTokens)
	}
	if rec.DurationMs != 2500 {
		t.Errorf("DurationMs = %d, want 2500", rec.DurationMs)
	}
	if rec.Success {
		t.Error("Success should be false")
	}
	if rec.CriteriaTotal != 4 {
		t.Errorf("CriteriaTotal = %d, want 4", rec.CriteriaTotal)
	}
}

// TestTDDSummaryRecord_Fields verifies TDDSummaryRecord has a type discriminator
// and all summary fields for a completed TDD bead run.
func TestTDDSummaryRecord_Fields(t *testing.T) {
	rec := TDDSummaryRecord{
		Type:            "tdd_summary",
		Timestamp:       time.Now(),
		BeadID:          "bead-xyz",
		TotalCycles:     3,
		TotalPhases:     9,
		Success:         true,
		TotalDurationMs: 15000,
	}

	if rec.Type != "tdd_summary" {
		t.Errorf("Type = %q, want %q", rec.Type, "tdd_summary")
	}
	if rec.BeadID != "bead-xyz" {
		t.Errorf("BeadID = %q, want %q", rec.BeadID, "bead-xyz")
	}
	if rec.TotalCycles != 3 {
		t.Errorf("TotalCycles = %d, want 3", rec.TotalCycles)
	}
	if rec.TotalPhases != 9 {
		t.Errorf("TotalPhases = %d, want 9", rec.TotalPhases)
	}
	if !rec.Success {
		t.Error("Success should be true")
	}
	if rec.TotalDurationMs != 15000 {
		t.Errorf("TotalDurationMs = %d, want 15000", rec.TotalDurationMs)
	}
}

// TestLogTDDPhase_WritesRecordToJSONL verifies LogTDDPhase writes a TDDPhaseRecord
// to the JSONL log file using the existing encoder pattern.
func TestLogTDDPhase_WritesRecordToJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	rec := &TDDPhaseRecord{
		Type:        "tdd_phase",
		Timestamp:   time.Now(),
		BeadID:      "bead-123",
		Phase:       "green",
		CycleNumber: 2,
		Model:       "sonnet",
		Tier:        "medium",
		Success:     true,
		DurationMs:  4500,
	}

	if err := l.LogTDDPhase(rec); err != nil {
		t.Fatalf("LogTDDPhase() error: %v", err)
	}

	data, err := os.ReadFile(l.FilePath())
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)

	if !contains(content, `"type":"tdd_phase"`) {
		t.Errorf("log should contain type tdd_phase, got: %s", content)
	}
	if !contains(content, `"bead_id":"bead-123"`) {
		t.Errorf("log should contain bead_id, got: %s", content)
	}
	if !contains(content, `"phase":"green"`) {
		t.Errorf("log should contain phase green, got: %s", content)
	}
	if !contains(content, `"cycle_number":2`) {
		t.Errorf("log should contain cycle_number 2, got: %s", content)
	}
}

// TestLogTDDPhase_NilLoggerReturnsNil verifies that calling LogTDDPhase on a nil
// Logger returns nil without panicking.
func TestLogTDDPhase_NilLoggerReturnsNil(t *testing.T) {
	var l *Logger
	rec := &TDDPhaseRecord{Type: "tdd_phase", BeadID: "b1"}
	if err := l.LogTDDPhase(rec); err != nil {
		t.Errorf("nil logger LogTDDPhase should return nil, got: %v", err)
	}
}

// TestLogTDDSummary_WritesRecordToJSONL verifies LogTDDSummary writes a TDDSummaryRecord
// to the JSONL log file using the existing encoder pattern.
func TestLogTDDSummary_WritesRecordToJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	rec := &TDDSummaryRecord{
		Type:            "tdd_summary",
		Timestamp:       time.Now(),
		BeadID:          "bead-456",
		TotalCycles:     4,
		TotalPhases:     12,
		Success:         true,
		TotalDurationMs: 30000,
	}

	if err := l.LogTDDSummary(rec); err != nil {
		t.Fatalf("LogTDDSummary() error: %v", err)
	}

	data, err := os.ReadFile(l.FilePath())
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)

	if !contains(content, `"type":"tdd_summary"`) {
		t.Errorf("log should contain type tdd_summary, got: %s", content)
	}
	if !contains(content, `"bead_id":"bead-456"`) {
		t.Errorf("log should contain bead_id, got: %s", content)
	}
	if !contains(content, `"total_cycles":4`) {
		t.Errorf("log should contain total_cycles 4, got: %s", content)
	}
	if !contains(content, `"total_duration_ms":30000`) {
		t.Errorf("log should contain total_duration_ms, got: %s", content)
	}
}

// TestLogTDDSummary_NilLoggerReturnsNil verifies that calling LogTDDSummary on a nil
// Logger returns nil without panicking.
func TestLogTDDSummary_NilLoggerReturnsNil(t *testing.T) {
	var l *Logger
	rec := &TDDSummaryRecord{Type: "tdd_summary", BeadID: "b1"}
	if err := l.LogTDDSummary(rec); err != nil {
		t.Errorf("nil logger LogTDDSummary should return nil, got: %v", err)
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

func TestReadAllLogsFiltered_NilFilterIncludesAll(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"error":"build failed"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadAllLogsFiltered(dir, nil)
	if err != nil {
		t.Fatalf("reading filtered logs: %v", err)
	}

	if stats.Total != 2 {
		t.Errorf("expected 2 total with nil filter, got %d", stats.Total)
	}
}

func TestReadAllLogsFiltered_EmptyFilterIncludesAll(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"error":"build failed"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	filter := make(map[string]bool)
	stats, err := ReadAllLogsFiltered(dir, filter)
	if err != nil {
		t.Fatalf("reading filtered logs: %v", err)
	}

	if stats.Total != 2 {
		t.Errorf("expected 2 total with empty filter, got %d", stats.Total)
	}
}

func TestReadAllLogsFiltered_FilterIncludesOnlyMatchingBeads(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"error":"build failed"}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"opus","success":true,"validated":true,"escalated":true,"escalated_to":"opus","duration_ms":3000}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Filter to only include b1 and b3
	filter := map[string]bool{
		"b1": true,
		"b3": true,
	}

	stats, err := ReadAllLogsFiltered(dir, filter)
	if err != nil {
		t.Fatalf("reading filtered logs: %v", err)
	}

	if stats.Total != 2 {
		t.Errorf("expected 2 total (b1 and b3), got %d", stats.Total)
	}

	if stats.Succeeded != 2 {
		t.Errorf("expected 2 succeeded (b1 and b3), got %d", stats.Succeeded)
	}

	if stats.Failed != 0 {
		t.Errorf("expected 0 failed (b2 excluded), got %d", stats.Failed)
	}
}

func TestReadPerBeadStatsFiltered_NilFilterIncludesAll(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"error":"build failed"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadPerBeadStatsFiltered(dir, nil)
	if err != nil {
		t.Fatalf("reading filtered per-bead stats: %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("expected 2 beads with nil filter, got %d", len(stats))
	}
}

func TestReadPerBeadStatsFiltered_EmptyFilterIncludesAll(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"error":"build failed"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	filter := make(map[string]bool)
	stats, err := ReadPerBeadStatsFiltered(dir, filter)
	if err != nil {
		t.Fatalf("reading filtered per-bead stats: %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("expected 2 beads with empty filter, got %d", len(stats))
	}
}

func TestReadPerBeadStatsFiltered_FilterIncludesOnlyMatchingBeads(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":1000}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":2000,"error":"build failed"}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b2","bead_title":"Task 2","model":"opus","success":true,"validated":true,"escalated":true,"escalated_to":"opus","duration_ms":3000}
{"timestamp":"2026-02-05T12:03:00Z","iteration":4,"bead_id":"b3","bead_title":"Task 3","model":"haiku","success":true,"validated":true,"escalated":false,"duration_ms":500}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Filter to only include b1 and b3
	filter := map[string]bool{
		"b1": true,
		"b3": true,
	}

	stats, err := ReadPerBeadStatsFiltered(dir, filter)
	if err != nil {
		t.Fatalf("reading filtered per-bead stats: %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("expected 2 beads (b1 and b3), got %d", len(stats))
	}

	if _, exists := stats["b1"]; !exists {
		t.Error("expected b1 in filtered stats")
	}

	if _, exists := stats["b3"]; !exists {
		t.Error("expected b3 in filtered stats")
	}

	if _, exists := stats["b2"]; exists {
		t.Error("b2 should be excluded from filtered stats")
	}

	// Verify b1 has 2 runs (both entries for b1)
	if stats["b1"].TotalRuns != 2 {
		t.Errorf("expected b1 to have 2 runs, got %d", stats["b1"].TotalRuns)
	}
}
