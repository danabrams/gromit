package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_FirstWrite(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create run stats to merge
	runStats := map[string]ModelStats{
		"opus": {
			Model:           "opus",
			Iterations:      5,
			Successes:       4,
			Failures:        1,
			EscalationsTo:   2,
			EscalationsFrom: 0,
			TotalCostUSD:    10.50,
		},
		"sonnet": {
			Model:           "sonnet",
			Iterations:      3,
			Successes:       1,
			Failures:        2,
			EscalationsTo:   0,
			EscalationsFrom: 2,
			TotalCostUSD:    1.50,
		},
	}

	// Update global stats (file doesn't exist yet)
	err := UpdateGlobalStats(statsPath, runStats)
	if err != nil {
		t.Fatalf("UpdateGlobalStats failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(statsPath); os.IsNotExist(err) {
		t.Fatal("Global stats file was not created")
	}

	// Read back and verify content
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	if len(stats.Models) != 2 {
		t.Fatalf("Expected 2 models in global stats, got %d", len(stats.Models))
	}

	// Verify opus stats match what we wrote
	opus := stats.Models["opus"]
	if opus.Iterations != 5 {
		t.Errorf("opus.Iterations = %d, want 5", opus.Iterations)
	}
	if opus.Successes != 4 {
		t.Errorf("opus.Successes = %d, want 4", opus.Successes)
	}
	if opus.Failures != 1 {
		t.Errorf("opus.Failures = %d, want 1", opus.Failures)
	}
	if opus.TotalCostUSD != 10.50 {
		t.Errorf("opus.TotalCostUSD = %.2f, want 10.50", opus.TotalCostUSD)
	}

	// Verify sonnet stats
	sonnet := stats.Models["sonnet"]
	if sonnet.Iterations != 3 {
		t.Errorf("sonnet.Iterations = %d, want 3", sonnet.Iterations)
	}
	if sonnet.EscalationsFrom != 2 {
		t.Errorf("sonnet.EscalationsFrom = %d, want 2", sonnet.EscalationsFrom)
	}
}

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_MergeWithExisting(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create initial global stats file
	initial := GlobalStats{
		Version: 1,
		Updated: "2026-02-10T12:00:00Z",
		Models: map[string]*GlobalModelStats{
			"opus": {
				Iterations:      10,
				Successes:       8,
				Failures:        2,
				TotalCostUSD:    20.00,
				EscalationsFrom: 0,
				EscalationsTo:   3,
			},
			"sonnet": {
				Iterations:      5,
				Successes:       2,
				Failures:        3,
				TotalCostUSD:    2.50,
				EscalationsFrom: 3,
				EscalationsTo:   0,
			},
		},
	}

	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(statsPath, data, 0644); err != nil {
		t.Fatalf("Failed to create initial stats file: %v", err)
	}

	// Create run stats to merge
	runStats := map[string]ModelStats{
		"opus": {
			Model:           "opus",
			Iterations:      3,
			Successes:       2,
			Failures:        1,
			EscalationsTo:   1,
			EscalationsFrom: 0,
			TotalCostUSD:    6.00,
		},
		"haiku": {
			Model:           "haiku",
			Iterations:      5,
			Successes:       4,
			Failures:        1,
			EscalationsTo:   0,
			EscalationsFrom: 1,
			TotalCostUSD:    0.50,
		},
	}

	// Merge run stats into global
	err := UpdateGlobalStats(statsPath, runStats)
	if err != nil {
		t.Fatalf("UpdateGlobalStats failed: %v", err)
	}

	// Read back and verify merged content
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	// Should have 3 models now (opus, sonnet, haiku)
	if len(stats.Models) != 3 {
		t.Fatalf("Expected 3 models in global stats, got %d", len(stats.Models))
	}

	// Verify opus stats are merged (10+3=13 iterations, 8+2=10 successes, etc.)
	opus := stats.Models["opus"]
	if opus.Iterations != 13 {
		t.Errorf("opus.Iterations = %d, want 13 (10+3)", opus.Iterations)
	}
	if opus.Successes != 10 {
		t.Errorf("opus.Successes = %d, want 10 (8+2)", opus.Successes)
	}
	if opus.Failures != 3 {
		t.Errorf("opus.Failures = %d, want 3 (2+1)", opus.Failures)
	}
	if opus.TotalCostUSD != 26.00 {
		t.Errorf("opus.TotalCostUSD = %.2f, want 26.00 (20.00+6.00)", opus.TotalCostUSD)
	}
	if opus.EscalationsTo != 4 {
		t.Errorf("opus.EscalationsTo = %d, want 4 (3+1)", opus.EscalationsTo)
	}

	// Verify sonnet stats unchanged (not in run stats)
	sonnet := stats.Models["sonnet"]
	if sonnet.Iterations != 5 {
		t.Errorf("sonnet.Iterations = %d, want 5 (unchanged)", sonnet.Iterations)
	}
	if sonnet.TotalCostUSD != 2.50 {
		t.Errorf("sonnet.TotalCostUSD = %.2f, want 2.50 (unchanged)", sonnet.TotalCostUSD)
	}

	// Verify haiku stats were added
	haiku := stats.Models["haiku"]
	if haiku.Iterations != 5 {
		t.Errorf("haiku.Iterations = %d, want 5", haiku.Iterations)
	}
	if haiku.Successes != 4 {
		t.Errorf("haiku.Successes = %d, want 4", haiku.Successes)
	}
	if haiku.TotalCostUSD != 0.50 {
		t.Errorf("haiku.TotalCostUSD = %.2f, want 0.50", haiku.TotalCostUSD)
	}
}

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_EmptyRunStats(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create initial stats
	initial := GlobalStats{
		Version: 1,
		Updated: "2026-02-10T12:00:00Z",
		Models: map[string]*GlobalModelStats{
			"opus": {
				Iterations:   5,
				Successes:    4,
				Failures:     1,
				TotalCostUSD: 10.00,
			},
		},
	}

	data, _ := json.Marshal(initial)
	if err := os.WriteFile(statsPath, data, 0644); err != nil {
		t.Fatalf("Failed to create initial stats file: %v", err)
	}

	// Update with empty run stats
	emptyRunStats := make(map[string]ModelStats)
	err := UpdateGlobalStats(statsPath, emptyRunStats)
	if err != nil {
		t.Fatalf("UpdateGlobalStats should handle empty run stats, got error: %v", err)
	}

	// Read back and verify nothing changed
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	opus := stats.Models["opus"]
	if opus.Iterations != 5 {
		t.Errorf("opus.Iterations = %d, want 5 (unchanged)", opus.Iterations)
	}
	if opus.TotalCostUSD != 10.00 {
		t.Errorf("opus.TotalCostUSD = %.2f, want 10.00 (unchanged)", opus.TotalCostUSD)
	}
}

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_UpdatesTimestamp(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create initial stats with old timestamp
	initial := GlobalStats{
		Version: 1,
		Updated: "2026-02-10T12:00:00Z",
		Models: map[string]*GlobalModelStats{
			"opus": {
				Iterations:   1,
				Successes:    1,
				Failures:     0,
				TotalCostUSD: 2.00,
			},
		},
	}

	data, _ := json.Marshal(initial)
	if err := os.WriteFile(statsPath, data, 0644); err != nil {
		t.Fatalf("Failed to create initial stats file: %v", err)
	}

	// Update stats
	runStats := map[string]ModelStats{
		"opus": {
			Model:        "opus",
			Iterations:   1,
			Successes:    1,
			TotalCostUSD: 2.00,
		},
	}

	err := UpdateGlobalStats(statsPath, runStats)
	if err != nil {
		t.Fatalf("UpdateGlobalStats failed: %v", err)
	}

	// Read back and verify timestamp was updated
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	if stats.Updated == "2026-02-10T12:00:00Z" {
		t.Error("Updated timestamp should have changed from initial value")
	}
	if stats.Updated == "" {
		t.Error("Updated timestamp should be set")
	}
}

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create initial valid stats
	initial := GlobalStats{
		Version: 1,
		Updated: "2026-02-10T12:00:00Z",
		Models: map[string]*GlobalModelStats{
			"opus": {
				Iterations:   5,
				Successes:    4,
				Failures:     1,
				TotalCostUSD: 10.00,
			},
		},
	}

	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(statsPath, data, 0644); err != nil {
		t.Fatalf("Failed to create initial stats file: %v", err)
	}

	// Update stats
	runStats := map[string]ModelStats{
		"sonnet": {
			Model:        "sonnet",
			Iterations:   3,
			Successes:    2,
			Failures:     1,
			TotalCostUSD: 1.50,
		},
	}

	err := UpdateGlobalStats(statsPath, runStats)
	if err != nil {
		t.Fatalf("UpdateGlobalStats failed: %v", err)
	}

	// Verify no temp files are left behind
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() != "stats.json" {
			t.Errorf("Found unexpected file %s (temp file not cleaned up?)", entry.Name())
		}
	}

	// Verify final file is valid JSON and complete
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed after atomic write: %v", err)
	}

	if len(stats.Models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(stats.Models))
	}
}

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_PreservesVersion(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create initial stats
	initial := GlobalStats{
		Version: 1,
		Updated: "2026-02-10T12:00:00Z",
		Models: map[string]*GlobalModelStats{
			"opus": {
				Iterations:   5,
				Successes:    4,
				Failures:     1,
				TotalCostUSD: 10.00,
			},
		},
	}

	data, _ := json.Marshal(initial)
	if err := os.WriteFile(statsPath, data, 0644); err != nil {
		t.Fatalf("Failed to create initial stats file: %v", err)
	}

	// Update stats
	runStats := map[string]ModelStats{
		"sonnet": {
			Model:        "sonnet",
			Iterations:   2,
			Successes:    1,
			Failures:     1,
			TotalCostUSD: 1.00,
		},
	}

	err := UpdateGlobalStats(statsPath, runStats)
	if err != nil {
		t.Fatalf("UpdateGlobalStats failed: %v", err)
	}

	// Read back and verify version is preserved
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	if stats.Version != 1 {
		t.Errorf("Version = %d, want 1 (should be preserved)", stats.Version)
	}
}

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_HandlesAllEscalationFields(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create initial stats with escalation data
	initial := GlobalStats{
		Version: 1,
		Updated: "2026-02-10T12:00:00Z",
		Models: map[string]*GlobalModelStats{
			"opus": {
				Iterations:      10,
				Successes:       9,
				Failures:        1,
				TotalCostUSD:    20.00,
				EscalationsFrom: 0,
				EscalationsTo:   5,
			},
			"sonnet": {
				Iterations:      8,
				Successes:       3,
				Failures:        5,
				TotalCostUSD:    4.00,
				EscalationsFrom: 5,
				EscalationsTo:   2,
			},
		},
	}

	data, _ := json.Marshal(initial)
	if err := os.WriteFile(statsPath, data, 0644); err != nil {
		t.Fatalf("Failed to create initial stats file: %v", err)
	}

	// Run stats with more escalation data
	runStats := map[string]ModelStats{
		"opus": {
			Model:           "opus",
			Iterations:      2,
			Successes:       2,
			Failures:        0,
			EscalationsTo:   3,
			EscalationsFrom: 0,
			TotalCostUSD:    4.00,
		},
		"sonnet": {
			Model:           "sonnet",
			Iterations:      4,
			Successes:       1,
			Failures:        3,
			EscalationsTo:   0,
			EscalationsFrom: 3,
			TotalCostUSD:    2.00,
		},
	}

	err := UpdateGlobalStats(statsPath, runStats)
	if err != nil {
		t.Fatalf("UpdateGlobalStats failed: %v", err)
	}

	// Verify escalation counts are properly merged
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	opus := stats.Models["opus"]
	if opus.EscalationsTo != 8 {
		t.Errorf("opus.EscalationsTo = %d, want 8 (5+3)", opus.EscalationsTo)
	}
	if opus.EscalationsFrom != 0 {
		t.Errorf("opus.EscalationsFrom = %d, want 0 (0+0)", opus.EscalationsFrom)
	}

	sonnet := stats.Models["sonnet"]
	if sonnet.EscalationsFrom != 8 {
		t.Errorf("sonnet.EscalationsFrom = %d, want 8 (5+3)", sonnet.EscalationsFrom)
	}
	if sonnet.EscalationsTo != 2 {
		t.Errorf("sonnet.EscalationsTo = %d, want 2 (2+0)", sonnet.EscalationsTo)
	}
}

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_CreatesMissingDirectory(t *testing.T) {
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "nested", "path")
	statsPath := filepath.Join(nestedDir, "stats.json")

	// Don't create the nested directory - UpdateGlobalStats should handle it
	runStats := map[string]ModelStats{
		"opus": {
			Model:        "opus",
			Iterations:   1,
			Successes:    1,
			TotalCostUSD: 2.00,
		},
	}

	err := UpdateGlobalStats(statsPath, runStats)
	if err != nil {
		t.Fatalf("UpdateGlobalStats should create missing directories, got error: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(statsPath); os.IsNotExist(err) {
		t.Error("UpdateGlobalStats did not create file in nested directory")
	}

	// Verify content is valid
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	if len(stats.Models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(stats.Models))
	}
}

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_CorruptedExistingFile(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create a corrupted stats file
	corruptData := []byte(`{"version": 1, "updated": "2026-02-11T12:00:00Z", "models": {corrupted`)
	if err := os.WriteFile(statsPath, corruptData, 0644); err != nil {
		t.Fatalf("Failed to create corrupt file: %v", err)
	}

	// Attempt to update - should return error on corrupted file
	runStats := map[string]ModelStats{
		"opus": {
			Model:        "opus",
			Iterations:   1,
			Successes:    1,
			TotalCostUSD: 2.00,
		},
	}

	err := UpdateGlobalStats(statsPath, runStats)
	if err == nil {
		t.Fatal("UpdateGlobalStats should return error when existing file is corrupted")
	}
}

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_NilModelsMap(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Pass nil instead of empty map
	err := UpdateGlobalStats(statsPath, nil)
	if err != nil {
		t.Fatalf("UpdateGlobalStats should handle nil runStats, got error: %v", err)
	}

	// When starting with missing file and nil stats, should create file with empty models
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	if stats.Models == nil {
		t.Error("Models map should be initialized, got nil")
	}
	if len(stats.Models) != 0 {
		t.Errorf("Expected 0 models with nil runStats, got %d", len(stats.Models))
	}
}

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_ZeroCostValues(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Run stats with zero cost (e.g., cached response)
	runStats := map[string]ModelStats{
		"opus": {
			Model:        "opus",
			Iterations:   2,
			Successes:    2,
			Failures:     0,
			TotalCostUSD: 0.00,
		},
	}

	err := UpdateGlobalStats(statsPath, runStats)
	if err != nil {
		t.Fatalf("UpdateGlobalStats failed: %v", err)
	}

	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	opus := stats.Models["opus"]
	if opus.TotalCostUSD != 0.00 {
		t.Errorf("opus.TotalCostUSD = %.2f, want 0.00", opus.TotalCostUSD)
	}
	if opus.Iterations != 2 {
		t.Errorf("opus.Iterations = %d, want 2 (zero cost doesn't mean zero iterations)", opus.Iterations)
	}
}

// Expected failure: UpdateGlobalStats function does not exist yet
func TestUpdateGlobalStats_MultipleModelsInOneCall(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Run that used multiple models
	runStats := map[string]ModelStats{
		"haiku": {
			Model:           "haiku",
			Iterations:      5,
			Successes:       3,
			Failures:        2,
			EscalationsFrom: 2,
			EscalationsTo:   0,
			TotalCostUSD:    0.50,
		},
		"sonnet": {
			Model:           "sonnet",
			Iterations:      2,
			Successes:       1,
			Failures:        1,
			EscalationsFrom: 1,
			EscalationsTo:   2,
			TotalCostUSD:    1.00,
		},
		"opus": {
			Model:           "opus",
			Iterations:      1,
			Successes:       1,
			Failures:        0,
			EscalationsFrom: 0,
			EscalationsTo:   1,
			TotalCostUSD:    2.50,
		},
	}

	err := UpdateGlobalStats(statsPath, runStats)
	if err != nil {
		t.Fatalf("UpdateGlobalStats failed: %v", err)
	}

	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	// All three models should be present
	if len(stats.Models) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(stats.Models))
	}

	// Verify each model
	if stats.Models["haiku"].Iterations != 5 {
		t.Errorf("haiku.Iterations = %d, want 5", stats.Models["haiku"].Iterations)
	}
	if stats.Models["sonnet"].Iterations != 2 {
		t.Errorf("sonnet.Iterations = %d, want 2", stats.Models["sonnet"].Iterations)
	}
	if stats.Models["opus"].Iterations != 1 {
		t.Errorf("opus.Iterations = %d, want 1", stats.Models["opus"].Iterations)
	}

	// Verify total cost across all models
	totalCost := stats.Models["haiku"].TotalCostUSD + stats.Models["sonnet"].TotalCostUSD + stats.Models["opus"].TotalCostUSD
	expectedTotal := 4.00
	if diff := totalCost - expectedTotal; diff > 0.001 || diff < -0.001 {
		t.Errorf("Total cost across models = %.2f, want %.2f", totalCost, expectedTotal)
	}
}
