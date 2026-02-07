package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

func TestStatusWriter_Write(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	sw, _ := NewStatusWriter(tmpDir)
	if sw == nil {
		t.Fatal("NewStatusWriter returned nil")
	}

	// Write status
	err := sw.Write(1, "bead-123", "Test Bead", "sonnet", true, 0, 0)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file was created
	statusPath := filepath.Join(tmpDir, "status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("Could not read status.json: %v", err)
	}

	// Verify JSON structure
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("Could not unmarshal status.json: %v", err)
	}

	if status.Running != true {
		t.Errorf("Expected running=true, got %v", status.Running)
	}
	if status.Iteration != 1 {
		t.Errorf("Expected iteration=1, got %d", status.Iteration)
	}
	if status.BeadID != "bead-123" {
		t.Errorf("Expected BeadID=bead-123, got %s", status.BeadID)
	}
	if status.BeadTitle != "Test Bead" {
		t.Errorf("Expected BeadTitle=Test Bead, got %s", status.BeadTitle)
	}
	if status.Model != "sonnet" {
		t.Errorf("Expected Model=sonnet, got %s", status.Model)
	}
	if status.ElapsedS < 0 {
		t.Errorf("Expected ElapsedS >= 0, got %d", status.ElapsedS)
	}
	if status.PID != os.Getpid() {
		t.Errorf("Expected PID=%d, got %d", os.Getpid(), status.PID)
	}
}

func TestStatusWriter_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	sw, _ := NewStatusWriter(tmpDir)
	statusPath := filepath.Join(tmpDir, "status.json")

	// Write status
	err := sw.Write(1, "bead-123", "Test Bead", "sonnet", true, 0, 0)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(statusPath); err != nil {
		t.Fatalf("Status file does not exist: %v", err)
	}

	// Delete status
	err = sw.Delete()
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(statusPath); err == nil {
		t.Fatal("Status file still exists after delete")
	} else if !os.IsNotExist(err) {
		t.Fatalf("Unexpected error checking status file: %v", err)
	}
}

func TestStatusWriter_Delete_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	sw, _ := NewStatusWriter(tmpDir)

	// Delete without creating - should not fail
	err := sw.Delete()
	if err != nil {
		t.Fatalf("Delete failed on non-existent file: %v", err)
	}
}

func TestStatusWriter_NilWriter(t *testing.T) {
	var sw *StatusWriter

	// Should be no-op without crashing
	err := sw.Write(1, "bead-123", "Test Bead", "sonnet", true, 0, 0)
	if err != nil {
		t.Fatalf("Write on nil writer should be no-op: %v", err)
	}

	err = sw.Delete()
	if err != nil {
		t.Fatalf("Delete on nil writer should be no-op: %v", err)
	}
}

func TestStatusWriter_ElapsedTime(t *testing.T) {
	tmpDir := t.TempDir()

	sw, _ := NewStatusWriter(tmpDir)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Write status
	err := sw.Write(1, "bead-123", "Test Bead", "sonnet", true, 0, 0)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify elapsed time is reasonable
	statusPath := filepath.Join(tmpDir, "status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("Could not read status.json: %v", err)
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("Could not unmarshal status.json: %v", err)
	}

	if status.ElapsedS < 0 {
		t.Errorf("Expected ElapsedS >= 0, got %d", status.ElapsedS)
	}
}

func TestStatusWriter_Write_IncludesPID(t *testing.T) {
	tmpDir := t.TempDir()

	sw, _ := NewStatusWriter(tmpDir)

	// Write status
	err := sw.Write(1, "bead-456", "PID Test Bead", "haiku", true, 0, 0)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read status file
	statusPath := filepath.Join(tmpDir, "status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("Could not read status.json: %v", err)
	}

	// Unmarshal and validate PID
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("Could not unmarshal status.json: %v", err)
	}

	expectedPID := os.Getpid()
	if status.PID != expectedPID {
		t.Errorf("Expected PID=%d, got %d", expectedPID, status.PID)
	}

	if status.PID <= 0 {
		t.Errorf("Expected positive PID, got %d", status.PID)
	}
}

func TestReadStatus(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a status file first
	sw, _ := NewStatusWriter(tmpDir)
	err := sw.Write(2, "bead-789", "Read Test Bead", "opus", true, 0, 0)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read the status
	status, err := ReadStatus(tmpDir)
	if err != nil {
		t.Fatalf("ReadStatus failed: %v", err)
	}

	if status == nil {
		t.Fatal("Expected status, got nil")
	}

	// Verify fields
	if status.Running != true {
		t.Errorf("Expected running=true, got %v", status.Running)
	}
	if status.Iteration != 2 {
		t.Errorf("Expected iteration=2, got %d", status.Iteration)
	}
	if status.BeadID != "bead-789" {
		t.Errorf("Expected BeadID=bead-789, got %s", status.BeadID)
	}
	if status.BeadTitle != "Read Test Bead" {
		t.Errorf("Expected BeadTitle=Read Test Bead, got %s", status.BeadTitle)
	}
	if status.Model != "opus" {
		t.Errorf("Expected Model=opus, got %s", status.Model)
	}
	if status.PID != os.Getpid() {
		t.Errorf("Expected PID=%d, got %d", os.Getpid(), status.PID)
	}
}

func TestReadStatus_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	// Read from directory without status.json
	status, err := ReadStatus(tmpDir)
	if err != nil {
		t.Fatalf("ReadStatus should not error on non-existent file, got: %v", err)
	}

	if status != nil {
		t.Errorf("Expected nil status for non-existent file, got: %v", status)
	}
}

func TestReadStatus_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON
	statusPath := filepath.Join(tmpDir, "status.json")
	err := os.WriteFile(statusPath, []byte("not valid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	// Read should return error
	status, err := ReadStatus(tmpDir)
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}

	if status != nil {
		t.Errorf("Expected nil status for invalid JSON, got: %v", status)
	}
}

func TestStatusWriter_Write_WithLimits(t *testing.T) {
	tmpDir := t.TempDir()

	sw, _ := NewStatusWriter(tmpDir)

	// Write status with max iterations and time budget
	err := sw.Write(5, "bead-limit-123", "Limited Bead", "sonnet", true, 50, 30)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read and verify
	statusPath := filepath.Join(tmpDir, "status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("Could not read status.json: %v", err)
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("Could not unmarshal status.json: %v", err)
	}

	if status.MaxIterations != 50 {
		t.Errorf("Expected MaxIterations=50, got %d", status.MaxIterations)
	}
	if status.TimeBudgetMinutes != 30 {
		t.Errorf("Expected TimeBudgetMinutes=30, got %d", status.TimeBudgetMinutes)
	}
}

func TestStatusWriter_Write_WithoutLimits(t *testing.T) {
	tmpDir := t.TempDir()

	sw, _ := NewStatusWriter(tmpDir)

	// Write status without limits (zero values)
	err := sw.Write(1, "bead-nolimit-456", "Unlimited Bead", "haiku", true, 0, 0)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read and verify
	statusPath := filepath.Join(tmpDir, "status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("Could not read status.json: %v", err)
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("Could not unmarshal status.json: %v", err)
	}

	// With omitempty, these fields should be zero but still parseable
	if status.MaxIterations != 0 {
		t.Errorf("Expected MaxIterations=0, got %d", status.MaxIterations)
	}
	if status.TimeBudgetMinutes != 0 {
		t.Errorf("Expected TimeBudgetMinutes=0, got %d", status.TimeBudgetMinutes)
	}
}

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	// Current process should be alive
	if !IsProcessAlive(os.Getpid()) {
		t.Error("Expected current process to be alive")
	}
}

func TestIsProcessAlive_NonExistentProcess(t *testing.T) {
	// PID 0 is never a valid user process on Unix systems
	// On most systems, it's the kernel scheduler
	// We use a very high PID that's unlikely to exist
	nonExistentPID := 999999
	if IsProcessAlive(nonExistentPID) {
		t.Errorf("Expected PID %d to not be alive", nonExistentPID)
	}
}

func TestRunner_Status_NoStatusFile(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	var buf strings.Builder
	cfg := &config.Config{}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{
				ID:       "test-123",
				Title:    "Test Bead",
				Priority: 1,
				Labels:   []string{"label1"},
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

	// Execute
	err = r.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	// Verify - should show normal status, no run-in-progress message
	output := buf.String()
	if strings.Contains(output, "Run in progress") {
		t.Errorf("Expected no 'Run in progress' message when status file doesn't exist, got: %s", output)
	}
	if strings.Contains(output, "stale run") {
		t.Errorf("Expected no 'stale run' message when status file doesn't exist, got: %s", output)
	}
	if !strings.Contains(output, "Next bead: test-123") {
		t.Errorf("Expected normal status output with next bead, got: %s", output)
	}
}

func TestRunner_Status_LivePID(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	var buf strings.Builder
	cfg := &config.Config{}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{
				ID:       "test-456",
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

	// Write a status file with live PID (current process)
	sw, _ := NewStatusWriter(tmpDir)
	err = sw.Write(3, "running-bead-789", "Running Bead Title", "opus", true, 0, 0)
	if err != nil {
		t.Fatalf("Failed to write status file: %v", err)
	}

	// Execute
	err = r.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	// Verify - should show run-in-progress info
	output := buf.String()
	if !strings.Contains(output, "Run in progress") {
		t.Errorf("Expected 'Run in progress' message for live PID, got: %s", output)
	}
	if !strings.Contains(output, "Iteration: 3") {
		t.Errorf("Expected iteration info, got: %s", output)
	}
	if !strings.Contains(output, "running-bead-789") {
		t.Errorf("Expected bead ID in output, got: %s", output)
	}
	if !strings.Contains(output, "Running Bead Title") {
		t.Errorf("Expected bead title in output, got: %s", output)
	}
	if !strings.Contains(output, "Model: opus") {
		t.Errorf("Expected model in output, got: %s", output)
	}
	if strings.Contains(output, "stale run") {
		t.Errorf("Should not show stale run message for live PID, got: %s", output)
	}

	// Should still show normal status after run-in-progress info
	if !strings.Contains(output, "Next bead: test-456") {
		t.Errorf("Expected normal status after run-in-progress info, got: %s", output)
	}

	// Verify status file still exists (not deleted for live run)
	statusPath := filepath.Join(tmpDir, "status.json")
	if _, err := os.Stat(statusPath); err != nil {
		t.Errorf("Status file should still exist for live run: %v", err)
	}
}

func TestRunner_Status_DeadPID(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	var buf strings.Builder
	cfg := &config.Config{}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{
				ID:       "test-999",
				Title:    "Next Available Bead",
				Priority: 2,
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

	// Get a dead PID by spawning a subprocess and waiting for it to exit
	deadPID := getDeadPID(t)

	// Write a status file with the dead PID
	statusPath := filepath.Join(tmpDir, "status.json")
	statusData := fmt.Sprintf(`{
  "running": true,
  "iteration": 5,
  "bead_id": "crashed-bead-999",
  "bead_title": "Crashed Bead Title",
  "model": "sonnet",
  "started_at": "%s",
  "elapsed_s": 120,
  "pid": %d
}`, time.Now().Add(-2*time.Hour).Format(time.RFC3339), deadPID)
	err = os.WriteFile(statusPath, []byte(statusData), 0644)
	if err != nil {
		t.Fatalf("Failed to write status file: %v", err)
	}

	// Execute
	err = r.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	// Verify - should show stale run warning
	output := buf.String()
	if !strings.Contains(output, "stale run") {
		t.Errorf("Expected 'stale run' message for dead PID, got: %s", output)
	}
	if !strings.Contains(output, "crashed-bead-999") {
		t.Errorf("Expected bead ID in stale run warning, got: %s", output)
	}
	if !strings.Contains(output, "Crashed Bead Title") {
		t.Errorf("Expected bead title in stale run warning, got: %s", output)
	}
	if !strings.Contains(output, "Removing stale status file") {
		t.Errorf("Expected file removal message, got: %s", output)
	}

	// Should show normal status after warning
	if !strings.Contains(output, "Next bead: test-999") {
		t.Errorf("Expected normal status after stale run warning, got: %s", output)
	}

	// Verify status file was deleted
	if _, err := os.Stat(statusPath); err == nil {
		t.Error("Status file should have been deleted for dead PID")
	} else if !os.IsNotExist(err) {
		t.Errorf("Unexpected error checking status file: %v", err)
	}
}

func TestStatusWriter_WriteFinal(t *testing.T) {
	tmpDir := t.TempDir()

	sw, _ := NewStatusWriter(tmpDir)

	// Write a running status first
	err := sw.Write(5, "bead-123", "Test Bead", "sonnet", true, 50, 30)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Wait a bit to ensure elapsed time is non-zero
	time.Sleep(100 * time.Millisecond)

	// Write final status
	err = sw.WriteFinal(5)
	if err != nil {
		t.Fatalf("WriteFinal failed: %v", err)
	}

	// Read and verify
	statusPath := filepath.Join(tmpDir, "status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("Could not read status.json: %v", err)
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("Could not unmarshal status.json: %v", err)
	}

	// Verify final status characteristics
	if status.Running != false {
		t.Errorf("Expected running=false, got %v", status.Running)
	}
	if status.Iteration != 5 {
		t.Errorf("Expected iteration=5, got %d", status.Iteration)
	}
	if status.BeadID != "" {
		t.Errorf("Expected empty BeadID, got %s", status.BeadID)
	}
	if status.BeadTitle != "" {
		t.Errorf("Expected empty BeadTitle, got %s", status.BeadTitle)
	}
	if status.Model != "" {
		t.Errorf("Expected empty Model, got %s", status.Model)
	}
	if status.ElapsedS < 0 {
		t.Errorf("Expected ElapsedS >= 0, got %d", status.ElapsedS)
	}
	if status.PID != os.Getpid() {
		t.Errorf("Expected PID=%d, got %d", os.Getpid(), status.PID)
	}
	// MaxIterations and TimeBudgetMinutes should be zero (omitted from JSON)
	if status.MaxIterations != 0 {
		t.Errorf("Expected MaxIterations=0, got %d", status.MaxIterations)
	}
	if status.TimeBudgetMinutes != 0 {
		t.Errorf("Expected TimeBudgetMinutes=0, got %d", status.TimeBudgetMinutes)
	}
}

func TestStatusWriter_WriteFinal_NilWriter(t *testing.T) {
	var sw *StatusWriter

	// Should be no-op without crashing
	err := sw.WriteFinal(10)
	if err != nil {
		t.Fatalf("WriteFinal on nil writer should be no-op: %v", err)
	}
}

func TestStatusWriter_WriteFinal_PreservesElapsedTime(t *testing.T) {
	tmpDir := t.TempDir()

	sw, _ := NewStatusWriter(tmpDir)

	// Wait a bit before writing final
	time.Sleep(100 * time.Millisecond)

	// Write final status
	err := sw.WriteFinal(3)
	if err != nil {
		t.Fatalf("WriteFinal failed: %v", err)
	}

	// Read and verify elapsed time is reasonable
	statusPath := filepath.Join(tmpDir, "status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("Could not read status.json: %v", err)
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("Could not unmarshal status.json: %v", err)
	}

	if status.ElapsedS < 0 {
		t.Errorf("Expected ElapsedS >= 0, got %d", status.ElapsedS)
	}
	// Should have at least 0 seconds elapsed (100ms rounds to 0)
	if status.ElapsedS < 0 {
		t.Errorf("Expected non-negative elapsed time, got %d", status.ElapsedS)
	}
}

// getDeadPID spawns a subprocess that exits immediately and returns its PID
func getDeadPID(t *testing.T) int {
	t.Helper()

	// Use exec.Command to spawn a process that exits immediately
	cmd := exec.Command("true")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start subprocess: %v", err)
	}

	pid := cmd.Process.Pid

	// Wait for it to exit
	err = cmd.Wait()
	if err != nil {
		t.Fatalf("Failed to wait for subprocess: %v", err)
	}

	// Verify the PID is indeed dead
	if IsProcessAlive(pid) {
		t.Fatalf("PID %d should be dead but isn't", pid)
	}

	return pid
}
