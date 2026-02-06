package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStatusWriter_Write(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	sw := NewStatusWriter(tmpDir)
	if sw == nil {
		t.Fatal("NewStatusWriter returned nil")
	}

	// Write status
	err := sw.Write(1, "bead-123", "Test Bead", "sonnet", true)
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
}

func TestStatusWriter_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	sw := NewStatusWriter(tmpDir)
	statusPath := filepath.Join(tmpDir, "status.json")

	// Write status
	err := sw.Write(1, "bead-123", "Test Bead", "sonnet", true)
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

	sw := NewStatusWriter(tmpDir)

	// Delete without creating - should not fail
	err := sw.Delete()
	if err != nil {
		t.Fatalf("Delete failed on non-existent file: %v", err)
	}
}

func TestStatusWriter_NilWriter(t *testing.T) {
	var sw *StatusWriter

	// Should be no-op without crashing
	err := sw.Write(1, "bead-123", "Test Bead", "sonnet", true)
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

	sw := NewStatusWriter(tmpDir)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Write status
	err := sw.Write(1, "bead-123", "Test Bead", "sonnet", true)
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
