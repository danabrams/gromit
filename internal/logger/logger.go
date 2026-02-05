package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// IterationLog represents a single iteration's outcome
type IterationLog struct {
	Timestamp   time.Time `json:"timestamp"`
	Iteration   int       `json:"iteration"`
	BeadID      string    `json:"bead_id"`
	BeadTitle   string    `json:"bead_title"`
	Model       string    `json:"model"`
	Success     bool      `json:"success"`
	Validated   bool      `json:"validated"`
	Escalated   bool      `json:"escalated"`
	EscalatedTo string    `json:"escalated_to,omitempty"`
	DurationMs  int64     `json:"duration_ms"`
	Error       string    `json:"error,omitempty"`
}

// Logger writes iteration logs to a file
type Logger struct {
	dir     string
	file    *os.File
	encoder *json.Encoder
	runID   string
}

// NewLogger creates a new logger that writes to the specified directory
func NewLogger(logsDir string) (*Logger, error) {
	// Create logs directory if needed
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating logs directory: %w", err)
	}

	// Generate run ID based on timestamp
	runID := time.Now().Format("20060102-150405")
	filename := filepath.Join(logsDir, fmt.Sprintf("run-%s.jsonl", runID))

	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("creating log file: %w", err)
	}

	return &Logger{
		dir:     logsDir,
		file:    file,
		encoder: json.NewEncoder(file),
		runID:   runID,
	}, nil
}

// LogIteration writes an iteration result to the log
func (l *Logger) LogIteration(log *IterationLog) error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.encoder.Encode(log)
}

// Close closes the log file
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

// RunID returns the current run's ID
func (l *Logger) RunID() string {
	if l == nil {
		return ""
	}
	return l.runID
}

// FilePath returns the path to the current log file
func (l *Logger) FilePath() string {
	if l == nil || l.file == nil {
		return ""
	}
	return l.file.Name()
}
