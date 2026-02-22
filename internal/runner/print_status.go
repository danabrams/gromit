package runner

import (
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/config"
)

// PrintStatus reads status.json and writes a formatted status display to w.
// processChecker, when non-nil, is used to verify whether the PID in status.json
// is still alive; passing nil defaults to IsProcessAlive.
func PrintStatus(gromitDir string, cfg *config.Config, w io.Writer, processChecker func(int) bool) error {
	status, err := ReadStatus(gromitDir)
	if err != nil {
		return err
	}
	if status == nil {
		if _, err := fmt.Fprintln(w, "No status file found. Gromit may not have run yet."); err != nil {
			return fmt.Errorf("writing status: %w", err)
		}
		return nil
	}

	if processChecker == nil {
		processChecker = IsProcessAlive
	}

	// Detect stale PID: status says running but process is dead.
	if status.Running && !processChecker(status.PID) {
		status.Running = false
	}

	if _, err := fmt.Fprintln(w, formatRun(status)); err != nil {
		return fmt.Errorf("writing status: %w", err)
	}
	return nil
}
