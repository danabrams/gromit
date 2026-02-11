package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
)

// Expected failure: Runner.Status() does not call logger.ReadModelStats and format/display model performance yet
func TestRunner_Status_DisplaysModelPerformance(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	var buf strings.Builder
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(tmpDir, "specs"),
			Plans: filepath.Join(tmpDir, "plans"),
			Logs:  logsDir,
		},
	}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{
				ID:       "next-123",
				Title:    "Next Bead",
				Priority: 1,
				Labels:   []string{},
			}, nil
		},
	}

	r, err := NewRunnerWithDeps(cfg, &buf, tmpDir, Deps{
		Beads:    mockBeads,
		Claude:   &mockClaudeClient{},
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Create minimal supporting files
	if err := os.WriteFile(filepath.Join(tmpDir, "backlog.jsonl"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write JSONL log files with model stats
	runID := "20260211-120000"
	logs := []logger.IterationLog{
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			CostUSD:    2.04,
			DurationMs: 30000,
		},
		{
			BeadID:     "b2",
			Model:      "opus",
			Success:    true,
			CostUSD:    2.04,
			DurationMs: 32000,
		},
		{
			BeadID:     "b3",
			Model:      "sonnet",
			Success:    false,
			CostUSD:    0.46,
			DurationMs: 20000,
		},
		{
			BeadID:     "b4",
			Model:      "sonnet",
			Success:    true,
			CostUSD:    0.46,
			DurationMs: 22000,
		},
		{
			BeadID:     "b5",
			Model:      "haiku",
			Success:    true,
			CostUSD:    0.12,
			DurationMs: 5000,
		},
		{
			BeadID:     "b6",
			Model:      "haiku",
			Success:    true,
			CostUSD:    0.12,
			DurationMs: 5500,
		},
	}
	writeTestLogFile(t, logsDir, runID, logs)

	// Execute
	err = r.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	output := buf.String()

	// Verify Model Performance section is displayed
	if !strings.Contains(output, "Model Performance") {
		t.Errorf("Expected 'Model Performance' section in output, got:\n%s", output)
	}

	// Verify opus stats appear
	if !strings.Contains(output, "opus") {
		t.Errorf("Expected opus model in output, got:\n%s", output)
	}
	if !strings.Contains(output, "100%") { // 2/2 = 100%
		t.Errorf("Expected opus success rate (100%%) in output, got:\n%s", output)
	}

	// Verify sonnet stats appear
	if !strings.Contains(output, "sonnet") {
		t.Errorf("Expected sonnet model in output, got:\n%s", output)
	}
	if !strings.Contains(output, "50%") { // 1/2 = 50%
		t.Errorf("Expected sonnet success rate (50%%) in output, got:\n%s", output)
	}

	// Verify haiku stats appear
	if !strings.Contains(output, "haiku") {
		t.Errorf("Expected haiku model in output, got:\n%s", output)
	}
	if !strings.Contains(output, "100%") { // 2/2 = 100%
		t.Errorf("Expected haiku success rate (100%%) in output, got:\n%s", output)
	}
}

// Expected failure: Runner.Status() does not call logger.ReadModelStats yet
func TestRunner_Status_ModelPerformanceSection_EmptyLogs(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	var buf strings.Builder
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(tmpDir, "specs"),
			Plans: filepath.Join(tmpDir, "plans"),
			Logs:  logsDir,
		},
	}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return nil, nil
		},
	}

	r, err := NewRunnerWithDeps(cfg, &buf, tmpDir, Deps{
		Beads:    mockBeads,
		Claude:   &mockClaudeClient{},
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Create minimal supporting files
	if err := os.WriteFile(filepath.Join(tmpDir, "backlog.jsonl"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// No log files - empty logs directory

	// Execute
	err = r.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	output := buf.String()

	// Should still show Model Performance section with "(no data)" message
	if !strings.Contains(output, "Model Performance") {
		t.Errorf("Expected 'Model Performance' section even with no data, got:\n%s", output)
	}
}

// Expected failure: Runner.Status() does not call logger.ReadModelStats and does not handle errors gracefully yet
func TestRunner_Status_ModelPerformanceSection_ErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	// Create logs dir path but don't actually create the directory to trigger error
	logsDir := filepath.Join(tmpDir, "nonexistent-logs")

	var buf strings.Builder
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(tmpDir, "specs"),
			Plans: filepath.Join(tmpDir, "plans"),
			Logs:  logsDir,
		},
	}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return nil, nil
		},
	}

	r, err := NewRunnerWithDeps(cfg, &buf, tmpDir, Deps{
		Beads:    mockBeads,
		Claude:   &mockClaudeClient{},
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Create minimal supporting files
	if err := os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "plans"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "backlog.jsonl"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Execute - should handle error gracefully without crashing
	err = r.Status()
	if err != nil {
		t.Fatalf("Status() should handle ReadModelStats errors gracefully, got error: %v", err)
	}

	output := buf.String()

	// Should still display other sections
	if !strings.Contains(output, "Pipeline:") {
		t.Errorf("Expected Pipeline section even with model stats error, got:\n%s", output)
	}
	if !strings.Contains(output, "Health:") {
		t.Errorf("Expected Health section even with model stats error, got:\n%s", output)
	}
}

// Expected failure: Runner.Status() does not format model performance between Health and Recommendation sections yet
func TestRunner_Status_ModelPerformanceSection_Placement(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	var buf strings.Builder
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(tmpDir, "specs"),
			Plans: filepath.Join(tmpDir, "plans"),
			Logs:  logsDir,
		},
	}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{
				ID:       "test-123",
				Title:    "Test Bead",
				Priority: 1,
				Labels:   []string{},
			}, nil
		},
	}

	r, err := NewRunnerWithDeps(cfg, &buf, tmpDir, Deps{
		Beads:    mockBeads,
		Claude:   &mockClaudeClient{},
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Create minimal supporting files
	if err := os.WriteFile(filepath.Join(tmpDir, "backlog.jsonl"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write minimal log file
	runID := "20260211-120000"
	logs := []logger.IterationLog{
		{
			BeadID:     "b1",
			Model:      "opus",
			Success:    true,
			CostUSD:    2.04,
			DurationMs: 30000,
		},
	}
	writeTestLogFile(t, logsDir, runID, logs)

	// Execute
	err = r.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	output := buf.String()

	// Verify section ordering: Model Performance appears between Health and Recommendation
	healthPos := strings.Index(output, "Health:")
	modelPerfPos := strings.Index(output, "Model Performance")
	recommendationPos := strings.Index(output, "Next action:")

	if healthPos == -1 || modelPerfPos == -1 || recommendationPos == -1 {
		t.Fatalf("Missing expected sections in output:\nHealth at %d\nModel Performance at %d\nRecommendation at %d\n\nOutput:\n%s",
			healthPos, modelPerfPos, recommendationPos, output)
	}

	// Model Performance should appear after Health
	if modelPerfPos <= healthPos {
		t.Errorf("Model Performance section should appear after Health section, got positions: Health=%d, ModelPerf=%d\n\nOutput:\n%s",
			healthPos, modelPerfPos, output)
	}

	// Recommendation should appear after Model Performance
	if recommendationPos <= modelPerfPos {
		t.Errorf("Recommendation section should appear after Model Performance section, got positions: ModelPerf=%d, Recommendation=%d\n\nOutput:\n%s",
			modelPerfPos, recommendationPos, output)
	}
}

// Expected failure: Runner.Status() does not call logger.ReadModelStats with correct path yet
func TestRunner_Status_ModelPerformanceSection_UsesConfiguredLogsPath(t *testing.T) {
	tmpDir := t.TempDir()
	customLogsDir := filepath.Join(tmpDir, "custom-logs-location")
	if err := os.MkdirAll(customLogsDir, 0755); err != nil {
		t.Fatalf("Failed to create custom logs dir: %v", err)
	}

	var buf strings.Builder
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(tmpDir, "specs"),
			Plans: filepath.Join(tmpDir, "plans"),
			Logs:  customLogsDir, // Custom path, not the default
		},
	}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return nil, nil
		},
	}

	r, err := NewRunnerWithDeps(cfg, &buf, tmpDir, Deps{
		Beads:    mockBeads,
		Claude:   &mockClaudeClient{},
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Create minimal supporting files
	if err := os.WriteFile(filepath.Join(tmpDir, "backlog.jsonl"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "state.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write log file to CUSTOM logs directory
	runID := "20260211-120000"
	logs := []logger.IterationLog{
		{
			BeadID:     "b1",
			Model:      "sonnet",
			Success:    true,
			CostUSD:    0.46,
			DurationMs: 20000,
		},
	}
	writeTestLogFile(t, customLogsDir, runID, logs)

	// Execute
	err = r.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	output := buf.String()

	// Should find and display stats from the custom logs directory
	if !strings.Contains(output, "Model Performance") {
		t.Errorf("Expected Model Performance section, got:\n%s", output)
	}
	if !strings.Contains(output, "sonnet") {
		t.Errorf("Expected sonnet model from custom logs dir, got:\n%s", output)
	}
}

// writeTestLogFile is a helper that writes JSONL log entries to a file
func writeTestLogFile(t *testing.T, dir string, runID string, logs []logger.IterationLog) {
	t.Helper()

	filename := filepath.Join(dir, "run-"+runID+".jsonl")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, log := range logs {
		if err := encoder.Encode(log); err != nil {
			t.Fatalf("Failed to write test log entry: %v", err)
		}
	}
}
