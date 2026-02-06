package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status represents the current state of Ralph execution
type Status struct {
	Running   bool      `json:"running"`
	Iteration int       `json:"iteration"`
	BeadID    string    `json:"bead_id"`
	BeadTitle string    `json:"bead_title"`
	Model     string    `json:"model"`
	StartedAt time.Time `json:"started_at"`
	ElapsedS  int       `json:"elapsed_s"`
}

// StatusWriter manages writing status.json
type StatusWriter struct {
	path      string
	startTime time.Time
}

// NewStatusWriter creates a new status writer for the given ralph directory
func NewStatusWriter(ralphDir string) *StatusWriter {
	return &StatusWriter{
		path:      filepath.Join(ralphDir, "status.json"),
		startTime: time.Now(),
	}
}

// Write writes the current status to status.json
func (sw *StatusWriter) Write(iteration int, beadID, beadTitle, model string, running bool) error {
	if sw == nil {
		return nil // No-op if writer is nil
	}

	status := Status{
		Running:   running,
		Iteration: iteration,
		BeadID:    beadID,
		BeadTitle: beadTitle,
		Model:     model,
		StartedAt: sw.startTime,
		ElapsedS:  int(time.Since(sw.startTime).Seconds()),
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling status: %w", err)
	}

	// Write with 0644 permissions
	if err := os.WriteFile(sw.path, data, 0644); err != nil {
		return fmt.Errorf("writing status file: %w", err)
	}

	return nil
}

// Delete removes the status file
func (sw *StatusWriter) Delete() error {
	if sw == nil {
		return nil // No-op if writer is nil
	}

	err := os.Remove(sw.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting status file: %w", err)
	}

	return nil
}
