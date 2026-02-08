package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewStreamLogger(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}
	defer sl.Close()

	// Verify file was created with correct prefix
	base := filepath.Base(sl.Path())
	if !strings.HasPrefix(base, "stream-") {
		t.Errorf("expected filename starting with 'stream-', got %s", base)
	}
	if filepath.Ext(sl.Path()) != ".log" {
		t.Errorf("expected .log extension, got %s", filepath.Ext(sl.Path()))
	}

	// Verify file exists
	if _, err := os.Stat(sl.Path()); err != nil {
		t.Errorf("stream log file does not exist: %v", err)
	}
}

func TestNewStreamLoggerCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "logs")
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}
	defer sl.Close()

	if _, err := os.Stat(sl.Path()); err != nil {
		t.Errorf("stream log file does not exist: %v", err)
	}
}

func TestStreamLoggerLogEvent(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}

	sl.LogEvent("TOOL_CALL: %s %s", "Read", "/some/file.go")
	sl.LogEvent("TOOL_RESULT: %d lines read", 42)
	sl.Close()

	content, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "TOOL_CALL: Read /some/file.go") {
		t.Errorf("line 0 missing expected content: %s", lines[0])
	}
	if !strings.Contains(lines[1], "TOOL_RESULT: 42 lines read") {
		t.Errorf("line 1 missing expected content: %s", lines[1])
	}

	// Check timestamp format [HH:MM:SS]
	if !strings.HasPrefix(lines[0], "[") {
		t.Errorf("expected line to start with timestamp bracket: %s", lines[0])
	}
}

func TestStreamLoggerNilSafe(t *testing.T) {
	var sl *StreamLogger
	// These should not panic
	sl.LogEvent("test %s", "message")
	sl.Close()
	if sl.Path() != "" {
		t.Errorf("expected empty path for nil logger, got %s", sl.Path())
	}
}

func TestStreamStats(t *testing.T) {
	stats, _ := NewStreamStats()

	stats.RecordToolCall("Read", "/foo/bar.go")
	stats.RecordToolCall("Edit", "/foo/bar.go")
	stats.RecordToolCall("Write", "/foo/baz.go")
	stats.RecordToolCall("Bash", "")

	toolCalls, filesModified, elapsed := stats.Snapshot()
	if toolCalls != 4 {
		t.Errorf("expected 4 tool calls, got %d", toolCalls)
	}
	if filesModified != 2 {
		t.Errorf("expected 2 files modified, got %d", filesModified)
	}
	if elapsed < 0 {
		t.Errorf("elapsed time should be non-negative")
	}
}

func TestStreamStatsOnlyCountsWriteTools(t *testing.T) {
	stats, _ := NewStreamStats()

	// Read and Bash should not count as file modifications
	stats.RecordToolCall("Read", "/foo/bar.go")
	stats.RecordToolCall("Bash", "")
	stats.RecordToolCall("Grep", "/foo/")

	_, filesModified, _ := stats.Snapshot()
	if filesModified != 0 {
		t.Errorf("expected 0 files modified for read-only tools, got %d", filesModified)
	}
}

func TestStreamStatsStartTime(t *testing.T) {
	before := time.Now()
	stats, _ := NewStreamStats()
	after := time.Now()

	if stats.StartTime.Before(before) || stats.StartTime.After(after) {
		t.Errorf("StartTime should be between before and after")
	}
}

func TestStreamStatsRecordEvent(t *testing.T) {
	stats, _ := NewStreamStats()
	time.Sleep(50 * time.Millisecond)
	stats.RecordEvent()

	since := stats.TimeSinceLastEvent()
	if since > 50*time.Millisecond {
		t.Errorf("TimeSinceLastEvent should be < 50ms after RecordEvent, got %v", since)
	}
}

func TestStreamStatsTimeSinceLastEventInitial(t *testing.T) {
	stats, _ := NewStreamStats()
	time.Sleep(100 * time.Millisecond)

	since := stats.TimeSinceLastEvent()
	if since < 100*time.Millisecond {
		t.Errorf("TimeSinceLastEvent should be >= 100ms, got %v", since)
	}
}

func TestParseAndLogEventUpdatesLastEventTime(t *testing.T) {
	stats, _ := NewStreamStats()
	time.Sleep(50 * time.Millisecond)

	line := []byte(`{"type":"system","subtype":"init"}`)
	ParseAndLogEvent(nil, stats, line)

	since := stats.TimeSinceLastEvent()
	if since > 50*time.Millisecond {
		t.Errorf("Expected LastEventTime updated by ParseAndLogEvent, got since=%v", since)
	}
}

func TestParseAndLogEventInvalidJSONNoUpdate(t *testing.T) {
	stats, _ := NewStreamStats()
	time.Sleep(50 * time.Millisecond)

	// Invalid JSON should not update LastEventTime
	ParseAndLogEvent(nil, stats, []byte("not json"))

	since := stats.TimeSinceLastEvent()
	if since < 50*time.Millisecond {
		t.Errorf("Invalid JSON should not update LastEventTime, got since=%v", since)
	}
}

func TestParseAndLogEventToolCall(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}
	stats, _ := NewStreamStats()

	// Simulate an assistant message with a tool_use
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/internal/runner/runner.go"}}]}}`)
	ParseAndLogEvent(sl, stats, line)
	sl.Close()

	content, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}

	if !strings.Contains(string(content), "TOOL_CALL: Read /internal/runner/runner.go") {
		t.Errorf("expected TOOL_CALL log entry, got: %s", string(content))
	}

	toolCalls, _, _ := stats.Snapshot()
	if toolCalls != 1 {
		t.Errorf("expected 1 tool call, got %d", toolCalls)
	}
}

func TestParseAndLogEventToolResult(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}
	stats, _ := NewStreamStats()

	line := []byte(`{"type":"user","tool_use_result":{"type":"text","file":{"filePath":"/foo/bar.go","numLines":42}}}`)
	ParseAndLogEvent(sl, stats, line)
	sl.Close()

	content, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}

	if !strings.Contains(string(content), "TOOL_RESULT: 42 lines read from /foo/bar.go") {
		t.Errorf("expected TOOL_RESULT log entry, got: %s", string(content))
	}
}

func TestParseAndLogEventResult(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}
	stats, _ := NewStreamStats()

	line := []byte(`{"type":"result","subtype":"success","total_cost_usd":0.0123}`)
	ParseAndLogEvent(sl, stats, line)
	sl.Close()

	content, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}

	if !strings.Contains(string(content), "RESULT: subtype=success, cost=$0.0123") {
		t.Errorf("expected RESULT log entry, got: %s", string(content))
	}
}

func TestParseAndLogEventEditTracksModifiedFiles(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}
	stats, _ := NewStreamStats()

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/foo/bar.go","old_string":"x","new_string":"y"}}]}}`)
	ParseAndLogEvent(sl, stats, line)
	sl.Close()

	_, filesModified, _ := stats.Snapshot()
	if filesModified != 1 {
		t.Errorf("expected 1 file modified, got %d", filesModified)
	}
}

func TestParseAndLogEventInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}
	stats, _ := NewStreamStats()

	// Should not panic on invalid JSON
	ParseAndLogEvent(sl, stats, []byte("not json"))
	sl.Close()

	content, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}
	// File should be empty - invalid JSON is skipped
	if len(strings.TrimSpace(string(content))) != 0 {
		t.Errorf("expected empty file for invalid JSON, got: %s", string(content))
	}
}

func TestParseAndLogEventNilStats(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}

	// Should not panic when stats is nil
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/foo.go"}}]}}`)
	ParseAndLogEvent(sl, nil, line)
	sl.Close()

	content, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}

	// Should still log the event even with nil stats
	if !strings.Contains(string(content), "TOOL_CALL: Read /foo.go") {
		t.Errorf("expected TOOL_CALL log entry with nil stats, got: %s", string(content))
	}
}

func TestParseAndLogEventNilStatsAndLogger(t *testing.T) {
	// Should not panic when both stats and logger are nil
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/foo.go"}}]}}`)
	ParseAndLogEvent(nil, nil, line)
}

func TestStreamMessageNormalizeNilFields(t *testing.T) {
	m := &StreamMessage{}
	m.normalizeNilFields()

	if m.Content == nil {
		t.Error("Expected Content to be non-nil after normalization")
	}
	if len(m.Content) != 0 {
		t.Errorf("Expected empty Content, got %d", len(m.Content))
	}
}

func TestStreamMessageNormalizeNilFieldsPreservesExisting(t *testing.T) {
	m := &StreamMessage{
		Content: []ContentBlock{{Type: "text", Text: "hello"}},
	}
	m.normalizeNilFields()

	if len(m.Content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(m.Content))
	}
	if m.Content[0].Text != "hello" {
		t.Errorf("Expected 'hello', got %q", m.Content[0].Text)
	}
}

func TestStreamMessageNormalizeNilFieldsOnNilReceiver(t *testing.T) {
	var m *StreamMessage
	m.normalizeNilFields() // should not panic
}

func TestParseAndLogEventNormalizesMessageContent(t *testing.T) {
	stats, _ := NewStreamStats()

	// JSON with message but null content
	line := []byte(`{"type":"assistant","message":{"content":null}}`)
	ParseAndLogEvent(nil, stats, line)

	// Should not panic - the nil Content is normalized before iteration
}

func TestParseAndLogEventNilLoggerUpdatesStats(t *testing.T) {
	// When logger is nil, stats should still be updated
	stats, _ := NewStreamStats()
	time.Sleep(50 * time.Millisecond)

	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/foo.go"}}]}}`)
	ParseAndLogEvent(nil, stats, line)

	if !stats.HasReceivedEvent() {
		t.Error("expected stats to record event even with nil logger")
	}
	since := stats.TimeSinceLastEvent()
	if since > 50*time.Millisecond {
		t.Errorf("expected LastEventTime updated, got since=%v", since)
	}
}

func TestParseAndLogEventTextTruncation(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}
	stats, _ := NewStreamStats()

	// Create a long text block
	longText := strings.Repeat("a", 200)
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"` + longText + `"}]}}`)
	ParseAndLogEvent(sl, stats, line)
	sl.Close()

	content, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}

	if !strings.Contains(string(content), "...") {
		t.Errorf("expected truncated text with '...', got: %s", string(content))
	}
}

func TestStreamEventTokenFields(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"success","total_cost_usd":0.0123,"input_tokens":1000,"output_tokens":500}`)

	var event StreamEvent
	if err := json.Unmarshal(line, &event); err != nil {
		t.Fatalf("unmarshaling event: %v", err)
	}

	if event.TotalCost != 0.0123 {
		t.Errorf("expected TotalCost=0.0123, got %f", event.TotalCost)
	}
	if event.InputTokens != 1000 {
		t.Errorf("expected InputTokens=1000, got %d", event.InputTokens)
	}
	if event.OutputTokens != 500 {
		t.Errorf("expected OutputTokens=500, got %d", event.OutputTokens)
	}
}

func TestStreamStatsTokenFields(t *testing.T) {
	stats, _ := NewStreamStats()

	// Fields should be initialized to zero
	if stats.TotalCost != 0 {
		t.Errorf("expected TotalCost=0, got %f", stats.TotalCost)
	}
	if stats.InputTokens != 0 {
		t.Errorf("expected InputTokens=0, got %d", stats.InputTokens)
	}
	if stats.OutputTokens != 0 {
		t.Errorf("expected OutputTokens=0, got %d", stats.OutputTokens)
	}

	// Set values
	stats.TotalCost = 0.0123
	stats.InputTokens = 1000
	stats.OutputTokens = 500

	if stats.TotalCost != 0.0123 {
		t.Errorf("expected TotalCost=0.0123, got %f", stats.TotalCost)
	}
	if stats.InputTokens != 1000 {
		t.Errorf("expected InputTokens=1000, got %d", stats.InputTokens)
	}
	if stats.OutputTokens != 500 {
		t.Errorf("expected OutputTokens=500, got %d", stats.OutputTokens)
	}
}

func TestParseAndLogEventCapturesCostData(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}
	stats, _ := NewStreamStats()

	line := []byte(`{"type":"result","subtype":"success","total_cost_usd":0.0456,"input_tokens":2000,"output_tokens":750}`)
	ParseAndLogEvent(sl, stats, line)
	sl.Close()

	cost, inputTokens, outputTokens := stats.CostData()
	if cost != 0.0456 {
		t.Errorf("expected cost=0.0456, got %f", cost)
	}
	if inputTokens != 2000 {
		t.Errorf("expected inputTokens=2000, got %d", inputTokens)
	}
	if outputTokens != 750 {
		t.Errorf("expected outputTokens=750, got %d", outputTokens)
	}
}

func TestParseAndLogEventCostDataNilStats(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}

	// Should not panic when stats is nil
	line := []byte(`{"type":"result","subtype":"success","total_cost_usd":0.0456,"input_tokens":2000,"output_tokens":750}`)
	ParseAndLogEvent(sl, nil, line)
	sl.Close()

	content, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}

	// Should still log the event even with nil stats
	if !strings.Contains(string(content), "RESULT: subtype=success, cost=$0.0456") {
		t.Errorf("expected RESULT log entry with nil stats, got: %s", string(content))
	}
}

func TestStreamStatsCostDataNilSafe(t *testing.T) {
	var stats *StreamStats
	cost, inputTokens, outputTokens := stats.CostData()
	if cost != 0 || inputTokens != 0 || outputTokens != 0 {
		t.Errorf("expected zeros for nil stats, got cost=%f, inputTokens=%d, outputTokens=%d",
			cost, inputTokens, outputTokens)
	}
}
