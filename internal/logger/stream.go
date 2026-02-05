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
	Result    string  `json:"result,omitempty"`
	IsError   bool    `json:"is_error,omitempty"`
	TotalCost float64 `json:"total_cost_usd,omitempty"`
}

// StreamMessage is an assistant or user message in stream-json
type StreamMessage struct {
	Content []ContentBlock `json:"content"`
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
	mu            sync.Mutex
	ToolCalls     int
	FilesModified map[string]bool
	StartTime     time.Time
}

// NewStreamStats creates a new StreamStats
func NewStreamStats() *StreamStats {
	return &StreamStats{
		FilesModified: make(map[string]bool),
		StartTime:     time.Now(),
	}
}

// RecordToolCall increments the tool call counter
func (s *StreamStats) RecordToolCall(toolName string, filePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ToolCalls++
	if filePath != "" && (toolName == "Edit" || toolName == "Write" || toolName == "NotebookEdit") {
		s.FilesModified[filePath] = true
	}
}

// Snapshot returns a copy of the current stats
func (s *StreamStats) Snapshot() (toolCalls int, filesModified int, elapsed time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ToolCalls, len(s.FilesModified), time.Since(s.StartTime)
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
// It returns the parsed event and updates stats.
func ParseAndLogEvent(sl *StreamLogger, stats *StreamStats, line []byte) {
	var event StreamEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return // Skip unparseable lines
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
		sl.LogEvent("RESULT: subtype=%s, cost=$%.4f", event.Subtype, event.TotalCost)
	}
}

// extractFilePath tries to get a file_path from tool input JSON
func extractFilePath(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var params struct {
		FilePath    string `json:"file_path"`
		Path        string `json:"path"`
		NotebookPath string `json:"notebook_path"`
		Command     string `json:"command"`
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
