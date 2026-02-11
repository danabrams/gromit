//go:build acceptance

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/logger"
)

// setupAnalyzeTimeoutsTest creates a test environment with config and optional log data
func setupAnalyzeTimeoutsTest(t *testing.T, withLogs bool) (string, string) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	logsDir := filepath.Join(gromitDir, "logs")

	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	// Create gromit.yaml
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `models:
  opus: claude-opus-4
  sonnet: claude-sonnet-3-5
  haiku: claude-haiku-3-5

paths:
  gromit_dir: .gromit

loop:
  max_iterations: 0
  validation: true
  skip_on_validation_failure: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	if withLogs {
		// Create sample log file with timeout data
		logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":120000,"timeout_type":"stall","time_to_first_event_ms":5000,"tool_call_count":3,"stall_count":1,"stall_tier":"initial","rate_limit_hits":0,"error":"stall timeout"}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":60000,"time_to_first_event_ms":2000,"tool_call_count":5}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"opus","success":false,"validated":false,"escalated":false,"duration_ms":600000,"timeout_type":"invocation","time_to_first_event_ms":10000,"tool_call_count":8,"rate_limit_hits":2,"error":"invocation timeout"}
`
		logPath := filepath.Join(logsDir, "run-20260205-120000.jsonl")
		if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
			t.Fatalf("failed to write log: %v", err)
		}
	}

	return tmpDir, configPath
}

func TestAnalyzeTimeoutsCommand_HumanReadableOutput(t *testing.T) {
	// Expected failure: analyzeTimeoutsCmd does not exist yet
	tmpDir, configPath := setupAnalyzeTimeoutsTest(t, true)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--config", configPath, "analyze-timeouts"})

	// Expected failure: command does not exist
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("analyze-timeouts command failed: %v", err)
	}

	output := buf.String()

	// Verify human-readable output format
	if !strings.Contains(output, "Timeout Analysis") {
		t.Errorf("expected output to contain 'Timeout Analysis' header, got: %s", output)
	}

	// Should show total statistics
	if !strings.Contains(output, "Total Iterations:") {
		t.Errorf("expected output to contain 'Total Iterations:', got: %s", output)
	}
	if !strings.Contains(output, "Total Timeouts:") {
		t.Errorf("expected output to contain 'Total Timeouts:', got: %s", output)
	}

	// Should show per-model breakdown
	if !strings.Contains(output, "sonnet") {
		t.Errorf("expected output to contain model name 'sonnet', got: %s", output)
	}
	if !strings.Contains(output, "opus") {
		t.Errorf("expected output to contain model name 'opus', got: %s", output)
	}

	// Should show timeout type breakdown
	if !strings.Contains(output, "stall") {
		t.Errorf("expected output to contain 'stall' timeout type, got: %s", output)
	}
	if !strings.Contains(output, "invocation") {
		t.Errorf("expected output to contain 'invocation' timeout type, got: %s", output)
	}

	// Should show diagnostic metrics
	if !strings.Contains(output, "Avg Time to First Event") {
		t.Errorf("expected output to contain 'Avg Time to First Event', got: %s", output)
	}
	if !strings.Contains(output, "Avg Tool Calls") {
		t.Errorf("expected output to contain 'Avg Tool Calls', got: %s", output)
	}
}

func TestAnalyzeTimeoutsCommand_JSONOutput(t *testing.T) {
	// Expected failure: analyzeTimeoutsCmd does not exist yet, and --json flag does not exist
	tmpDir, configPath := setupAnalyzeTimeoutsTest(t, true)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--config", configPath, "analyze-timeouts", "--json"})

	// Expected failure: command and --json flag do not exist
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("analyze-timeouts --json command failed: %v", err)
	}

	output := buf.String()

	// Parse JSON output
	var analysis logger.TimeoutAnalysis
	if err := json.Unmarshal([]byte(output), &analysis); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, output)
	}

	// Verify structure matches TimeoutAnalysis
	if analysis.TotalIterations != 3 {
		t.Errorf("expected TotalIterations=3, got %d", analysis.TotalIterations)
	}
	if analysis.TotalTimeouts != 2 {
		t.Errorf("expected TotalTimeouts=2, got %d", analysis.TotalTimeouts)
	}

	// Verify per-model data
	sonnetStats, hasSonnet := analysis.ByModel["sonnet"]
	if !hasSonnet {
		t.Errorf("expected ByModel to contain 'sonnet' key")
	} else {
		if sonnetStats.TotalIterations != 2 {
			t.Errorf("sonnet: expected TotalIterations=2, got %d", sonnetStats.TotalIterations)
		}
		if sonnetStats.TimeoutCount != 1 {
			t.Errorf("sonnet: expected TimeoutCount=1, got %d", sonnetStats.TimeoutCount)
		}
	}

	opusStats, hasOpus := analysis.ByModel["opus"]
	if !hasOpus {
		t.Errorf("expected ByModel to contain 'opus' key")
	} else {
		if opusStats.TotalIterations != 1 {
			t.Errorf("opus: expected TotalIterations=1, got %d", opusStats.TotalIterations)
		}
		if opusStats.TimeoutCount != 1 {
			t.Errorf("opus: expected TimeoutCount=1, got %d", opusStats.TimeoutCount)
		}
	}
}

func TestAnalyzeTimeoutsCommand_EmptyLogsDirectory(t *testing.T) {
	// Expected failure: analyzeTimeoutsCmd does not exist yet
	tmpDir, configPath := setupAnalyzeTimeoutsTest(t, false)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--config", configPath, "analyze-timeouts"})

	// Expected failure: command does not exist
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("analyze-timeouts command failed: %v", err)
	}

	output := buf.String()

	// Should show informative message about no data
	if !strings.Contains(output, "No timeout data found") && !strings.Contains(output, "0 iterations") {
		t.Errorf("expected informative message about missing data, got: %s", output)
	}
}

func TestAnalyzeTimeoutsCommand_MissingConfig(t *testing.T) {
	// Expected failure: analyzeTimeoutsCmd does not exist yet
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"analyze-timeouts"})

	// Expected failure: command does not exist
	err = rootCmd.Execute()

	// Should return error about missing config
	if err == nil {
		t.Error("expected error for missing config, got nil")
	}

	// Error message should be informative
	errMsg := err.Error()
	if !strings.Contains(errMsg, "gromit.yaml") && !strings.Contains(errMsg, "config") {
		t.Errorf("expected error message to mention config file, got: %s", errMsg)
	}
}

func TestAnalyzeTimeoutsCommand_MissingLogsDirectory(t *testing.T) {
	// Expected failure: analyzeTimeoutsCmd does not exist yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")

	// Create gromit dir but NOT logs dir
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `models:
  opus: claude-opus-4
  sonnet: claude-sonnet-3-5
  haiku: claude-haiku-3-5

paths:
  gromit_dir: .gromit

loop:
  max_iterations: 0
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--config", configPath, "analyze-timeouts"})

	// Expected failure: command does not exist
	err = rootCmd.Execute()

	// Should NOT error - should handle gracefully
	if err != nil {
		t.Errorf("expected no error for missing logs dir, got: %v", err)
	}

	output := buf.String()

	// Should show informative message
	if !strings.Contains(output, "No timeout data") && !strings.Contains(output, "0 iterations") {
		t.Errorf("expected informative message about no data, got: %s", output)
	}
}

func TestAnalyzeTimeoutsCommand_DetailedBreakdown(t *testing.T) {
	// Expected failure: analyzeTimeoutsCmd does not exist yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	logsDir := filepath.Join(gromitDir, "logs")

	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	// Create config
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `models:
  opus: claude-opus-4
  sonnet: claude-sonnet-3-5
  haiku: claude-haiku-3-5

paths:
  gromit_dir: .gromit

loop:
  max_iterations: 0
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create log with multiple timeout types for detailed breakdown testing
	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":120000,"timeout_type":"stall","time_to_first_event_ms":5000,"tool_call_count":3,"stall_count":1,"stall_tier":"initial","rate_limit_hits":0,"error":"stall timeout"}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":1200000,"timeout_type":"bead","time_to_first_event_ms":2000,"tool_call_count":15,"error":"bead timeout"}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":600000,"timeout_type":"invocation","time_to_first_event_ms":3000,"tool_call_count":7,"error":"invocation timeout"}
`
	logPath := filepath.Join(logsDir, "run-20260205-120000.jsonl")
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("failed to write log: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--config", configPath, "analyze-timeouts"})

	// Expected failure: command does not exist
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("analyze-timeouts command failed: %v", err)
	}

	output := buf.String()

	// Should show all three timeout types
	if !strings.Contains(output, "Stall:") {
		t.Errorf("expected output to show 'Stall:' breakdown, got: %s", output)
	}
	if !strings.Contains(output, "Bead:") {
		t.Errorf("expected output to show 'Bead:' breakdown, got: %s", output)
	}
	if !strings.Contains(output, "Invocation:") {
		t.Errorf("expected output to show 'Invocation:' breakdown, got: %s", output)
	}

	// Should show rate limit correlation
	if !strings.Contains(output, "Rate Limit Correlation") || !strings.Contains(output, "rate limit") {
		t.Errorf("expected output to show rate limit correlation info, got: %s", output)
	}
}

func TestAnalyzeTimeoutsCommand_JSONOutputEmptyLogs(t *testing.T) {
	// Expected failure: analyzeTimeoutsCmd and --json flag do not exist yet
	tmpDir, configPath := setupAnalyzeTimeoutsTest(t, false)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--config", configPath, "analyze-timeouts", "--json"})

	// Expected failure: command and --json flag do not exist
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("analyze-timeouts --json command failed: %v", err)
	}

	output := buf.String()

	// Parse JSON output - should be valid even with no data
	var analysis logger.TimeoutAnalysis
	if err := json.Unmarshal([]byte(output), &analysis); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, output)
	}

	// Should be zero-value analysis
	if analysis.TotalIterations != 0 {
		t.Errorf("expected TotalIterations=0, got %d", analysis.TotalIterations)
	}
	if analysis.TotalTimeouts != 0 {
		t.Errorf("expected TotalTimeouts=0, got %d", analysis.TotalTimeouts)
	}
	if len(analysis.ByModel) != 0 {
		t.Errorf("expected empty ByModel map, got %d entries", len(analysis.ByModel))
	}
}
