package worktree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
)

// IsRunLoopActive returns true when the run loop is currently active.
// It checks status.json for Running==true and verifies the PID is alive.
// Returns false for any error condition (missing file, invalid JSON, dead PID, etc).
func IsRunLoopActive(gromitDir string) bool {
	if gromitDir == "" {
		return false
	}

	statusPath := filepath.Join(gromitDir, "status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return false
	}

	var status struct {
		Running bool `json:"running"`
		PID     int  `json:"pid"`
	}

	if err := json.Unmarshal(data, &status); err != nil {
		return false
	}

	if !status.Running {
		return false
	}

	if status.PID <= 0 {
		return false
	}

	return isProcessAlive(status.PID)
}

func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}
