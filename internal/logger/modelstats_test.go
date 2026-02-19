package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Expected failure: ModelStats struct does not exist yet
func TestModelStats_Struct(t *testing.T) {
	ms := ModelStats{
		Model:           "opus",
		Iterations:      10,
		Successes:       8,
		Failures:        2,
		EscalationsTo:   3,
		EscalationsFrom: 1,
		TotalCostUSD:    12.50,
	}

	if ms.Model != "opus" {
		t.Errorf("Model = %s, want opus", ms.Model)
	}
	if ms.Iterations != 10 {
		t.Errorf("Iterations = %d, want 10", ms.Iterations)
	}
	if ms.Successes != 8 {
		t.Errorf("Successes = %d, want 8", ms.Successes)
	}
	if ms.Failures != 2 {
		t.Errorf("Failures = %d, want 2", ms.Failures)
	}
}

// Expected failure: ModelStats.SuccessRate() method does not exist yet
func TestModelStats_SuccessRate(t *testing.T) {
	tests := []struct {
		name     string
		stats    ModelStats
		expected float64
	}{
		{
			name:     "empty",
			stats:    ModelStats{},
			expected: 0.0,
		},
		{
			name: "all successes",
			stats: ModelStats{
				Iterations: 5,
				Successes:  5,
				Failures:   0,
			},
			expected: 1.0,
		},
		{
			name: "all failures",
			stats: ModelStats{
				Iterations: 3,
				Successes:  0,
				Failures:   3,
			},
			expected: 0.0,
		},
		{
			name: "mixed",
			stats: ModelStats{
				Iterations: 10,
				Successes:  7,
				Failures:   3,
			},
			expected: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stats.SuccessRate()
			if diff := got - tt.expected; diff > 0.001 || diff < -0.001 {
				t.Errorf("SuccessRate() = %.3f, want %.3f", got, tt.expected)
			}
		})
	}
}

// Expected failure: ReadModelStats function does not exist yet
func TestReadModelStats_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	stats, err := ReadModelStats(dir)
	if err != nil {
		t.Fatalf("ReadModelStats failed: %v", err)
	}

	if len(stats) != 0 {
		t.Errorf("Expected 0 models in empty dir, got %d", len(stats))
	}
}

// Expected failure: ReadModelStats function does not exist yet
func TestReadModelStats_SingleModel(t *testing.T) {
	dir := t.TempDir()
	runID := "20260211-120000"

	logs := []IterationLog{
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			Escalated:  false,
			CostUSD:    0.50,
			DurationMs: 30000,
		},
		{
			BeadID:     "b2",
			Model:      "opus",
			Success:    false,
			Escalated:  false,
			CostUSD:    0.45,
			DurationMs: 25000,
		},
		{
			BeadID:     "b3",
			Model:      "opus",
			Success:    true,
			Escalated:  false,
			CostUSD:    0.55,
			DurationMs: 32000,
		},
	}

	writeTestLogFile(t, dir, runID, logs)

	stats, err := ReadModelStats(dir)
	if err != nil {
		t.Fatalf("ReadModelStats failed: %v", err)
	}

	if len(stats) != 1 {
		t.Fatalf("Expected 1 model, got %d", len(stats))
	}

	opus := stats["opus"]
	if opus.Model != "opus" {
		t.Errorf("Model = %s, want opus", opus.Model)
	}
	if opus.Iterations != 3 {
		t.Errorf("Iterations = %d, want 3", opus.Iterations)
	}
	if opus.Successes != 2 {
		t.Errorf("Successes = %d, want 2", opus.Successes)
	}
	if opus.Failures != 1 {
		t.Errorf("Failures = %d, want 1", opus.Failures)
	}
	if opus.TotalCostUSD != 1.50 {
		t.Errorf("TotalCostUSD = %.2f, want 1.50", opus.TotalCostUSD)
	}
}

// Expected failure: ReadModelStats function does not exist yet
func TestReadModelStats_MultipleModels(t *testing.T) {
	dir := t.TempDir()
	runID := "20260211-120000"

	logs := []IterationLog{
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			Escalated:  false,
			CostUSD:    0.50,
			DurationMs: 30000,
		},
		{
			BeadID:      "b2",
			Model:       "sonnet",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "opus",
			CostUSD:     0.25,
			DurationMs:  20000,
		},
		{
			BeadID:     "b2",
			Model:      "opus",
			Success:    true,
			Escalated:  false,
			CostUSD:    0.55,
			DurationMs: 32000,
		},
		{
			BeadID:     "b3",
			Model:      "haiku",
			Success:    true,
			Escalated:  false,
			CostUSD:    0.10,
			DurationMs: 5000,
		},
	}

	writeTestLogFile(t, dir, runID, logs)

	stats, err := ReadModelStats(dir)
	if err != nil {
		t.Fatalf("ReadModelStats failed: %v", err)
	}

	if len(stats) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(stats))
	}

	// Check opus
	opus := stats["opus"]
	if opus.Iterations != 2 {
		t.Errorf("opus Iterations = %d, want 2", opus.Iterations)
	}
	if opus.Successes != 2 {
		t.Errorf("opus Successes = %d, want 2", opus.Successes)
	}
	if opus.EscalationsTo != 1 {
		t.Errorf("opus EscalationsTo = %d, want 1 (escalated from sonnet)", opus.EscalationsTo)
	}

	// Check sonnet
	sonnet := stats["sonnet"]
	if sonnet.Iterations != 1 {
		t.Errorf("sonnet Iterations = %d, want 1", sonnet.Iterations)
	}
	if sonnet.Failures != 1 {
		t.Errorf("sonnet Failures = %d, want 1", sonnet.Failures)
	}
	if sonnet.EscalationsFrom != 1 {
		t.Errorf("sonnet EscalationsFrom = %d, want 1", sonnet.EscalationsFrom)
	}

	// Check haiku
	haiku := stats["haiku"]
	if haiku.Iterations != 1 {
		t.Errorf("haiku Iterations = %d, want 1", haiku.Iterations)
	}
	if haiku.Successes != 1 {
		t.Errorf("haiku Successes = %d, want 1", haiku.Successes)
	}
}

// Expected failure: ReadModelStats function does not exist yet
func TestReadModelStats_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	// First run
	logs1 := []IterationLog{
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			CostUSD:    0.50,
			DurationMs: 30000,
		},
		{
			BeadID:     "b2",
			Model:      "sonnet",
			Success:    false,
			CostUSD:    0.25,
			DurationMs: 20000,
		},
	}
	writeTestLogFile(t, dir, "20260211-120000", logs1)

	// Second run
	logs2 := []IterationLog{
		{
			BeadID:     "b3",
			Model:      "opus",
			Success:    false,
			CostUSD:    0.45,
			DurationMs: 28000,
		},
		{
			BeadID:     "b4",
			Model:      "sonnet",
			Success:    true,
			CostUSD:    0.30,
			DurationMs: 22000,
		},
	}
	writeTestLogFile(t, dir, "20260211-130000", logs2)

	stats, err := ReadModelStats(dir)
	if err != nil {
		t.Fatalf("ReadModelStats failed: %v", err)
	}

	// Opus: 2 iterations (1 success, 1 failure)
	opus := stats["opus"]
	if opus.Iterations != 2 {
		t.Errorf("opus Iterations = %d, want 2", opus.Iterations)
	}
	if opus.Successes != 1 {
		t.Errorf("opus Successes = %d, want 1", opus.Successes)
	}
	if opus.Failures != 1 {
		t.Errorf("opus Failures = %d, want 1", opus.Failures)
	}
	if opus.TotalCostUSD != 0.95 {
		t.Errorf("opus TotalCostUSD = %.2f, want 0.95", opus.TotalCostUSD)
	}

	// Sonnet: 2 iterations (1 success, 1 failure)
	sonnet := stats["sonnet"]
	if sonnet.Iterations != 2 {
		t.Errorf("sonnet Iterations = %d, want 2", sonnet.Iterations)
	}
	if sonnet.Successes != 1 {
		t.Errorf("sonnet Successes = %d, want 1", sonnet.Successes)
	}
	if sonnet.Failures != 1 {
		t.Errorf("sonnet Failures = %d, want 1", sonnet.Failures)
	}
}

// Expected failure: ReadRunModelStats function does not exist yet
func TestReadRunModelStats_SingleRun(t *testing.T) {
	dir := t.TempDir()
	runID := "20260211-120000"

	logs := []IterationLog{
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			CostUSD:    0.50,
			DurationMs: 30000,
		},
		{
			BeadID:     "b2",
			Model:      "sonnet",
			Success:    false,
			CostUSD:    0.25,
			DurationMs: 20000,
		},
	}
	writeTestLogFile(t, dir, runID, logs)

	// Also write another run that should be excluded
	logsOther := []IterationLog{
		{
			BeadID:     "b3",
			Model:      "haiku",
			Success:    true,
			CostUSD:    0.10,
			DurationMs: 5000,
		},
	}
	writeTestLogFile(t, dir, "20260211-130000", logsOther)

	stats, err := ReadRunModelStats(dir, runID)
	if err != nil {
		t.Fatalf("ReadRunModelStats failed: %v", err)
	}

	// Should only include data from the specified run
	if len(stats) != 2 {
		t.Fatalf("Expected 2 models from run %s, got %d", runID, len(stats))
	}

	if _, exists := stats["opus"]; !exists {
		t.Error("Expected opus in stats")
	}
	if _, exists := stats["sonnet"]; !exists {
		t.Error("Expected sonnet in stats")
	}
	if _, exists := stats["haiku"]; exists {
		t.Error("haiku should not be in stats (different run)")
	}
}

// Expected failure: ReadRunModelStats function does not exist yet
func TestReadRunModelStats_EmptyRunID(t *testing.T) {
	dir := t.TempDir()

	logs := []IterationLog{
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			CostUSD:    0.50,
			DurationMs: 30000,
		},
	}
	writeTestLogFile(t, dir, "20260211-120000", logs)

	// Empty runID should return empty stats
	stats, err := ReadRunModelStats(dir, "")
	if err != nil {
		t.Fatalf("ReadRunModelStats failed: %v", err)
	}

	if len(stats) != 0 {
		t.Errorf("Expected 0 models with empty runID, got %d", len(stats))
	}
}

// Expected failure: ReadRunModelStats function does not exist yet
func TestReadRunModelStats_NonexistentRun(t *testing.T) {
	dir := t.TempDir()

	logs := []IterationLog{
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			CostUSD:    0.50,
			DurationMs: 30000,
		},
	}
	writeTestLogFile(t, dir, "20260211-120000", logs)

	// Request nonexistent run
	stats, err := ReadRunModelStats(dir, "20260211-999999")
	if err != nil {
		t.Fatalf("ReadRunModelStats failed: %v", err)
	}

	if len(stats) != 0 {
		t.Errorf("Expected 0 models for nonexistent run, got %d", len(stats))
	}
}

// Expected failure: CostPerCompletedBead function does not exist yet
func TestCostPerCompletedBead_SingleBeadSingleAttempt(t *testing.T) {
	dir := t.TempDir()
	runID := "20260211-120000"

	logs := []IterationLog{
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			CostUSD:    0.50,
			DurationMs: 30000,
		},
	}
	writeTestLogFile(t, dir, runID, logs)

	costs, err := CostPerCompletedBead(dir)
	if err != nil {
		t.Fatalf("CostPerCompletedBead failed: %v", err)
	}

	if len(costs) != 1 {
		t.Fatalf("Expected 1 completed bead, got %d", len(costs))
	}

	cost, exists := costs["b1"]
	if !exists {
		t.Fatal("Expected b1 in costs")
	}

	if cost != 0.50 {
		t.Errorf("Cost for b1 = %.2f, want 0.50", cost)
	}
}

// Expected failure: CostPerCompletedBead function does not exist yet
func TestCostPerCompletedBead_MultipleAttempts(t *testing.T) {
	dir := t.TempDir()
	runID := "20260211-120000"

	// Bead b1: haiku fails (0.10), escalates to sonnet fails (0.25), escalates to opus succeeds (0.55)
	// Total cost for b1 completion: 0.10 + 0.25 + 0.55 = 0.90
	logs := []IterationLog{
		{
			BeadID:      "b1",
			Model:       "haiku",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "sonnet",
			CostUSD:     0.10,
			DurationMs:  5000,
		},
		{
			BeadID:      "b1",
			Model:       "sonnet",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "opus",
			CostUSD:     0.25,
			DurationMs:  20000,
		},
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			Escalated:  false,
			CostUSD:    0.55,
			DurationMs: 30000,
		},
	}
	writeTestLogFile(t, dir, runID, logs)

	costs, err := CostPerCompletedBead(dir)
	if err != nil {
		t.Fatalf("CostPerCompletedBead failed: %v", err)
	}

	if len(costs) != 1 {
		t.Fatalf("Expected 1 completed bead, got %d", len(costs))
	}

	cost := costs["b1"]
	expected := 0.90
	if diff := cost - expected; diff > 0.001 || diff < -0.001 {
		t.Errorf("Cost for b1 = %.2f, want %.2f", cost, expected)
	}
}

// Expected failure: CostPerCompletedBead function does not exist yet
func TestCostPerCompletedBead_IncompleteBeadsExcluded(t *testing.T) {
	dir := t.TempDir()
	runID := "20260211-120000"

	logs := []IterationLog{
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			CostUSD:    0.50,
			DurationMs: 30000,
		},
		{
			BeadID:     "b2",
			Model:      "sonnet",
			Success:    false,
			CostUSD:    0.25,
			DurationMs: 20000,
		},
		{
			BeadID:     "b3",
			Model:      "haiku",
			Success:    true,
			CostUSD:    0.10,
			DurationMs: 5000,
		},
	}
	writeTestLogFile(t, dir, runID, logs)

	costs, err := CostPerCompletedBead(dir)
	if err != nil {
		t.Fatalf("CostPerCompletedBead failed: %v", err)
	}

	if len(costs) != 2 {
		t.Fatalf("Expected 2 completed beads, got %d", len(costs))
	}

	if _, exists := costs["b1"]; !exists {
		t.Error("Expected b1 in costs (succeeded)")
	}
	if _, exists := costs["b3"]; !exists {
		t.Error("Expected b3 in costs (succeeded)")
	}
	if _, exists := costs["b2"]; exists {
		t.Error("b2 should not be in costs (failed)")
	}
}

// Expected failure: CostPerCompletedBead function does not exist yet
func TestCostPerCompletedBead_MultipleBeadsMultipleAttempts(t *testing.T) {
	dir := t.TempDir()
	runID := "20260211-120000"

	logs := []IterationLog{
		// b1: haiku fails (0.10), sonnet succeeds (0.25) = 0.35 total
		{
			BeadID:      "b1",
			Model:       "haiku",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "sonnet",
			CostUSD:     0.10,
			DurationMs:  5000,
		},
		{
			BeadID:     "b1",
			Model:      "sonnet",
			Success:    true,
			Escalated:  false,
			CostUSD:    0.25,
			DurationMs: 20000,
		},
		// b2: opus succeeds first try (0.50) = 0.50 total
		{
			BeadID:     "b2",
			Model:      "opus",
			Success:    true,
			Escalated:  false,
			CostUSD:    0.50,
			DurationMs: 30000,
		},
		// b3: haiku succeeds first try (0.08) = 0.08 total
		{
			BeadID:     "b3",
			Model:      "haiku",
			Success:    true,
			Escalated:  false,
			CostUSD:    0.08,
			DurationMs: 4500,
		},
	}
	writeTestLogFile(t, dir, runID, logs)

	costs, err := CostPerCompletedBead(dir)
	if err != nil {
		t.Fatalf("CostPerCompletedBead failed: %v", err)
	}

	if len(costs) != 3 {
		t.Fatalf("Expected 3 completed beads, got %d", len(costs))
	}

	// Check b1
	if diff := costs["b1"] - 0.35; diff > 0.001 || diff < -0.001 {
		t.Errorf("Cost for b1 = %.2f, want 0.35", costs["b1"])
	}

	// Check b2
	if diff := costs["b2"] - 0.50; diff > 0.001 || diff < -0.001 {
		t.Errorf("Cost for b2 = %.2f, want 0.50", costs["b2"])
	}

	// Check b3
	if diff := costs["b3"] - 0.08; diff > 0.001 || diff < -0.001 {
		t.Errorf("Cost for b3 = %.2f, want 0.08", costs["b3"])
	}
}

// Expected failure: CostPerCompletedBead function does not exist yet
func TestCostPerCompletedBead_AcrossMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	// First run: b1 fails with haiku
	logs1 := []IterationLog{
		{
			BeadID:      "b1",
			Model:       "haiku",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "sonnet",
			CostUSD:     0.10,
			DurationMs:  5000,
		},
	}
	writeTestLogFile(t, dir, "20260211-120000", logs1)

	// Second run: b1 succeeds with sonnet
	logs2 := []IterationLog{
		{
			BeadID:     "b1",
			Model:      "sonnet",
			Success:    true,
			Escalated:  false,
			CostUSD:    0.25,
			DurationMs: 20000,
		},
	}
	writeTestLogFile(t, dir, "20260211-130000", logs2)

	costs, err := CostPerCompletedBead(dir)
	if err != nil {
		t.Fatalf("CostPerCompletedBead failed: %v", err)
	}

	if len(costs) != 1 {
		t.Fatalf("Expected 1 completed bead, got %d", len(costs))
	}

	// Total cost should include both attempts across runs
	expected := 0.35
	if diff := costs["b1"] - expected; diff > 0.001 || diff < -0.001 {
		t.Errorf("Cost for b1 = %.2f, want %.2f (sum across runs)", costs["b1"], expected)
	}
}

// Expected failure: ModelStats struct does not exist yet
func TestModelStats_EscalationTracking(t *testing.T) {
	dir := t.TempDir()
	runID := "20260211-120000"

	logs := []IterationLog{
		{
			BeadID:      "b1",
			Model:       "haiku",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "sonnet",
			CostUSD:     0.10,
			DurationMs:  5000,
		},
		{
			BeadID:      "b1",
			Model:       "sonnet",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "opus",
			CostUSD:     0.25,
			DurationMs:  20000,
		},
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			Escalated:  false,
			CostUSD:    0.55,
			DurationMs: 30000,
		},
	}
	writeTestLogFile(t, dir, runID, logs)

	stats, err := ReadModelStats(dir)
	if err != nil {
		t.Fatalf("ReadModelStats failed: %v", err)
	}

	// Haiku escalated from
	haiku := stats["haiku"]
	if haiku.EscalationsFrom != 1 {
		t.Errorf("haiku EscalationsFrom = %d, want 1", haiku.EscalationsFrom)
	}
	if haiku.EscalationsTo != 0 {
		t.Errorf("haiku EscalationsTo = %d, want 0", haiku.EscalationsTo)
	}

	// Sonnet escalated from and to
	sonnet := stats["sonnet"]
	if sonnet.EscalationsFrom != 1 {
		t.Errorf("sonnet EscalationsFrom = %d, want 1", sonnet.EscalationsFrom)
	}
	if sonnet.EscalationsTo != 1 {
		t.Errorf("sonnet EscalationsTo = %d, want 1 (from haiku)", sonnet.EscalationsTo)
	}

	// Opus escalated to only
	opus := stats["opus"]
	if opus.EscalationsFrom != 0 {
		t.Errorf("opus EscalationsFrom = %d, want 0", opus.EscalationsFrom)
	}
	if opus.EscalationsTo != 1 {
		t.Errorf("opus EscalationsTo = %d, want 1 (from sonnet)", opus.EscalationsTo)
	}
}

// Expected failure: ReadModelStats function does not exist yet
func TestReadModelStats_BackwardCompatibility(t *testing.T) {
	dir := t.TempDir()

	// Write log entries without escalation fields (old format)
	logContent := `{"timestamp":"2026-02-11T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"duration_ms":1000,"cost_usd":0.25}
{"timestamp":"2026-02-11T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"opus","success":false,"validated":false,"duration_ms":2000,"cost_usd":0.50}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260211-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadModelStats(dir)
	if err != nil {
		t.Fatalf("ReadModelStats failed: %v", err)
	}

	// Should handle missing fields gracefully
	sonnet := stats["sonnet"]
	if sonnet.Iterations != 1 {
		t.Errorf("sonnet Iterations = %d, want 1", sonnet.Iterations)
	}
	if sonnet.Successes != 1 {
		t.Errorf("sonnet Successes = %d, want 1", sonnet.Successes)
	}
	if sonnet.EscalationsFrom != 0 {
		t.Errorf("sonnet EscalationsFrom = %d, want 0 (missing field)", sonnet.EscalationsFrom)
	}

	opus := stats["opus"]
	if opus.Failures != 1 {
		t.Errorf("opus Failures = %d, want 1", opus.Failures)
	}
}

// Expected failure: readLogFile is private, but this tests the public API surface
func TestReadModelStats_MalformedJSON(t *testing.T) {
	dir := t.TempDir()

	// Write partially valid JSONL
	logContent := `{"timestamp":"2026-02-11T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"opus","success":true,"validated":true,"duration_ms":1000,"cost_usd":0.50}
this is not json
{"timestamp":"2026-02-11T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":true,"validated":true,"duration_ms":500,"cost_usd":0.25}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260211-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadModelStats(dir)
	if err != nil {
		t.Fatalf("ReadModelStats failed: %v", err)
	}

	// Should parse valid entries before error
	if len(stats) == 0 {
		t.Error("Expected at least one model parsed before error")
	}

	if _, exists := stats["opus"]; !exists {
		t.Error("Expected opus (first valid entry) in stats")
	}
}

// TestCostPerSpec_AggregatesAcrossMultipleFiles verifies stats accumulate across multiple run files
func TestCostPerSpec_AggregatesAcrossMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	logs1 := []IterationLog{
		{BeadID: "b1", Model: "opus", Success: true, CostUSD: 0.50, SpecID: "spec-A"},
	}
	writeTestLogFile(t, dir, "20260218-120000", logs1)

	logs2 := []IterationLog{
		{BeadID: "b2", Model: "sonnet", Success: true, CostUSD: 0.25, SpecID: "spec-A"},
		{BeadID: "b3", Model: "haiku", Success: true, CostUSD: 0.10, SpecID: "spec-B"},
	}
	writeTestLogFile(t, dir, "20260218-130000", logs2)

	result, err := CostPerSpec(dir)
	if err != nil {
		t.Fatalf("CostPerSpec failed: %v", err)
	}
	if result["spec-A"].Iterations != 2 {
		t.Errorf("spec-A Iterations = %d, want 2", result["spec-A"].Iterations)
	}
	if result["spec-A"].Beads != 2 {
		t.Errorf("spec-A Beads = %d, want 2", result["spec-A"].Beads)
	}
	if diff := result["spec-A"].TotalCostUSD - 0.75; diff > 0.001 || diff < -0.001 {
		t.Errorf("spec-A TotalCostUSD = %.2f, want 0.75", result["spec-A"].TotalCostUSD)
	}
	if result["spec-B"].Iterations != 1 {
		t.Errorf("spec-B Iterations = %d, want 1", result["spec-B"].Iterations)
	}
}

// TestCostPerSpec_ModelMixTracksModelUsage verifies ModelMix counts iterations per model per spec
func TestCostPerSpec_ModelMixTracksModelUsage(t *testing.T) {
	dir := t.TempDir()

	logs := []IterationLog{
		{BeadID: "b1", Model: "haiku", Success: false, CostUSD: 0.05, SpecID: "spec-A"},
		{BeadID: "b1", Model: "sonnet", Success: false, CostUSD: 0.15, SpecID: "spec-A"},
		{BeadID: "b1", Model: "opus", Success: true, CostUSD: 0.50, SpecID: "spec-A"},
		{BeadID: "b2", Model: "haiku", Success: true, CostUSD: 0.05, SpecID: "spec-A"},
	}
	writeTestLogFile(t, dir, "20260218-120000", logs)

	result, err := CostPerSpec(dir)
	if err != nil {
		t.Fatalf("CostPerSpec failed: %v", err)
	}
	mix := result["spec-A"].ModelMix
	if mix["haiku"] != 2 {
		t.Errorf("ModelMix[haiku] = %d, want 2", mix["haiku"])
	}
	if mix["sonnet"] != 1 {
		t.Errorf("ModelMix[sonnet] = %d, want 1", mix["sonnet"])
	}
	if mix["opus"] != 1 {
		t.Errorf("ModelMix[opus] = %d, want 1", mix["opus"])
	}
}

// TestCostPerSpec_CountsDistinctBeads verifies Beads counts distinct bead_ids per spec
func TestCostPerSpec_CountsDistinctBeads(t *testing.T) {
	dir := t.TempDir()

	// b1 appears twice (retry), b2 once — 2 distinct beads for spec-A
	logs := []IterationLog{
		{BeadID: "b1", Model: "opus", Success: false, CostUSD: 0.20, SpecID: "spec-A"},
		{BeadID: "b1", Model: "opus", Success: true, CostUSD: 0.30, SpecID: "spec-A"},
		{BeadID: "b2", Model: "sonnet", Success: true, CostUSD: 0.25, SpecID: "spec-A"},
	}
	writeTestLogFile(t, dir, "20260218-120000", logs)

	result, err := CostPerSpec(dir)
	if err != nil {
		t.Fatalf("CostPerSpec failed: %v", err)
	}
	if result["spec-A"].Beads != 2 {
		t.Errorf("spec-A Beads = %d, want 2 (b1 and b2 are distinct)", result["spec-A"].Beads)
	}
}

// TestCostPerSpec_CountsIterationsPerSpec verifies Iterations counts all log entries per spec
func TestCostPerSpec_CountsIterationsPerSpec(t *testing.T) {
	dir := t.TempDir()

	logs := []IterationLog{
		{BeadID: "b1", Model: "opus", Success: false, CostUSD: 0.20, SpecID: "spec-A"},
		{BeadID: "b1", Model: "opus", Success: true, CostUSD: 0.30, SpecID: "spec-A"},
		{BeadID: "b2", Model: "sonnet", Success: true, CostUSD: 0.25, SpecID: "spec-A"},
		{BeadID: "b3", Model: "haiku", Success: true, CostUSD: 0.10, SpecID: "spec-B"},
	}
	writeTestLogFile(t, dir, "20260218-120000", logs)

	result, err := CostPerSpec(dir)
	if err != nil {
		t.Fatalf("CostPerSpec failed: %v", err)
	}
	if result["spec-A"].Iterations != 3 {
		t.Errorf("spec-A Iterations = %d, want 3", result["spec-A"].Iterations)
	}
	if result["spec-B"].Iterations != 1 {
		t.Errorf("spec-B Iterations = %d, want 1", result["spec-B"].Iterations)
	}
}

// TestCostPerSpec_GroupsBySpecIDAndAccumulatesCost verifies cost is summed per spec_id
func TestCostPerSpec_GroupsBySpecIDAndAccumulatesCost(t *testing.T) {
	dir := t.TempDir()

	logs := []IterationLog{
		{BeadID: "b1", Model: "opus", Success: true, CostUSD: 0.50, SpecID: "spec-A"},
		{BeadID: "b2", Model: "sonnet", Success: false, CostUSD: 0.25, SpecID: "spec-A"},
		{BeadID: "b3", Model: "haiku", Success: true, CostUSD: 0.10, SpecID: "spec-B"},
	}
	writeTestLogFile(t, dir, "20260218-120000", logs)

	result, err := CostPerSpec(dir)
	if err != nil {
		t.Fatalf("CostPerSpec failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("Expected 2 specs, got %d", len(result))
	}
	if diff := result["spec-A"].TotalCostUSD - 0.75; diff > 0.001 || diff < -0.001 {
		t.Errorf("spec-A TotalCostUSD = %.2f, want 0.75", result["spec-A"].TotalCostUSD)
	}
	if diff := result["spec-B"].TotalCostUSD - 0.10; diff > 0.001 || diff < -0.001 {
		t.Errorf("spec-B TotalCostUSD = %.2f, want 0.10", result["spec-B"].TotalCostUSD)
	}
}

// TestCostPerSpec_EmptySpecIDMapsToUnassigned verifies entries with empty spec_id are grouped under "(unassigned)"
func TestCostPerSpec_EmptySpecIDMapsToUnassigned(t *testing.T) {
	dir := t.TempDir()

	logs := []IterationLog{
		{BeadID: "b1", Model: "opus", Success: true, CostUSD: 0.50, SpecID: ""},
	}
	writeTestLogFile(t, dir, "20260218-120000", logs)

	result, err := CostPerSpec(dir)
	if err != nil {
		t.Fatalf("CostPerSpec failed: %v", err)
	}
	if _, ok := result["(unassigned)"]; !ok {
		t.Errorf("Expected '(unassigned)' key for empty spec_id, got keys: %v", result)
	}
	if _, ok := result[""]; ok {
		t.Error("Expected no empty-string key in result")
	}
	if _, ok := result["unassigned"]; ok {
		t.Error("Expected no 'unassigned' key (should be '(unassigned)')")
	}
}

// TestCostPerSpec_EmptyDir verifies CostPerSpec returns empty map for empty directory
func TestCostPerSpec_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := CostPerSpec(dir)
	if err != nil {
		t.Fatalf("CostPerSpec failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 specs, got %d", len(result))
	}
}

// TestModelStats_JSONTagsAreSnakeCase verifies ModelStats serializes with snake_case field names
func TestModelStats_JSONTagsAreSnakeCase(t *testing.T) {
	ms := ModelStats{
		Model:           "opus",
		Iterations:      10,
		Successes:       8,
		Failures:        2,
		EscalationsTo:   3,
		EscalationsFrom: 1,
		TotalCostUSD:    12.50,
	}

	data, err := json.Marshal(ms)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	for _, key := range []string{"model", "iterations", "successes", "failures", "escalations_to", "escalations_from", "total_cost_usd"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q not found in output; got keys: %v", key, keysOf(raw))
		}
	}
}

func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestSpecCost_JSONTagsAreSnakeCase verifies SpecCost serializes with snake_case field names
func TestSpecCost_JSONTagsAreSnakeCase(t *testing.T) {
	sc := SpecCost{
		TotalCostUSD: 1.50,
		Iterations:   5,
		Beads:        3,
		ModelMix:     map[string]int{"opus": 2},
	}

	data, err := json.Marshal(sc)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	for _, key := range []string{"total_cost_usd", "iterations", "beads", "model_mix"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q not found in output; got keys: %v", key, keysOf(raw))
		}
	}
}

// TestSpecCost_Struct verifies the SpecCost struct fields are accessible
func TestSpecCost_Struct(t *testing.T) {
	sc := SpecCost{
		TotalCostUSD: 1.50,
		Iterations:   5,
		Beads:        3,
		ModelMix:     map[string]int{"opus": 2, "sonnet": 3},
	}

	if sc.TotalCostUSD != 1.50 {
		t.Errorf("TotalCostUSD = %.2f, want 1.50", sc.TotalCostUSD)
	}
	if sc.Iterations != 5 {
		t.Errorf("Iterations = %d, want 5", sc.Iterations)
	}
	if sc.Beads != 3 {
		t.Errorf("Beads = %d, want 3", sc.Beads)
	}
	if sc.ModelMix["opus"] != 2 {
		t.Errorf("ModelMix[opus] = %d, want 2", sc.ModelMix["opus"])
	}
	if sc.ModelMix["sonnet"] != 3 {
		t.Errorf("ModelMix[sonnet] = %d, want 3", sc.ModelMix["sonnet"])
	}
}

// Expected failure: CostPerCompletedBead function does not exist yet
func TestCostPerCompletedBead_NoCompletedBeads(t *testing.T) {
	dir := t.TempDir()
	runID := "20260211-120000"

	// All beads fail
	logs := []IterationLog{
		{
			BeadID:     "b1",
			Model:      "sonnet",
			Success:    false,
			CostUSD:    0.25,
			DurationMs: 20000,
		},
		{
			BeadID:     "b2",
			Model:      "opus",
			Success:    false,
			CostUSD:    0.50,
			DurationMs: 30000,
		},
	}
	writeTestLogFile(t, dir, runID, logs)

	costs, err := CostPerCompletedBead(dir)
	if err != nil {
		t.Fatalf("CostPerCompletedBead failed: %v", err)
	}

	if len(costs) != 0 {
		t.Errorf("Expected 0 completed beads, got %d", len(costs))
	}
}
