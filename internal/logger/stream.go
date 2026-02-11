package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StreamEvent represents a parsed event from Claude's stream-json output
type StreamEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`

	// For assistant messages with tool calls
	Message *StreamMessage `json:"message,omitempty"`

	// For tool results
	ToolUseResult *ToolUseResult `json:"tool_use_result,omitempty"`

	// For result events
	Result       string  `json:"result,omitempty"`
	IsError      bool    `json:"is_error,omitempty"`
	TotalCost    float64 `json:"total_cost_usd,omitempty"`
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
}

// StreamMessage is an assistant or user message in stream-json
type StreamMessage struct {
	Content []ContentBlock `json:"content"`
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
// This prevents issues with downstream code that marshals to JSON (nil → "null"
// vs [] → "[]") and ensures consistent behavior.
func (m *StreamMessage) normalizeNilFields() {
	if m == nil {
		return
	}
	if m.Content == nil {
		m.Content = []ContentBlock{}
	}
}

// ContentBlock is a single block in a message's content array
type ContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// ToolUseResult holds info about a tool result
type ToolUseResult struct {
	Type string          `json:"type,omitempty"`
	File *ToolResultFile `json:"file,omitempty"`
}

// ToolResultFile holds file info from a tool result
type ToolResultFile struct {
	FilePath string `json:"filePath,omitempty"`
	NumLines int    `json:"numLines,omitempty"`
}

// StreamStats tracks activity during a Claude run
type StreamStats struct {
	mu                      sync.Mutex
	ToolCalls               int
	FilesModified           map[string]bool
	StartTime               time.Time
	LastEventTime           time.Time
	firstEventReceived      bool
	FirstEventTime          time.Time
	TotalCost               float64
	InputTokens             int
	OutputTokens            int
	StallCount              int
	StallTier               string // "initial" or "active"
	RateLimitHits           int
	lastRateLimitTime       time.Time // timestamp of most recent rate limit
	rateLimitRecoveryMs     int64     // recovery time in ms from most recent rate limit
	hasUnrecoveredRateLimit bool      // true if rate limit hit but no event after
}

// NewStreamStats creates a new StreamStats
func NewStreamStats() (*StreamStats, error) {
	now := time.Now()
	return &StreamStats{
		FilesModified: make(map[string]bool),
		StartTime:     now,
		LastEventTime: now,
	}, nil
}

// RecordToolCall increments the tool call counter
func (s *StreamStats) RecordToolCall(toolName string, filePath string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ToolCalls++
	if filePath != "" && (toolName == "Edit" || toolName == "Write" || toolName == "NotebookEdit") {
		s.FilesModified[filePath] = true
	}
}

// Snapshot returns a copy of the current stats
func (s *StreamStats) Snapshot() (toolCalls int, filesModified int, elapsed time.Duration) {
	if s == nil {
		return 0, 0, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ToolCalls, len(s.FilesModified), time.Since(s.StartTime)
}

// RecordEvent updates the last event timestamp. Called on every stream event.
func (s *StreamStats) RecordEvent() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.LastEventTime = now
	if !s.firstEventReceived {
		s.FirstEventTime = now
	}
	s.firstEventReceived = true

	// Calculate recovery time if there was a rate limit
	if s.hasUnrecoveredRateLimit && !s.lastRateLimitTime.IsZero() {
		s.rateLimitRecoveryMs = now.Sub(s.lastRateLimitTime).Milliseconds()
		s.hasUnrecoveredRateLimit = false
	}
}

// TimeSinceLastEvent returns the duration since the last stream event was received.
func (s *StreamStats) TimeSinceLastEvent() time.Duration {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.LastEventTime)
}

// HasReceivedEvent returns true if at least one stream event has been recorded.
func (s *StreamStats) HasReceivedEvent() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.firstEventReceived
}

// HasToolActivity returns true if Claude has made at least one tool call,
// indicating it has started actively working (not just connected).
func (s *StreamStats) HasToolActivity() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ToolCalls > 0
}

// CostData returns the cost and token counts recorded from the result event.
func (s *StreamStats) CostData() (totalCost float64, inputTokens int, outputTokens int) {
	if s == nil {
		return 0, 0, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.TotalCost, s.InputTokens, s.OutputTokens
}

// TimeToFirstEvent returns the duration from stream start to the first event.
// Returns 0 if no events have been received.
func (s *StreamStats) TimeToFirstEvent() time.Duration {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.firstEventReceived {
		return 0
	}
	return s.FirstEventTime.Sub(s.StartTime)
}

// RecordStall records that a stall was detected, including which tier fired.
func (s *StreamStats) RecordStall(tier string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StallCount++
	s.StallTier = tier
}

// RecordRateLimitHit records that a rate limit indicator was detected in stream events.
func (s *StreamStats) RecordRateLimitHit() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RateLimitHits++
	s.lastRateLimitTime = time.Now()
	s.hasUnrecoveredRateLimit = true
}

// DiagnosticSnapshot returns all diagnostic data under one lock acquisition.
func (s *StreamStats) DiagnosticSnapshot() (stallCount int, stallTier string, timeToFirstEvent time.Duration, toolCalls int, rateLimitHits int, rateLimitRecoveryMs int64) {
	if s == nil {
		return 0, "", 0, 0, 0, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var ttfe time.Duration
	if s.firstEventReceived {
		ttfe = s.FirstEventTime.Sub(s.StartTime)
	}
	return s.StallCount, s.StallTier, ttfe, s.ToolCalls, s.RateLimitHits, s.rateLimitRecoveryMs
}

// StreamLogger writes firehose stream events to a log file
type StreamLogger struct {
	file *os.File
	path string
}

// NewStreamLogger creates a stream log file in the given directory
func NewStreamLogger(logsDir string) (*StreamLogger, error) {
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating logs directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(logsDir, fmt.Sprintf("stream-%s.log", timestamp))

	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("creating stream log file: %w", err)
	}

	return &StreamLogger{
		file: file,
		path: filename,
	}, nil
}

// Path returns the path to the stream log file
func (sl *StreamLogger) Path() string {
	if sl == nil {
		return ""
	}
	return sl.path
}

// LogEvent writes a formatted event line to the stream log
func (sl *StreamLogger) LogEvent(format string, args ...any) {
	if sl == nil || sl.file == nil {
		return
	}
	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s\n", timestamp, fmt.Sprintf(format, args...))
	sl.file.WriteString(line)
}

// Close closes the stream log file
func (sl *StreamLogger) Close() error {
	if sl == nil || sl.file == nil {
		return nil
	}
	return sl.file.Close()
}

// ParseAndLogEvent parses a JSON stream event and logs a human-readable summary.
// It updates stats and logs event details. Both sl and stats may be nil.
func ParseAndLogEvent(sl *StreamLogger, stats *StreamStats, line []byte) {
	var event StreamEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return // Skip unparseable lines
	}
	if event.Message != nil {
		event.Message.normalizeNilFields()
	}

	if stats != nil {
		stats.RecordEvent()

		// Detect rate limiting even when stream logger is nil
		if event.Type == "error" && isRateLimitEvent(event) {
			stats.RecordRateLimitHit()
		}
	}

	if sl == nil {
		return
	}

	switch event.Type {
	case "system":
		sl.LogEvent("SESSION: initialized (subtype=%s)", event.Subtype)

	case "assistant":
		if event.Message == nil {
			return
		}
		for _, block := range event.Message.Content {
			switch block.Type {
			case "tool_use":
				filePath := extractFilePath(block.Input)
				if stats != nil {
					stats.RecordToolCall(block.Name, filePath)
				}
				if filePath != "" {
					sl.LogEvent("TOOL_CALL: %s %s", block.Name, filePath)
				} else {
					sl.LogEvent("TOOL_CALL: %s", block.Name)
				}
			case "text":
				// Log a truncated version of the text
				text := block.Text
				if len(text) > 120 {
					text = text[:120] + "..."
				}
				sl.LogEvent("TEXT: %s", text)
			}
		}

	case "user":
		// Tool results
		if event.ToolUseResult != nil {
			if event.ToolUseResult.File != nil {
				sl.LogEvent("TOOL_RESULT: %d lines read from %s",
					event.ToolUseResult.File.NumLines,
					event.ToolUseResult.File.FilePath)
			} else {
				sl.LogEvent("TOOL_RESULT: %s", event.ToolUseResult.Type)
			}
		} else {
			sl.LogEvent("TOOL_RESULT: completed")
		}

	case "result":
		if stats != nil {
			stats.mu.Lock()
			stats.TotalCost = event.TotalCost
			stats.InputTokens = event.InputTokens
			stats.OutputTokens = event.OutputTokens
			stats.mu.Unlock()
		}
		sl.LogEvent("RESULT: subtype=%s, cost=$%.4f", event.Subtype, event.TotalCost)

	case "error":
		if isRateLimitEvent(event) {
			sl.LogEvent("RATE_LIMIT: subtype=%s", event.Subtype)
		} else {
			sl.LogEvent("ERROR: subtype=%s", event.Subtype)
		}
	}
}

// isRateLimitEvent returns true if the stream event indicates API rate limiting.
func isRateLimitEvent(event StreamEvent) bool {
	switch event.Subtype {
	case "overloaded", "rate_limit", "rate_limited":
		return true
	}
	return false
}

// extractFilePath tries to get a file_path from tool input JSON
func extractFilePath(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var params struct {
		FilePath     string `json:"file_path"`
		Path         string `json:"path"`
		NotebookPath string `json:"notebook_path"`
		Command      string `json:"command"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ""
	}
	if params.FilePath != "" {
		return params.FilePath
	}
	if params.NotebookPath != "" {
		return params.NotebookPath
	}
	if params.Path != "" {
		return params.Path
	}
	return ""
}
