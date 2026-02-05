package logger

import (
	"os"
	"path/filepath"
	"testing"
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
