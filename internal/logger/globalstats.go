package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GlobalStats represents aggregated statistics across all projects
type GlobalStats struct {
	Version int                          `json:"version"`
	Updated string                       `json:"updated"`
	Models  map[string]*GlobalModelStats `json:"models"`
}

// GlobalModelStats represents per-model statistics in the global stats file
type GlobalModelStats struct {
	Iterations      int     `json:"iterations"`
	Successes       int     `json:"successes"`
	Failures        int     `json:"failures"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
	EscalationsFrom int     `json:"escalations_from"`
	EscalationsTo   int     `json:"escalations_to"`
}

// ReadGlobalStats loads global stats from the given path.
// Returns a zero-value GlobalStats (empty Models map) if the file doesn't exist.
// Returns an error if the file exists but contains invalid JSON.
func ReadGlobalStats(path string) (*GlobalStats, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return zero-value stats
			return &GlobalStats{
				Version: 1,
				Updated: time.Now().UTC().Format(time.RFC3339),
				Models:  make(map[string]*GlobalModelStats),
			}, nil
		}
		// Some other error occurred
		return nil, fmt.Errorf("reading global stats file: %w", err)
	}

	var stats GlobalStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("parsing global stats JSON: %w", err)
	}

	return &stats, nil
}

// UpdateGlobalStats merges run ModelStats into global totals using atomic write.
// Handles missing file on first call by creating new stats.
func UpdateGlobalStats(path string, runStats map[string]ModelStats) error {
	// Read existing global stats (or get initialized empty stats if file doesn't exist)
	global, err := ReadGlobalStats(path)
	if err != nil {
		return fmt.Errorf("reading global stats: %w", err)
	}

	// Ensure Models map is initialized
	if global.Models == nil {
		global.Models = make(map[string]*GlobalModelStats)
	}

	// Merge run stats into global stats
	for modelName, runStat := range runStats {
		existing, exists := global.Models[modelName]
		if !exists {
			// Create new entry for this model
			existing = &GlobalModelStats{}
			global.Models[modelName] = existing
		}

		// Accumulate counts
		existing.Iterations += runStat.Iterations
		existing.Successes += runStat.Successes
		existing.Failures += runStat.Failures
		existing.TotalCostUSD += runStat.TotalCostUSD
		existing.EscalationsFrom += runStat.EscalationsFrom
		existing.EscalationsTo += runStat.EscalationsTo
	}

	// Update timestamp
	global.Updated = time.Now().UTC().Format(time.RFC3339)

	// Write atomically: write to temp file, then rename
	dir := filepath.Dir(path)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	tempFile, err := os.CreateTemp(dir, ".stats-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Ensure temp file cleanup on error (removed after successful rename)
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			tempFile.Close()
			os.Remove(tempPath)
		}
	}()

	// Marshal and write to temp file
	data, err := json.MarshalIndent(global, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling global stats: %w", err)
	}

	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}

	cleanupTemp = false
	return nil
}
