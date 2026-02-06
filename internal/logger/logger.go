package logger

import (
	"bytes"
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

// Logger writes iteration logs to a file.
// The log file is created lazily on the first LogIteration call to avoid
// leaving empty 0-byte files when runs fail before processing any bead.
type Logger struct {
	dir      string
	filePath string
	file     *os.File
	encoder  *json.Encoder
	runID    string
}

// NewLogger creates a new logger that writes to the specified directory.
// The log file is not created until the first iteration is logged.
func NewLogger(logsDir string) (*Logger, error) {
	// Create logs directory if needed
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating logs directory: %w", err)
	}

	// Generate run ID based on timestamp
	runID := time.Now().Format("20060102-150405")
	filePath := filepath.Join(logsDir, fmt.Sprintf("run-%s.jsonl", runID))

	return &Logger{
		dir:      logsDir,
		filePath: filePath,
		runID:    runID,
	}, nil
}

// ensureFile creates the log file on first use
func (l *Logger) ensureFile() error {
	if l.file != nil {
		return nil
	}
	file, err := os.Create(l.filePath)
	if err != nil {
		return fmt.Errorf("creating log file: %w", err)
	}
	l.file = file
	l.encoder = json.NewEncoder(file)
	return nil
}

// LogIteration writes an iteration result to the log
func (l *Logger) LogIteration(log *IterationLog) error {
	if l == nil {
		return nil
	}
	if err := l.ensureFile(); err != nil {
		return err
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
	if l == nil {
		return ""
	}
	return l.filePath
}

// RunStats holds aggregate statistics from log files
type RunStats struct {
	Total    int
	Failed   int
	Succeeded int
}

// FailureRate returns the failure rate as a float64 (0.0-1.0)
func (s RunStats) FailureRate() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Failed) / float64(s.Total)
}

// BeadStats holds per-bead statistics
type BeadStats struct {
	BeadID      string
	BeadTitle   string
	TotalRuns   int
	Failures    int
	Successes   int
	LastAttempt time.Time
	Status      string
	CloseReason string
	Comments    []string
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
func (s *BeadStats) normalizeNilFields() {
	if s == nil {
		return
	}
	if s.Comments == nil {
		s.Comments = []string{}
	}
}

// FailureRate returns the failure rate for this bead as a float64 (0.0-1.0)
func (s BeadStats) FailureRate() float64 {
	if s.TotalRuns == 0 {
		return 0
	}
	return float64(s.Failures) / float64(s.TotalRuns)
}

// ReadAllLogs reads all JSONL log files in the directory and returns aggregate stats
func ReadAllLogs(logsDir string) (RunStats, error) {
	var stats RunStats

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return stats, fmt.Errorf("globbing log files: %w", err)
	}

	for _, f := range files {
		entries, err := readLogFile(f)
		if err != nil {
			continue // Skip unreadable files
		}
		for _, entry := range entries {
			stats.Total++
			if entry.Success {
				stats.Succeeded++
			} else {
				stats.Failed++
			}
		}
	}

	return stats, nil
}

// ReadPerBeadStats reads all JSONL log files and returns per-bead statistics
func ReadPerBeadStats(logsDir string) (map[string]BeadStats, error) {
	beadMap := make(map[string]BeadStats)

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return beadMap, fmt.Errorf("globbing log files: %w", err)
	}

	for _, f := range files {
		entries, err := readLogFile(f)
		if err != nil {
			continue // Skip unreadable files
		}
		for _, entry := range entries {
			stats := beadMap[entry.BeadID]

			// Initialize with bead ID and title if this is the first time we see it
			if stats.BeadID == "" {
				stats.BeadID = entry.BeadID
				stats.BeadTitle = entry.BeadTitle
			}

			stats.TotalRuns++
			if entry.Success {
				stats.Successes++
			} else {
				stats.Failures++
			}

			// Update last attempt time if this entry is more recent
			if entry.Timestamp.After(stats.LastAttempt) {
				stats.LastAttempt = entry.Timestamp
			}

			stats.normalizeNilFields()
			beadMap[entry.BeadID] = stats
		}
	}

	return beadMap, nil
}

// WriteValidationLog saves full validation output to a dedicated log file.
// Returns the path to the written log file.
func WriteValidationLog(logsDir string, output string) (string, error) {
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return "", fmt.Errorf("creating logs directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(logsDir, fmt.Sprintf("validation-%s.log", timestamp))

	if err := os.WriteFile(filename, []byte(output), 0644); err != nil {
		return "", fmt.Errorf("writing validation log: %w", err)
	}

	return filename, nil
}

func readLogFile(path string) ([]IterationLog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	entries := []IterationLog{}
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var entry IterationLog
		if err := dec.Decode(&entry); err != nil {
			break
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
