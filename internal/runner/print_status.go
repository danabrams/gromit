package runner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/state"
)

// PrintStatus reads status.json and writes a formatted status display to w.
// processChecker, when non-nil, is used to verify whether the PID in status.json
// is still alive; passing nil defaults to IsProcessAlive.
func PrintStatus(gromitDir string, cfg *config.Config, w io.Writer, processChecker func(int) bool) error {
	status, err := ReadStatus(gromitDir)
	if err != nil {
		return err
	}

	if status != nil {
		if processChecker == nil {
			processChecker = IsProcessAlive
		}

		if err := handleStalePID(status, gromitDir, w, processChecker); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, formatRun(status)); err != nil {
		return fmt.Errorf("writing status: %w", err)
	}

	// Pipeline section
	var startedAt *time.Time
	if status != nil && !status.StartedAt.IsZero() {
		startedAt = &status.StartedAt
	}
	ps, err := pipeline.ReadStatus(gromitDir, cfg.Paths.Specs, cfg.Paths.Plans, startedAt)
	if err != nil {
		return fmt.Errorf("reading pipeline status: %w", err)
	}
	if _, err := fmt.Fprintln(w, formatPipeline(ps)); err != nil {
		return fmt.Errorf("writing pipeline status: %w", err)
	}

	// Next action recommendation
	if rec := formatRecommendation(ps.Recommendation); rec != "" {
		if _, err := fmt.Fprintln(w, rec); err != nil {
			return fmt.Errorf("writing recommendation: %w", err)
		}
	}

	// Health section: read state.json and interactive-state.json
	var iterationsSinceReview int
	var lastRetro time.Time

	if sf, err := state.NewFile(gromitDir); err == nil {
		if err := sf.Load(); err == nil {
			iterationsSinceReview = sf.IterationsSinceReview()
		}
	}

	if isf, err := state.NewInteractiveFile(gromitDir); err == nil {
		if err := isf.Load(); err == nil {
			lastRetro = isf.LastRetro()
		}
	}

	if _, err := fmt.Fprint(w, formatHealth(lastRetro, iterationsSinceReview)); err != nil {
		return fmt.Errorf("writing health status: %w", err)
	}

	// Model Performance section: read per-model stats from iteration logs
	if cfg.Paths.Logs != "" {
		if modelStats, err := logger.ReadModelStats(cfg.Paths.Logs); err == nil && len(modelStats) > 0 {
			if _, err := fmt.Fprintln(w, formatModelPerformance(modelStats)); err != nil {
				return fmt.Errorf("writing model performance: %w", err)
			}
		}
	}

	// SPC section: read process trend from logs directory
	var trend *logger.ProcessTrend
	if cfg.Paths.Logs != "" {
		trend, _ = logger.ReadProcessTrend(filepath.Join(cfg.Paths.Logs, "process_trend.json"))
	}
	if _, err := fmt.Fprintln(w, formatSPCSummary(trend)); err != nil {
		return fmt.Errorf("writing SPC status: %w", err)
	}

	return nil
}

// handleStalePID checks whether status indicates a stale run (Running=true but
// process is dead). If stale, it warns the user, removes the status file, and
// sets Running=false so subsequent formatting shows "not running".
func handleStalePID(status *Status, gromitDir string, w io.Writer, processChecker func(int) bool) error {
	if !status.Running || processChecker(status.PID) {
		return nil
	}

	if _, err := fmt.Fprintf(w, "Warning: stale run detected (PID %d is no longer alive)\n", status.PID); err != nil {
		return fmt.Errorf("writing stale warning: %w", err)
	}
	if _, err := fmt.Fprintf(w, "  Bead: %s — %s\n", status.BeadID, status.BeadTitle); err != nil {
		return fmt.Errorf("writing stale bead info: %w", err)
	}
	if _, err := fmt.Fprintln(w, "Removing stale status file"); err != nil {
		return fmt.Errorf("writing removal message: %w", err)
	}

	statusPath := filepath.Join(gromitDir, "status.json")
	if err := os.Remove(statusPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing stale status file: %w", err)
	}

	status.Running = false
	return nil
}
