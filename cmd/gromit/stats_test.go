package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/logger"
)

// Expected failure: statsCmd variable does not exist yet
func TestStatsCmd_Registration(t *testing.T) {
	// Verify the stats command is registered with rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "stats" {
			found = true
			break
		}
	}

	if !found {
		t.Error("stats command not registered with rootCmd")
	}
}

// Expected failure: statsCmd does not exist yet
func TestStatsCmd_Flags(t *testing.T) {
	// Verify the --json flag is available
	jsonFlag := statsCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("stats command should have --json flag")
	}

	// Verify flag type is bool
	if jsonFlag != nil && jsonFlag.Value.Type() != "bool" {
		t.Errorf("--json flag should be bool, got %s", jsonFlag.Value.Type())
	}
}

// Expected failure: runStats function does not exist yet
func TestStatsCmd_DisplaysProjectStats(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	// Create gromit.yaml config
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create log file with model stats
	runID := "20260211-120000"
	logs := []logger.IterationLog{
		{
			BeadID:     "bead-1",
			Model:      "opus",
			Success:    true,
			CostUSD:    2.50,
			DurationMs: 45000,
		},
		{
			BeadID:     "bead-2",
			Model:      "opus",
			Success:    true,
			CostUSD:    2.00,
			DurationMs: 40000,
		},
		{
			BeadID:     "bead-3",
			Model:      "sonnet",
			Success:    false,
			Escalated:  true,
			EscalatedTo: "opus",
			CostUSD:    0.50,
			DurationMs: 30000,
		},
		{
			BeadID:     "bead-4",
			Model:      "haiku",
			Success:    true,
			CostUSD:    0.15,
			DurationMs: 20000,
		},
	}

	logFilePath := filepath.Join(logsDir, "run-"+runID+".jsonl")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	defer logFile.Close()

	encoder := json.NewEncoder(logFile)
	for _, log := range logs {
		if err := encoder.Encode(log); err != nil {
			t.Fatalf("failed to write log entry: %v", err)
		}
	}
	logFile.Close()

	// Change to tmpDir so config loading works
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Execute stats command
	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"stats"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("stats command failed: %v", err)
		}
	})

	// Verify project stats are displayed
	if !strings.Contains(output, "opus") {
		t.Error("output should contain opus model stats")
	}
	if !strings.Contains(output, "sonnet") {
		t.Error("output should contain sonnet model stats")
	}
	if !strings.Contains(output, "haiku") {
		t.Error("output should contain haiku model stats")
	}

	// Verify success rate is shown
	if !strings.Contains(output, "success") || !strings.Contains(output, "%") {
		t.Error("output should show success rates as percentages")
	}

	// Verify iteration count is shown
	if !strings.Contains(output, "2") {
		t.Error("output should show opus iteration count of 2")
	}

	// Verify cost information is shown
	if !strings.Contains(output, "$") || !strings.Contains(output, "cost") {
		t.Error("output should show cost information")
	}

	// Verify escalation information is shown
	if !strings.Contains(output, "escalation") || !strings.Contains(output, "Escalation") {
		t.Error("output should show escalation frequency")
	}
}

// Expected failure: runStats function does not exist yet
func TestStatsCmd_DisplaysGlobalStats(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	// Create home directory for global stats
	homeDir := filepath.Join(tmpDir, "home")
	gromitHomeDir := filepath.Join(homeDir, ".gromit")
	if err := os.MkdirAll(gromitHomeDir, 0755); err != nil {
		t.Fatalf("failed to create .gromit home dir: %v", err)
	}

	// Create global stats file
	globalStatsPath := filepath.Join(gromitHomeDir, "stats.json")
	globalStats := logger.GlobalStats{
		Version: 1,
		Updated: "2026-02-11T14:30:00Z",
		Models: map[string]*logger.GlobalModelStats{
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
		},
	}

	data, _ := json.MarshalIndent(globalStats, "", "  ")
	if err := os.WriteFile(globalStatsPath, data, 0644); err != nil {
		t.Fatalf("failed to write global stats: %v", err)
	}

	// Set HOME env var for test
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", homeDir)

	// Create gromit.yaml config
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create minimal project log
	runID := "20260211-120000"
	logs := []logger.IterationLog{
		{
			BeadID:     "bead-1",
			Model:      "opus",
			Success:    true,
			CostUSD:    2.00,
			DurationMs: 40000,
		},
	}

	logFilePath := filepath.Join(logsDir, "run-"+runID+".jsonl")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	defer logFile.Close()

	encoder := json.NewEncoder(logFile)
	for _, log := range logs {
		if err := encoder.Encode(log); err != nil {
			t.Fatalf("failed to write log entry: %v", err)
		}
	}
	logFile.Close()

	// Change to tmpDir so config loading works
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Execute stats command
	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"stats"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("stats command failed: %v", err)
		}
	})

	// Verify global stats are displayed
	if !strings.Contains(output, "global") || !strings.Contains(output, "Global") {
		t.Error("output should indicate global stats section")
	}

	// Verify global iteration counts (much higher than project)
	if !strings.Contains(output, "42") || !strings.Contains(output, "65") {
		t.Error("output should show global iteration counts from all projects")
	}

	// Verify both project and global stats are present
	if !strings.Contains(output, "project") && !strings.Contains(output, "Project") {
		t.Error("output should distinguish between project and global stats")
	}
}

// Expected failure: runStats function does not exist yet
func TestStatsCmd_JSONOutput(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	// Create gromit.yaml config
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create log file with model stats
	runID := "20260211-120000"
	logs := []logger.IterationLog{
		{
			BeadID:     "bead-1",
			Model:      "opus",
			Success:    true,
			CostUSD:    2.50,
			DurationMs: 45000,
		},
		{
			BeadID:     "bead-2",
			Model:      "sonnet",
			Success:    false,
			CostUSD:    0.50,
			DurationMs: 30000,
		},
	}

	logFilePath := filepath.Join(logsDir, "run-"+runID+".jsonl")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	defer logFile.Close()

	encoder := json.NewEncoder(logFile)
	for _, log := range logs {
		if err := encoder.Encode(log); err != nil {
			t.Fatalf("failed to write log entry: %v", err)
		}
	}
	logFile.Close()

	// Change to tmpDir so config loading works
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Execute stats command with --json flag
	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"stats", "--json"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("stats command with --json failed: %v", err)
		}
	})

	// Verify output is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output should be valid JSON, got parse error: %v", err)
	}

	// Verify JSON structure has expected fields
	if _, ok := result["project_stats"]; !ok {
		t.Error("JSON output should have project_stats field")
	}

	if _, ok := result["global_stats"]; !ok {
		t.Error("JSON output should have global_stats field")
	}
}

// Expected failure: runStats function does not exist yet
func TestStatsCmd_ShowsCostPerCompletedBead(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	// Create gromit.yaml config
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create log file simulating retry/escalation chain for a single bead
	runID := "20260211-120000"
	logs := []logger.IterationLog{
		// Bead 1: sonnet fails twice, opus succeeds
		{
			BeadID:      "bead-1",
			Model:       "sonnet",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "sonnet",
			CostUSD:     0.50,
			DurationMs:  20000,
		},
		{
			BeadID:      "bead-1",
			Model:       "sonnet",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "opus",
			CostUSD:     0.50,
			DurationMs:  25000,
		},
		{
			BeadID:     "bead-1",
			Model:      "opus",
			Success:    true,
			CostUSD:    2.50,
			DurationMs: 45000,
		},
		// Bead 2: haiku succeeds immediately
		{
			BeadID:     "bead-2",
			Model:      "haiku",
			Success:    true,
			CostUSD:    0.10,
			DurationMs: 15000,
		},
	}

	logFilePath := filepath.Join(logsDir, "run-"+runID+".jsonl")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	defer logFile.Close()

	encoder := json.NewEncoder(logFile)
	for _, log := range logs {
		if err := encoder.Encode(log); err != nil {
			t.Fatalf("failed to write log entry: %v", err)
		}
	}
	logFile.Close()

	// Change to tmpDir so config loading works
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Execute stats command
	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"stats"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("stats command failed: %v", err)
		}
	})

	// Verify cost-per-completed-bead is shown
	if !strings.Contains(output, "cost") && !strings.Contains(output, "bead") {
		t.Error("output should show cost per completed bead")
	}

	// The cost per completed bead should factor in retries:
	// bead-1: $0.50 + $0.50 + $2.50 = $3.50 total (with retries)
	// bead-2: $0.10 (no retries)
	// This should be reflected in the output
	if !strings.Contains(output, "3.50") || !strings.Contains(output, "0.10") {
		t.Error("output should show full retry chain cost per completed bead")
	}
}

// Expected failure: runStats function does not exist yet
func TestStatsCmd_HandlesNoGlobalStats(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	// Set HOME env var to a non-existent location (no global stats)
	homeDir := filepath.Join(tmpDir, "home")
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", homeDir)

	// Create gromit.yaml config
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create log file with project stats
	runID := "20260211-120000"
	logs := []logger.IterationLog{
		{
			BeadID:     "bead-1",
			Model:      "opus",
			Success:    true,
			CostUSD:    2.00,
			DurationMs: 40000,
		},
	}

	logFilePath := filepath.Join(logsDir, "run-"+runID+".jsonl")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	defer logFile.Close()

	encoder := json.NewEncoder(logFile)
	for _, log := range logs {
		if err := encoder.Encode(log); err != nil {
			t.Fatalf("failed to write log entry: %v", err)
		}
	}
	logFile.Close()

	// Change to tmpDir so config loading works
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Execute stats command - should not error even without global stats
	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"stats"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("stats command should not error when global stats missing: %v", err)
		}
	})

	// Verify project stats are still shown
	if !strings.Contains(output, "opus") {
		t.Error("output should still show project stats when global stats missing")
	}

	// Global stats section should indicate no data or be omitted gracefully
	if strings.Contains(output, "panic") || strings.Contains(output, "error") {
		t.Error("output should handle missing global stats gracefully without errors")
	}
}

// Expected failure: runStats function does not exist yet
func TestStatsCmd_ShowsEscalationFrequency(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	// Create gromit.yaml config
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create log file with escalation patterns
	runID := "20260211-120000"
	logs := []logger.IterationLog{
		// Sonnet escalates to opus twice
		{
			BeadID:      "bead-1",
			Model:       "sonnet",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "opus",
			CostUSD:     0.50,
			DurationMs:  30000,
		},
		{
			BeadID:      "bead-2",
			Model:       "sonnet",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "opus",
			CostUSD:     0.50,
			DurationMs:  30000,
		},
		// Haiku escalates to sonnet once
		{
			BeadID:      "bead-3",
			Model:       "haiku",
			Success:     false,
			Escalated:   true,
			EscalatedTo: "sonnet",
			CostUSD:     0.10,
			DurationMs:  15000,
		},
		// Opus never escalates (highest tier)
		{
			BeadID:     "bead-1",
			Model:      "opus",
			Success:    true,
			CostUSD:    2.50,
			DurationMs: 45000,
		},
		{
			BeadID:     "bead-2",
			Model:      "opus",
			Success:    true,
			CostUSD:    2.50,
			DurationMs: 45000,
		},
	}

	logFilePath := filepath.Join(logsDir, "run-"+runID+".jsonl")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	defer logFile.Close()

	encoder := json.NewEncoder(logFile)
	for _, log := range logs {
		if err := encoder.Encode(log); err != nil {
			t.Fatalf("failed to write log entry: %v", err)
		}
	}
	logFile.Close()

	// Change to tmpDir so config loading works
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Execute stats command
	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"stats"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("stats command failed: %v", err)
		}
	})

	// Verify escalation information is displayed
	if !strings.Contains(output, "escalat") {
		t.Error("output should contain escalation information")
	}

	// Verify sonnet shows escalations FROM (2 times)
	// Verify opus shows escalations TO (2 times)
	// The exact format may vary, but these numbers should appear
	if !strings.Contains(output, "2") {
		t.Error("output should show escalation counts")
	}

	// Verify escalation targets are shown (which model escalated to which)
	if !strings.Contains(output, "opus") && !strings.Contains(output, "sonnet") {
		t.Error("output should show escalation target models")
	}
}

// captureStdout captures stdout during function execution for testing
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	// Reset command flags before execution
	statsCmd.Flags().Set("json", "false")

	// Create a pipe to capture stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	// Save original stdout and replace it
	origStdout := os.Stdout
	os.Stdout = w

	// Capture output in a goroutine to avoid deadlock
	done := make(chan string)
	go func() {
		var buf strings.Builder
		io.Copy(&buf, r)
		done <- buf.String()
	}()

	// Run the function
	fn()

	// Close writer to signal goroutine
	w.Close()

	// Restore stdout
	os.Stdout = origStdout

	// Wait for captured output
	output := <-done

	return output
}
