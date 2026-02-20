//go:build integration

package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
)

// Expected failure: Run() does not call logger.ReadRunModelStats after loop completes
// Expected failure: Run() does not call logger.UpdateGlobalStats with run stats
// Expected failure: Global stats file is not created at ~/.gromit/stats.json
func TestRun_UpdatesGlobalStatsAfterCompletion(t *testing.T) {
	// Setup temp directory for logs
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")

	// Setup temp home directory for global stats
	homeDir := t.TempDir()
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		goPath = filepath.Join(os.Getenv("HOME"), "go")
	}
	t.Setenv("HOME", homeDir)
	t.Setenv("GOPATH", goPath)

	expectedGlobalStatsPath := filepath.Join(homeDir, ".gromit", "stats.json")

	// Setup bead queue with one successful bead
	beadQueue := []*bead.Bead{
		{ID: "test-1", Title: "Test task", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}},
	}
	beadIdx := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if beadIdx >= len(beadQueue) {
				return nil, nil
			}
			b := beadQueue[beadIdx]
			beadIdx++
			return b, nil
		},
	}

	// Mock Claude to return success (no longer needed - using Router)

	var buf strings.Builder
	cfg := &config.Config{
		Claude: config.ClaudeConfig{BeadTimeout: 60},
		Paths: config.PathsConfig{
			Logs: logsDir,
		},
	}

	r, err := NewRunnerWithDeps(
		cfg,
		&buf, tmpDir,
		Deps{Beads: beads, Router: newMockRouter(), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Run the loop
	if err := r.Run(context.Background(), 0, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify global stats file was created
	if _, err := os.Stat(expectedGlobalStatsPath); os.IsNotExist(err) {
		t.Errorf("Expected global stats file at %s, but it does not exist", expectedGlobalStatsPath)
	}

	// Read and verify global stats content
	data, err := os.ReadFile(expectedGlobalStatsPath)
	if err != nil {
		t.Fatalf("Failed to read global stats file: %v", err)
	}

	var stats logger.GlobalStats
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatalf("Failed to parse global stats JSON: %v", err)
	}

	// Verify stats were updated
	if len(stats.Models) == 0 {
		t.Error("Expected global stats to contain model data, got empty map")
	}

	// The model used should have iteration count updated
	if stats.Version != 1 {
		t.Errorf("Expected Version=1, got %d", stats.Version)
	}

	if stats.Updated == "" {
		t.Error("Expected Updated timestamp to be set, got empty string")
	}
}

// Expected failure: Run() does not call logger.ReadRunModelStats with the current run ID
// Expected failure: Run() does not extract run ID from logger.RunID() method
func TestRun_UsesCurrentRunIDForModelStatsAggregation(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")

	homeDir := t.TempDir()
	goPath2 := os.Getenv("GOPATH")
	if goPath2 == "" {
		goPath2 = filepath.Join(os.Getenv("HOME"), "go")
	}
	t.Setenv("HOME", homeDir)
	t.Setenv("GOPATH", goPath2)

	expectedGlobalStatsPath := filepath.Join(homeDir, ".gromit", "stats.json")

	// Setup two runs worth of beads to create multiple log files
	// First, run one bead to create a log file
	beadQueue1 := []*bead.Bead{
		{ID: "old-bead", Title: "Old task", Priority: 0, Labels: []string{}, ExpectedOutputs: []string{}},
	}
	beadIdx1 := 0
	beads1 := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if beadIdx1 >= len(beadQueue1) {
				return nil, nil
			}
			b := beadQueue1[beadIdx1]
			beadIdx1++
			return b, nil
		},
	}

	var buf1 strings.Builder
	cfg := &config.Config{
		Claude: config.ClaudeConfig{BeadTimeout: 60},
		Paths: config.PathsConfig{
			Logs: logsDir,
		},
		Models: config.ModelsConfig{P0: "opus"},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
	}

	r1, err := NewRunnerWithDeps(cfg, &buf1, tmpDir,
		Deps{Beads: beads1, Router: newMockRouter(), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	if err := r1.Run(context.Background(), 0, time.Time{}, nil, false); err != nil {
		t.Fatalf("First run failed: %v", err)
	}

	// Logger run IDs are second-granularity. Preserve the first run's log under a
	// stable distinct filename so a same-second second run cannot overwrite it.
	firstRunLogs, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		t.Fatalf("glob first run logs: %v", err)
	}
	if len(firstRunLogs) != 1 {
		t.Fatalf("expected exactly one first-run log file, got %d", len(firstRunLogs))
	}
	archivedFirstRunPath := filepath.Join(logsDir, "run-20000101-000000.jsonl")
	if err := os.Rename(firstRunLogs[0], archivedFirstRunPath); err != nil {
		t.Fatalf("rename first run log %s -> %s: %v", firstRunLogs[0], archivedFirstRunPath, err)
	}

	// Now run a second bead - this should only aggregate stats from THIS run
	beadQueue2 := []*bead.Bead{
		{ID: "new-bead", Title: "New task", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}},
	}
	beadIdx2 := 0
	beads2 := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if beadIdx2 >= len(beadQueue2) {
				return nil, nil
			}
			b := beadQueue2[beadIdx2]
			beadIdx2++
			return b, nil
		},
	}

	var buf2 strings.Builder
	r2, err := NewRunnerWithDeps(cfg, &buf2, tmpDir,
		Deps{Beads: beads2, Router: newMockRouter(), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed for second run: %v", err)
	}

	if err := r2.Run(context.Background(), 0, time.Time{}, nil, false); err != nil {
		t.Fatalf("Second run failed: %v", err)
	}

	// Read global stats - should show cumulative data from both runs
	data, err := os.ReadFile(expectedGlobalStatsPath)
	if err != nil {
		t.Fatalf("Failed to read global stats: %v", err)
	}

	var stats logger.GlobalStats
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatalf("Failed to parse global stats: %v", err)
	}

	// Verify that stats include data from both runs
	// The key assertion: total iterations should reflect BOTH runs being aggregated
	var totalIterations int
	for _, modelStats := range stats.Models {
		totalIterations += modelStats.Iterations
	}

	if totalIterations < 2 {
		t.Errorf("Expected at least 2 iterations (one from each run), got %d - suggests Run() is not using current run ID", totalIterations)
	}
}

// Expected failure: Run() does not log warnings when global stats update fails
// This test verifies that UpdateGlobalStats failures are logged but don't halt the run
func TestRun_LogsWarningOnGlobalStatsUpdateFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")

	// Setup unwritable home directory to cause UpdateGlobalStats to fail
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	gromitDir := filepath.Join(homeDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0555); err != nil { // Read-only directory
		t.Fatalf("Failed to create read-only directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(gromitDir, 0755); err != nil {
			t.Fatalf("Failed to restore permissions: %v", err)
		}
	})

	// Setup bead queue with one successful bead
	beadQueue := []*bead.Bead{
		{ID: "test-1", Title: "Test task", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}},
	}
	beadIdx := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if beadIdx >= len(beadQueue) {
				return nil, nil
			}
			b := beadQueue[beadIdx]
			beadIdx++
			return b, nil
		},
	}

	var buf strings.Builder
	cfg := &config.Config{
		Claude: config.ClaudeConfig{BeadTimeout: 60},
		Paths: config.PathsConfig{
			Logs: logsDir,
		},
	}

	r, err := NewRunnerWithDeps(
		cfg,
		&buf, tmpDir,
		Deps{Beads: beads, Router: newMockRouter(), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Run the loop (should complete despite global stats update failure)
	if err := r.Run(context.Background(), 0, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() should not fail due to global stats update error: %v", err)
	}

	// Verify warning was logged about global stats failure
	output := buf.String()
	warningFound := strings.Contains(output, "Warning") && strings.Contains(output, "global stats")
	if !warningFound {
		t.Errorf("Expected warning message containing 'Warning' and 'global stats' in output.\nGot: %s", output)
	}
}

// Expected failure: Run() does not accumulate stats from multiple iterations
func TestRun_AccumulatesStatsFromMultipleIterations(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	expectedGlobalStatsPath := filepath.Join(homeDir, ".gromit", "stats.json")

	// Setup bead queue with multiple beads
	beadQueue := []*bead.Bead{
		{ID: "test-1", Title: "First task", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}},
		{ID: "test-2", Title: "Second task", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}},
		{ID: "test-3", Title: "Third task", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}},
	}
	beadIdx := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if beadIdx >= len(beadQueue) {
				return nil, nil
			}
			b := beadQueue[beadIdx]
			beadIdx++
			return b, nil
		},
	}

	var buf strings.Builder
	cfg := &config.Config{
		Claude: config.ClaudeConfig{BeadTimeout: 60},
		Paths: config.PathsConfig{
			Logs: logsDir,
		},
	}

	r, err := NewRunnerWithDeps(
		cfg,
		&buf, tmpDir,
		Deps{Beads: beads, Router: newMockRouter(), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Run the loop
	if err := r.Run(context.Background(), 0, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Read and verify global stats
	data, err := os.ReadFile(expectedGlobalStatsPath)
	if err != nil {
		t.Fatalf("Failed to read global stats file: %v", err)
	}

	var stats logger.GlobalStats
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatalf("Failed to parse global stats JSON: %v", err)
	}

	// The model used should have accumulated stats from all 3 iterations
	if len(stats.Models) == 0 {
		t.Fatal("Expected model stats, got empty map")
	}

	// Verify iteration count reflects all processed beads
	var totalIterations int
	for _, modelStats := range stats.Models {
		totalIterations += modelStats.Iterations
	}

	if totalIterations < 3 {
		t.Errorf("Expected at least 3 total iterations across models, got %d", totalIterations)
	}
}

// Expected failure: Run() does not resolve global stats path using os.UserHomeDir()
func TestRun_UsesUserHomeDirForGlobalStatsPath(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Setup bead queue
	beadQueue := []*bead.Bead{
		{ID: "test-1", Title: "Test task", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}},
	}
	beadIdx := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if beadIdx >= len(beadQueue) {
				return nil, nil
			}
			b := beadQueue[beadIdx]
			beadIdx++
			return b, nil
		},
	}

	var buf strings.Builder
	cfg := &config.Config{
		Claude: config.ClaudeConfig{BeadTimeout: 60},
		Paths: config.PathsConfig{
			Logs: logsDir,
		},
	}

	r, err := NewRunnerWithDeps(
		cfg,
		&buf, tmpDir,
		Deps{Beads: beads, Router: newMockRouter(), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	if err := r.Run(context.Background(), 0, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify the stats file was created under the HOME directory we set
	expectedPath := filepath.Join(homeDir, ".gromit", "stats.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected global stats at %s (resolved from HOME=%s), but not found", expectedPath, homeDir)
	}

	// Verify it was NOT created in the project directory
	projectStatsPath := filepath.Join(tmpDir, ".gromit", "stats.json")
	if _, err := os.Stat(projectStatsPath); !os.IsNotExist(err) {
		t.Errorf("Global stats should NOT be created in project directory at %s", projectStatsPath)
	}
}
