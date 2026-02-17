package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Status represents the current state of Gromit execution
type Status struct {
	Running           bool      `json:"running"`
	Iteration         int       `json:"iteration"`
	BeadID            string    `json:"bead_id"`
	BeadTitle         string    `json:"bead_title"`
	Model             string    `json:"model"`
	StartedAt         time.Time `json:"started_at"`
	ElapsedS          int       `json:"elapsed_s"`
	PID               int       `json:"pid"`
	MaxIterations     int       `json:"max_iterations,omitempty"`
	TimeBudgetMinutes int       `json:"time_budget_minutes,omitempty"`
	LastFailureClass  string    `json:"last_failure_class,omitempty"`
	LastAndonLevel    string    `json:"last_andon_level,omitempty"`
	LastTrimDecision  string    `json:"last_trim_decision,omitempty"`

	AutonomyRate           float64            `json:"autonomy_rate,omitempty"`
	FirstPassSuccessRate   float64            `json:"first_pass_success_rate,omitempty"`
	MTTRProxyMs            int64              `json:"mttr_proxy_ms,omitempty"`
	EscalationRatesByClass map[string]float64 `json:"escalation_rates_by_class,omitempty"`
	RecurrenceCounters     map[string]int     `json:"recurrence_counters,omitempty"`
}

// StatusWriter manages writing status.json
type StatusWriter struct {
	path      string
	startTime time.Time
	mu        sync.Mutex
}

// NewStatusWriter creates a new status writer for the given gromit directory
func NewStatusWriter(gromitDir string) (*StatusWriter, error) {
	return &StatusWriter{
		path:      filepath.Join(gromitDir, "status.json"),
		startTime: time.Now(),
	}, nil
}

// Write writes the current status to status.json
func (sw *StatusWriter) Write(iteration int, beadID, beadTitle, model string, running bool, maxIterations, timeBudgetMinutes int) error {
	if sw == nil {
		return nil // No-op if writer is nil
	}
	sw.mu.Lock()
	defer sw.mu.Unlock()

	status := Status{
		Running:           running,
		Iteration:         iteration,
		BeadID:            beadID,
		BeadTitle:         beadTitle,
		Model:             model,
		StartedAt:         sw.startTime,
		ElapsedS:          int(time.Since(sw.startTime).Seconds()),
		PID:               os.Getpid(),
		MaxIterations:     maxIterations,
		TimeBudgetMinutes: timeBudgetMinutes,
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
	sw.mu.Lock()
	defer sw.mu.Unlock()

	err := os.Remove(sw.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting status file: %w", err)
	}

	return nil
}

// WriteFinal writes a final status entry with running: false on clean exit.
// The iteration parameter is the total number of completed iterations.
func (sw *StatusWriter) WriteFinal(iteration int) error {
	if sw == nil {
		return nil // No-op if writer is nil
	}
	sw.mu.Lock()
	defer sw.mu.Unlock()

	status := Status{
		Running:   false,
		Iteration: iteration,
		BeadID:    "",
		BeadTitle: "",
		Model:     "",
		StartedAt: sw.startTime,
		ElapsedS:  int(time.Since(sw.startTime).Seconds()),
		PID:       os.Getpid(),
		// MaxIterations and TimeBudgetMinutes are omitted (zero values) on final write
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling final status: %w", err)
	}

	// Write with 0644 permissions
	if err := os.WriteFile(sw.path, data, 0644); err != nil {
		return fmt.Errorf("writing final status file: %w", err)
	}

	return nil
}

// ReadStatus reads and parses status.json from the gromit directory.
// Returns nil, nil when the file doesn't exist (not an error).
func ReadStatus(gromitDir string) (*Status, error) {
	path := filepath.Join(gromitDir, "status.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading status file: %w", err)
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("unmarshaling status: %w", err)
	}

	return &status, nil
}

// IsProcessAlive checks if a process with the given PID is still running.
// Returns false if the process doesn't exist or signal fails.
func IsProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if the process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
