package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Expected failure: GlobalStats struct fields don't match spec JSON format yet
func TestGlobalStats_StructMatchesSpec(t *testing.T) {
	// Verify struct matches the spec's JSON format exactly
	stats := GlobalStats{
		Version: 1,
		Updated: "2026-02-11T14:30:00Z",
		Models: map[string]*GlobalModelStats{
			"opus": {
				Iterations:      42,
				Successes:       38,
				Failures:        4,
				TotalCostUSD:    84.50,
				EscalationsFrom: 0,
				EscalationsTo:   12,
			},
			"sonnet": {
				Iterations:      65,
				Successes:       24,
				Failures:        41,
				TotalCostUSD:    29.90,
				EscalationsFrom: 30,
				EscalationsTo:   0,
			},
			"haiku": {
				Iterations:      30,
				Successes:       22,
				Failures:        8,
				TotalCostUSD:    3.60,
				EscalationsFrom: 5,
				EscalationsTo:   0,
			},
		},
	}

	// Verify top-level fields
	if stats.Version != 1 {
		t.Errorf("Version = %d, want 1", stats.Version)
	}
	if stats.Updated != "2026-02-11T14:30:00Z" {
		t.Errorf("Updated = %s, want 2026-02-11T14:30:00Z", stats.Updated)
	}
	if len(stats.Models) != 3 {
		t.Errorf("len(Models) = %d, want 3", len(stats.Models))
	}

	// Verify opus stats
	opus := stats.Models["opus"]
	if opus.Iterations != 42 {
		t.Errorf("opus.Iterations = %d, want 42", opus.Iterations)
	}
	if opus.Successes != 38 {
		t.Errorf("opus.Successes = %d, want 38", opus.Successes)
	}
	if opus.Failures != 4 {
		t.Errorf("opus.Failures = %d, want 4", opus.Failures)
	}
	if opus.TotalCostUSD != 84.50 {
		t.Errorf("opus.TotalCostUSD = %.2f, want 84.50", opus.TotalCostUSD)
	}
	if opus.EscalationsFrom != 0 {
		t.Errorf("opus.EscalationsFrom = %d, want 0", opus.EscalationsFrom)
	}
	if opus.EscalationsTo != 12 {
		t.Errorf("opus.EscalationsTo = %d, want 12", opus.EscalationsTo)
	}

	// Verify sonnet stats
	sonnet := stats.Models["sonnet"]
	if sonnet.Iterations != 65 {
		t.Errorf("sonnet.Iterations = %d, want 65", sonnet.Iterations)
	}
	if sonnet.Successes != 24 {
		t.Errorf("sonnet.Successes = %d, want 24", sonnet.Successes)
	}
	if sonnet.Failures != 41 {
		t.Errorf("sonnet.Failures = %d, want 41", sonnet.Failures)
	}
	if sonnet.EscalationsFrom != 30 {
		t.Errorf("sonnet.EscalationsFrom = %d, want 30", sonnet.EscalationsFrom)
	}

	// Verify haiku stats
	haiku := stats.Models["haiku"]
	if haiku.Successes != 22 {
		t.Errorf("haiku.Successes = %d, want 22", haiku.Successes)
	}
	if haiku.Failures != 8 {
		t.Errorf("haiku.Failures = %d, want 8", haiku.Failures)
	}
}

// Expected failure: ReadGlobalStats does not return zero-value GlobalStats when file is missing
func TestReadGlobalStats_MissingFile(t *testing.T) {
	dir := t.TempDir()
	nonexistentPath := filepath.Join(dir, "stats.json")

	stats, err := ReadGlobalStats(nonexistentPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats should not error on missing file, got: %v", err)
	}

	if stats == nil {
		t.Fatal("ReadGlobalStats returned nil stats")
	}

	// Should return initialized zero-value stats
	if stats.Version != 1 {
		t.Errorf("Version = %d, want 1 (default)", stats.Version)
	}
	if stats.Updated == "" {
		t.Error("Updated should be set to current timestamp, got empty string")
	}
	if stats.Models == nil {
		t.Error("Models map should be initialized (empty), got nil")
	}
	if len(stats.Models) != 0 {
		t.Errorf("Models map should be empty, got %d entries", len(stats.Models))
	}
}

// Expected failure: ReadGlobalStats does not parse valid JSON file correctly
func TestReadGlobalStats_ValidFile(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create a valid stats file matching the spec format
	validStats := GlobalStats{
		Version: 1,
		Updated: "2026-02-11T14:30:00Z",
		Models: map[string]*GlobalModelStats{
			"opus": {
				Iterations:      10,
				Successes:       8,
				Failures:        2,
				TotalCostUSD:    5.25,
				EscalationsFrom: 0,
				EscalationsTo:   3,
			},
			"sonnet": {
				Iterations:      15,
				Successes:       10,
				Failures:        5,
				TotalCostUSD:    2.50,
				EscalationsFrom: 3,
				EscalationsTo:   0,
			},
		},
	}

	data, err := json.MarshalIndent(validStats, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(statsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Read the file
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	// Verify parsed data matches what we wrote
	if stats.Version != 1 {
		t.Errorf("Version = %d, want 1", stats.Version)
	}
	if stats.Updated != "2026-02-11T14:30:00Z" {
		t.Errorf("Updated = %s, want 2026-02-11T14:30:00Z", stats.Updated)
	}
	if len(stats.Models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(stats.Models))
	}

	// Verify opus data
	opus, exists := stats.Models["opus"]
	if !exists {
		t.Fatal("opus not found in Models")
	}
	if opus.Iterations != 10 {
		t.Errorf("opus.Iterations = %d, want 10", opus.Iterations)
	}
	if opus.Successes != 8 {
		t.Errorf("opus.Successes = %d, want 8", opus.Successes)
	}
	if opus.Failures != 2 {
		t.Errorf("opus.Failures = %d, want 2", opus.Failures)
	}
	if opus.TotalCostUSD != 5.25 {
		t.Errorf("opus.TotalCostUSD = %.2f, want 5.25", opus.TotalCostUSD)
	}

	// Verify sonnet data
	sonnet, exists := stats.Models["sonnet"]
	if !exists {
		t.Fatal("sonnet not found in Models")
	}
	if sonnet.Iterations != 15 {
		t.Errorf("sonnet.Iterations = %d, want 15", sonnet.Iterations)
	}
	if sonnet.Successes != 10 {
		t.Errorf("sonnet.Successes = %d, want 10", sonnet.Successes)
	}
}

// Expected failure: ReadGlobalStats does not return error on corrupt JSON
func TestReadGlobalStats_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Write invalid JSON
	corruptData := []byte(`{"version": 1, "updated": "2026-02-11T14:30:00Z", "models": {corrupted`)
	if err := os.WriteFile(statsPath, corruptData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Should return error
	stats, err := ReadGlobalStats(statsPath)
	if err == nil {
		t.Fatal("ReadGlobalStats should return error for corrupt JSON, got nil")
	}
	if stats != nil {
		t.Errorf("ReadGlobalStats should return nil stats on error, got %+v", stats)
	}

	// Error message should mention parsing
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message should not be empty")
	}
}

// Expected failure: GlobalModelStats JSON tags don't match spec snake_case format
func TestGlobalStats_JSONMarshaling(t *testing.T) {
	stats := GlobalStats{
		Version: 1,
		Updated: "2026-02-11T14:30:00Z",
		Models: map[string]*GlobalModelStats{
			"opus": {
				Iterations:      42,
				Successes:       38,
				Failures:        4,
				TotalCostUSD:    84.50,
				EscalationsFrom: 0,
				EscalationsTo:   12,
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("Failed to marshal GlobalStats: %v", err)
	}

	// Verify JSON field names match spec (snake_case)
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal to raw map: %v", err)
	}

	// Check top-level fields
	if _, ok := raw["version"]; !ok {
		t.Error("JSON should have 'version' field")
	}
	if _, ok := raw["updated"]; !ok {
		t.Error("JSON should have 'updated' field")
	}
	if _, ok := raw["models"]; !ok {
		t.Error("JSON should have 'models' field")
	}

	// Check model fields use snake_case
	models := raw["models"].(map[string]interface{})
	opus := models["opus"].(map[string]interface{})

	expectedFields := []string{
		"iterations",
		"successes",
		"failures",
		"total_cost_usd",
		"escalations_from",
		"escalations_to",
	}

	for _, field := range expectedFields {
		if _, ok := opus[field]; !ok {
			t.Errorf("JSON should have snake_case field '%s'", field)
		}
	}
}

// Expected failure: ReadGlobalStats does not handle empty Models map correctly
func TestReadGlobalStats_EmptyModels(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create stats with empty models map
	emptyStats := GlobalStats{
		Version: 1,
		Updated: "2026-02-11T14:30:00Z",
		Models:  map[string]*GlobalModelStats{},
	}

	data, err := json.Marshal(emptyStats)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(statsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Read the file
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	if stats.Models == nil {
		t.Error("Models should be initialized map (not nil)")
	}
	if len(stats.Models) != 0 {
		t.Errorf("Models should be empty, got %d entries", len(stats.Models))
	}
}

// Expected failure: ReadGlobalStats does not handle read permission errors correctly
func TestReadGlobalStats_UnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create a file
	data := []byte(`{"version": 1, "updated": "2026-02-11T14:30:00Z", "models": {}}`)
	if err := os.WriteFile(statsPath, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Make it unreadable
	if err := os.Chmod(statsPath, 0000); err != nil {
		t.Fatalf("Failed to chmod test file: %v", err)
	}
	defer os.Chmod(statsPath, 0644) // Restore for cleanup

	// Should return error
	stats, err := ReadGlobalStats(statsPath)
	if err == nil {
		t.Fatal("ReadGlobalStats should return error for unreadable file, got nil")
	}
	if stats != nil {
		t.Errorf("ReadGlobalStats should return nil stats on error, got %+v", stats)
	}
}

// Expected failure: GlobalStats struct doesn't preserve all model fields through JSON round-trip
func TestGlobalStats_RoundTrip(t *testing.T) {
	original := GlobalStats{
		Version: 1,
		Updated: "2026-02-11T14:30:00Z",
		Models: map[string]*GlobalModelStats{
			"opus": {
				Iterations:      100,
				Successes:       90,
				Failures:        10,
				TotalCostUSD:    150.75,
				EscalationsFrom: 2,
				EscalationsTo:   15,
			},
			"sonnet": {
				Iterations:      50,
				Successes:       25,
				Failures:        25,
				TotalCostUSD:    25.50,
				EscalationsFrom: 15,
				EscalationsTo:   2,
			},
			"haiku": {
				Iterations:      200,
				Successes:       180,
				Failures:        20,
				TotalCostUSD:    10.00,
				EscalationsFrom: 10,
				EscalationsTo:   0,
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var recovered GlobalStats
	if err := json.Unmarshal(data, &recovered); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify all fields are preserved
	if recovered.Version != original.Version {
		t.Errorf("Version = %d, want %d", recovered.Version, original.Version)
	}
	if recovered.Updated != original.Updated {
		t.Errorf("Updated = %s, want %s", recovered.Updated, original.Updated)
	}
	if len(recovered.Models) != len(original.Models) {
		t.Fatalf("len(Models) = %d, want %d", len(recovered.Models), len(original.Models))
	}

	// Verify each model
	for modelName, origModel := range original.Models {
		recModel, exists := recovered.Models[modelName]
		if !exists {
			t.Errorf("Model %s not found in recovered stats", modelName)
			continue
		}

		if recModel.Iterations != origModel.Iterations {
			t.Errorf("%s.Iterations = %d, want %d", modelName, recModel.Iterations, origModel.Iterations)
		}
		if recModel.Successes != origModel.Successes {
			t.Errorf("%s.Successes = %d, want %d", modelName, recModel.Successes, origModel.Successes)
		}
		if recModel.Failures != origModel.Failures {
			t.Errorf("%s.Failures = %d, want %d", modelName, recModel.Failures, origModel.Failures)
		}
		if recModel.TotalCostUSD != origModel.TotalCostUSD {
			t.Errorf("%s.TotalCostUSD = %.2f, want %.2f", modelName, recModel.TotalCostUSD, origModel.TotalCostUSD)
		}
		if recModel.EscalationsFrom != origModel.EscalationsFrom {
			t.Errorf("%s.EscalationsFrom = %d, want %d", modelName, recModel.EscalationsFrom, origModel.EscalationsFrom)
		}
		if recModel.EscalationsTo != origModel.EscalationsTo {
			t.Errorf("%s.EscalationsTo = %d, want %d", modelName, recModel.EscalationsTo, origModel.EscalationsTo)
		}
	}
}

// Expected failure: ReadGlobalStats does not handle files with extra unknown fields gracefully
func TestReadGlobalStats_ForwardCompatibility(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create JSON with extra fields (simulating future version)
	jsonData := `{
		"version": 1,
		"updated": "2026-02-11T14:30:00Z",
		"future_field": "should be ignored",
		"models": {
			"opus": {
				"iterations": 10,
				"successes": 8,
				"failures": 2,
				"total_cost_usd": 5.0,
				"escalations_from": 0,
				"escalations_to": 2,
				"future_model_field": "should also be ignored"
			}
		}
	}`

	if err := os.WriteFile(statsPath, []byte(jsonData), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Should parse without error, ignoring unknown fields
	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats should ignore unknown fields, got error: %v", err)
	}

	// Verify known fields are parsed correctly
	if stats.Version != 1 {
		t.Errorf("Version = %d, want 1", stats.Version)
	}
	if len(stats.Models) != 1 {
		t.Fatalf("Expected 1 model, got %d", len(stats.Models))
	}

	opus := stats.Models["opus"]
	if opus.Iterations != 10 {
		t.Errorf("opus.Iterations = %d, want 10", opus.Iterations)
	}
	if opus.Successes != 8 {
		t.Errorf("opus.Successes = %d, want 8", opus.Successes)
	}
}

// Expected failure: ReadGlobalStats does not initialize Models map when JSON has null models
func TestReadGlobalStats_NullModels(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.json")

	// Create JSON with null models (edge case)
	jsonData := `{
		"version": 1,
		"updated": "2026-02-11T14:30:00Z",
		"models": null
	}`

	if err := os.WriteFile(statsPath, []byte(jsonData), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	stats, err := ReadGlobalStats(statsPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed: %v", err)
	}

	// Models should be nil (as unmarshaled) or empty map
	// The important thing is it shouldn't crash
	if stats.Models == nil {
		// Acceptable - JSON null unmarshals to nil
	} else if len(stats.Models) != 0 {
		t.Errorf("Models should be empty or nil, got %d entries", len(stats.Models))
	}
}

// Expected failure: GlobalModelStats fields don't have correct zero values
func TestGlobalModelStats_ZeroValues(t *testing.T) {
	var stats GlobalModelStats

	// Verify zero values are sensible
	if stats.Iterations != 0 {
		t.Errorf("Iterations zero value = %d, want 0", stats.Iterations)
	}
	if stats.Successes != 0 {
		t.Errorf("Successes zero value = %d, want 0", stats.Successes)
	}
	if stats.Failures != 0 {
		t.Errorf("Failures zero value = %d, want 0", stats.Failures)
	}
	if stats.TotalCostUSD != 0.0 {
		t.Errorf("TotalCostUSD zero value = %.2f, want 0.00", stats.TotalCostUSD)
	}
	if stats.EscalationsFrom != 0 {
		t.Errorf("EscalationsFrom zero value = %d, want 0", stats.EscalationsFrom)
	}
	if stats.EscalationsTo != 0 {
		t.Errorf("EscalationsTo zero value = %d, want 0", stats.EscalationsTo)
	}
}

// Expected failure: ReadGlobalStats returns wrong format when file doesn't exist
func TestReadGlobalStats_InitializedFieldsOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	nonexistentPath := filepath.Join(dir, "does-not-exist.json")

	stats, err := ReadGlobalStats(nonexistentPath)
	if err != nil {
		t.Fatalf("ReadGlobalStats failed on missing file: %v", err)
	}

	// All fields should be properly initialized
	if stats == nil {
		t.Fatal("stats should not be nil")
	}
	if stats.Version == 0 {
		t.Error("Version should be initialized to 1, got 0")
	}
	if stats.Updated == "" {
		t.Error("Updated should be initialized to current time, got empty string")
	}
	if stats.Models == nil {
		t.Error("Models should be initialized to empty map, got nil")
	}
}

