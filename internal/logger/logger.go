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
	Timestamp               time.Time `json:"timestamp"`
	Iteration               int       `json:"iteration"`
	BeadID                  string    `json:"bead_id"`
	BeadTitle               string    `json:"bead_title"`
	SpecID                  string    `json:"spec_id,omitempty"`
	Model                   string    `json:"model"`
	Provider                string    `json:"provider,omitempty"`
	FailurePhase            string    `json:"failure_phase,omitempty"`
	FailureCategory         string    `json:"failure_category,omitempty"`
	Success                 bool      `json:"success"`
	Validated               bool      `json:"validated"`
	Escalated               bool      `json:"escalated"`
	EscalatedTo             string    `json:"escalated_to,omitempty"`
	DurationMs              int64     `json:"duration_ms"`
	CostUSD                 float64   `json:"cost_usd"`
	InputTokens             int       `json:"input_tokens"`
	OutputTokens            int       `json:"output_tokens"`
	Error                   string    `json:"error,omitempty"`
	Outcome                 string    `json:"outcome,omitempty"`
	ValidationRetried       bool      `json:"validation_retried,omitempty"`
	TrivialAutoFixed        bool      `json:"trivial_auto_fixed,omitempty"`
	UsageLimited            bool      `json:"usage_limited,omitempty"`
	ValidationMode          string    `json:"validation_mode,omitempty"`
	ValidationDurationMs    int64     `json:"validation_duration_ms,omitempty"`
	CompilationErrors       bool      `json:"compilation_errors,omitempty"`
	HardStopPendingApproval bool      `json:"hard_stop_pending_approval,omitempty"`

	// Diagnostic fields for timeout investigation
	TimeoutType         string `json:"timeout_type,omitempty"`           // "stall", "bead", "invocation", ""
	TimeoutPhase        string `json:"timeout_phase,omitempty"`          // phase active when timeout/cancel occurred
	TimeToFirstEventMs  int64  `json:"time_to_first_event_ms,omitempty"` // ms from start to first stream event
	ToolCallCount       int    `json:"tool_call_count,omitempty"`        // tool calls before completion/timeout
	StallCount          int    `json:"stall_count,omitempty"`            // number of stall detections
	StallTier           string `json:"stall_tier,omitempty"`             // "initial" or "active"
	RateLimitHits       int    `json:"rate_limit_hits,omitempty"`        // rate limit events detected
	RateLimitRecoveryMs int64  `json:"rate_limit_recovery_ms,omitempty"` // ms to recover from most recent rate limit

	AcceptanceFailureSummary  string `json:"acceptance_failure_summary,omitempty"`
	AcceptanceFailureOutput   string `json:"acceptance_failure_output,omitempty"`
	AcceptanceFailureArtifact string `json:"acceptance_failure_artifact,omitempty"`
	AcceptanceFailureExitCode int    `json:"acceptance_failure_exit_code,omitempty"`

	FailureClass     string `json:"failure_class,omitempty"`
	AndonLevel       string `json:"andon_level,omitempty"`
	TrimDecision     string `json:"trim_decision,omitempty"`
	AutonomyEligible bool   `json:"autonomy_eligible"`
	AutonomySuccess  bool   `json:"autonomy_success"`
	FirstPassSuccess bool   `json:"first_pass_success"`
	FilesTouched     int    `json:"files_touched,omitempty"`
	MTTRProxyMs      int64  `json:"mttr_proxy_ms,omitempty"`
	EscalationClass  string `json:"escalation_class,omitempty"`
	RecurrenceCount  int    `json:"recurrence_count,omitempty"`

	// Coverage tracking fields
	CriteriaTotal      int      `json:"criteria_total,omitempty"`
	CriteriaCovered    int      `json:"criteria_covered,omitempty"`
	CriteriaUntestable int      `json:"criteria_untestable,omitempty"`
	UncoveredCriteria  []string `json:"uncovered_criteria,omitempty"`
}

// ReviewLog represents a review's outcome (light or thorough)
type ReviewLog struct {
	Timestamp      time.Time `json:"timestamp"`
	Type           string    `json:"type"`
	ReviewType     string    `json:"review_type"`
	Iteration      int       `json:"iteration"`
	BeadID         string    `json:"bead_id,omitempty"`
	Model          string    `json:"model"`
	Passed         bool      `json:"passed"`
	FixesApplied   int       `json:"fixes_applied"`
	BeadsCreated   int       `json:"beads_created"`
	BacklogCreated int       `json:"backlog_created"`
	DurationMs     int64     `json:"duration_ms"`
}

// TDDPhaseRecord represents a single TDD phase (red/green/refactor) event in the JSONL log.
type TDDPhaseRecord struct {
	Type               string    `json:"type"`
	Timestamp          time.Time `json:"timestamp"`
	BeadID             string    `json:"bead_id"`
	Phase              string    `json:"phase"`
	CycleNumber        int       `json:"cycle_number"`
	Model              string    `json:"model"`
	Tier               string    `json:"tier"`
	InputTokens        int       `json:"input_tokens"`
	OutputTokens       int       `json:"output_tokens"`
	DurationMs         int64     `json:"duration_ms"`
	Success            bool      `json:"success"`
	Escalated          bool      `json:"escalated"`
	EscalatedFrom      string    `json:"escalated_from,omitempty"`
	CriteriaTotal      int       `json:"criteria_total,omitempty"`
	CriteriaCovered    int       `json:"criteria_covered,omitempty"`
	CriteriaUntestable int       `json:"criteria_untestable,omitempty"`
}

// TDDSummaryRecord represents a summary of a full TDD bead run in the JSONL log.
type TDDSummaryRecord struct {
	Type            string    `json:"type"`
	Timestamp       time.Time `json:"timestamp"`
	BeadID          string    `json:"bead_id"`
	TotalCycles     int       `json:"total_cycles"`
	TotalPhases     int       `json:"total_phases"`
	Success         bool      `json:"success"`
	TotalDurationMs int64     `json:"total_duration_ms"`
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

// LogReview writes a review result to the log
func (l *Logger) LogReview(log *ReviewLog) error {
	if l == nil {
		return nil
	}
	if err := l.ensureFile(); err != nil {
		return err
	}
	return l.encoder.Encode(log)
}

// LogTDDPhase writes a TDD phase record to the log
func (l *Logger) LogTDDPhase(rec *TDDPhaseRecord) error {
	if l == nil {
		return nil
	}
	if err := l.ensureFile(); err != nil {
		return err
	}
	return l.encoder.Encode(rec)
}

// LogTDDSummary writes a TDD summary record to the log
func (l *Logger) LogTDDSummary(rec *TDDSummaryRecord) error {
	if l == nil {
		return nil
	}
	if err := l.ensureFile(); err != nil {
		return err
	}
	return l.encoder.Encode(rec)
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
	Total     int
	Failed    int
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
	return ReadAllLogsFiltered(logsDir, nil)
}

// ReadAllLogsFiltered reads all JSONL log files and returns aggregate stats,
// optionally filtered by bead ID. If beadFilter is nil or empty, all entries are included.
func ReadAllLogsFiltered(logsDir string, beadFilter map[string]bool) (RunStats, error) {
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
			// Apply filter if provided
			if len(beadFilter) > 0 && !beadFilter[entry.BeadID] {
				continue
			}

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
	return ReadPerBeadStatsFiltered(logsDir, nil)
}

// ReadPerBeadStatsFiltered reads all JSONL log files and returns per-bead statistics,
// optionally filtered by bead ID. If beadFilter is nil or empty, all entries are included.
func ReadPerBeadStatsFiltered(logsDir string, beadFilter map[string]bool) (map[string]BeadStats, error) {
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
			// Apply filter if provided
			if len(beadFilter) > 0 && !beadFilter[entry.BeadID] {
				continue
			}

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
