package worktree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestIsRunLoopActive_NoStatusFile verifies that IsRunLoopActive returns false
// when status.json does not exist.
func TestIsRunLoopActive_NoStatusFile(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	active := IsRunLoopActive(gromitDir)
	if active {
		t.Error("IsRunLoopActive() = true when status.json does not exist, want false")
	}
}

// TestIsRunLoopActive_RunningFalse verifies that IsRunLoopActive returns false
// when status.json exists but Running field is false.
func TestIsRunLoopActive_RunningFalse(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Write status.json with Running: false
	status := map[string]interface{}{
		"running": false,
		"pid":     os.Getpid(),
	}
	statusPath := filepath.Join(gromitDir, "status.json")
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(statusPath, data, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	active := IsRunLoopActive(gromitDir)
	if active {
		t.Error("IsRunLoopActive() = true when Running is false, want false")
	}
}

// TestIsRunLoopActive_RunningTrueButDeadPID verifies that IsRunLoopActive returns false
// when status.json has Running=true but the PID is no longer alive.
func TestIsRunLoopActive_RunningTrueButDeadPID(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Write status.json with Running: true but a PID that doesn't exist
	// Use an impossibly high PID that should not exist on any system
	status := map[string]interface{}{
		"running": true,
		"pid":     999999999,
	}
	statusPath := filepath.Join(gromitDir, "status.json")
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(statusPath, data, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	active := IsRunLoopActive(gromitDir)
	if active {
		t.Error("IsRunLoopActive() = true when PID is dead, want false")
	}
}

// TestIsRunLoopActive_RunningTrueAndAlivePID verifies that IsRunLoopActive returns true
// when status.json has Running=true and the PID is alive.
func TestIsRunLoopActive_RunningTrueAndAlivePID(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Write status.json with Running: true and current process PID (which is alive)
	status := map[string]interface{}{
		"running": true,
		"pid":     os.Getpid(),
	}
	statusPath := filepath.Join(gromitDir, "status.json")
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(statusPath, data, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	active := IsRunLoopActive(gromitDir)
	if !active {
		t.Error("IsRunLoopActive() = false when Running is true and PID is alive, want true")
	}
}

// TestIsRunLoopActive_MalformedJSON verifies that IsRunLoopActive returns false
// when status.json exists but contains invalid JSON.
func TestIsRunLoopActive_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Write invalid JSON to status.json
	statusPath := filepath.Join(gromitDir, "status.json")
	if err := os.WriteFile(statusPath, []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	active := IsRunLoopActive(gromitDir)
	if active {
		t.Error("IsRunLoopActive() = true when status.json is malformed, want false")
	}
}

// TestIsRunLoopActive_EmptyGromitDir verifies that IsRunLoopActive returns false
// when given an empty gromit directory path.
func TestIsRunLoopActive_EmptyGromitDir(t *testing.T) {
	active := IsRunLoopActive("")
	if active {
		t.Error("IsRunLoopActive(\"\") = true, want false")
	}
}

// TestIsRunLoopActive_NonexistentGromitDir verifies that IsRunLoopActive returns false
// when the gromit directory does not exist.
func TestIsRunLoopActive_NonexistentGromitDir(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, "does-not-exist")

	active := IsRunLoopActive(gromitDir)
	if active {
		t.Error("IsRunLoopActive() = true for nonexistent directory, want false")
	}
}

// TestIsRunLoopActive_TableDriven verifies IsRunLoopActive behavior across
// multiple scenarios using a table-driven approach.
func TestIsRunLoopActive_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		running        bool
		pid            int
		createFile     bool
		malformedJSON  bool
		expectedActive bool
	}{
		{
			name:           "running true with alive PID",
			running:        true,
			pid:            os.Getpid(),
			createFile:     true,
			malformedJSON:  false,
			expectedActive: true,
		},
		{
			name:           "running true with dead PID",
			running:        true,
			pid:            999999999,
			createFile:     true,
			malformedJSON:  false,
			expectedActive: false,
		},
		{
			name:           "running false with alive PID",
			running:        false,
			pid:            os.Getpid(),
			createFile:     true,
			malformedJSON:  false,
			expectedActive: false,
		},
		{
			name:           "running false with dead PID",
			running:        false,
			pid:            999999999,
			createFile:     true,
			malformedJSON:  false,
			expectedActive: false,
		},
		{
			name:           "no status file",
			running:        true,
			pid:            os.Getpid(),
			createFile:     false,
			malformedJSON:  false,
			expectedActive: false,
		},
		{
			name:           "malformed JSON",
			running:        true,
			pid:            os.Getpid(),
			createFile:     true,
			malformedJSON:  true,
			expectedActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gromitDir := filepath.Join(tmpDir, ".gromit")
			if err := os.MkdirAll(gromitDir, 0755); err != nil {
				t.Fatalf("failed to create gromit dir: %v", err)
			}

			if tt.createFile {
				statusPath := filepath.Join(gromitDir, "status.json")
				if tt.malformedJSON {
					if err := os.WriteFile(statusPath, []byte("{invalid json"), 0644); err != nil {
						t.Fatalf("failed to write malformed status.json: %v", err)
					}
				} else {
					status := map[string]interface{}{
						"running": tt.running,
						"pid":     tt.pid,
					}
					data, err := json.Marshal(status)
					if err != nil {
						t.Fatalf("failed to marshal status: %v", err)
					}
					if err := os.WriteFile(statusPath, data, 0644); err != nil {
						t.Fatalf("failed to write status.json: %v", err)
					}
				}
			}

			active := IsRunLoopActive(gromitDir)
			if active != tt.expectedActive {
				t.Errorf("IsRunLoopActive() = %v, want %v", active, tt.expectedActive)
			}
		})
	}
}

// TestIsRunLoopActive_ZeroPID verifies that IsRunLoopActive returns false
// when the PID field is zero (invalid PID).
func TestIsRunLoopActive_ZeroPID(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Write status.json with Running: true but PID: 0
	status := map[string]interface{}{
		"running": true,
		"pid":     0,
	}
	statusPath := filepath.Join(gromitDir, "status.json")
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(statusPath, data, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	active := IsRunLoopActive(gromitDir)
	if active {
		t.Error("IsRunLoopActive() = true when PID is 0, want false")
	}
}

// TestIsRunLoopActive_NegativePID verifies that IsRunLoopActive returns false
// when the PID field is negative (invalid PID).
func TestIsRunLoopActive_NegativePID(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Write status.json with Running: true but PID: -1
	status := map[string]interface{}{
		"running": true,
		"pid":     -1,
	}
	statusPath := filepath.Join(gromitDir, "status.json")
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(statusPath, data, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	active := IsRunLoopActive(gromitDir)
	if active {
		t.Error("IsRunLoopActive() = true when PID is negative, want false")
	}
}

// TestIsRunLoopActive_MissingPIDField verifies that IsRunLoopActive returns false
// when status.json is valid JSON but missing the PID field.
func TestIsRunLoopActive_MissingPIDField(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Write status.json with Running: true but no PID field
	status := map[string]interface{}{
		"running": true,
	}
	statusPath := filepath.Join(gromitDir, "status.json")
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(statusPath, data, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	active := IsRunLoopActive(gromitDir)
	if active {
		t.Error("IsRunLoopActive() = true when PID field is missing, want false")
	}
}

// TestIsRunLoopActive_MissingRunningField verifies that IsRunLoopActive returns false
// when status.json is valid JSON but missing the Running field.
func TestIsRunLoopActive_MissingRunningField(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Write status.json with PID but no Running field (defaults to false)
	status := map[string]interface{}{
		"pid": os.Getpid(),
	}
	statusPath := filepath.Join(gromitDir, "status.json")
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}
	if err := os.WriteFile(statusPath, data, 0644); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}

	active := IsRunLoopActive(gromitDir)
	if active {
		t.Error("IsRunLoopActive() = true when Running field is missing (defaults to false), want false")
	}
}

// TestIsRunLoopActive_StatusFileIsDirectory verifies that IsRunLoopActive returns false
// when status.json is actually a directory (edge case).
func TestIsRunLoopActive_StatusFileIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	// Create status.json as a directory instead of a file
	statusPath := filepath.Join(gromitDir, "status.json")
	if err := os.MkdirAll(statusPath, 0755); err != nil {
		t.Fatalf("failed to create status.json directory: %v", err)
	}

	active := IsRunLoopActive(gromitDir)
	if active {
		t.Error("IsRunLoopActive() = true when status.json is a directory, want false")
	}
}
