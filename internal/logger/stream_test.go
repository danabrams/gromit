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

func TestStreamStats_TimeToFirstEvent(t *testing.T) {
	stats, _ := NewStreamStats()

	// Before any events, should return 0
	if ttfe := stats.TimeToFirstEvent(); ttfe != 0 {
		t.Errorf("expected 0 before first event, got %v", ttfe)
	}

	time.Sleep(50 * time.Millisecond)
	stats.RecordEvent()

	ttfe := stats.TimeToFirstEvent()
	if ttfe < 50*time.Millisecond {
		t.Errorf("expected >= 50ms time to first event, got %v", ttfe)
	}

	// Second event should not change FirstEventTime
	time.Sleep(50 * time.Millisecond)
	stats.RecordEvent()
	ttfe2 := stats.TimeToFirstEvent()
	if ttfe2 != ttfe {
		t.Errorf("expected time to first event unchanged after second event, got %v vs %v", ttfe2, ttfe)
	}
}

func TestStreamStats_TimeToFirstEventNilSafe(t *testing.T) {
	var stats *StreamStats
	if ttfe := stats.TimeToFirstEvent(); ttfe != 0 {
		t.Errorf("expected 0 for nil stats, got %v", ttfe)
	}
}

func TestStreamStats_RecordStall(t *testing.T) {
	stats, _ := NewStreamStats()

	stats.RecordStall("initial")
	if stats.StallCount != 1 {
		t.Errorf("expected StallCount=1, got %d", stats.StallCount)
	}
	if stats.StallTier != "initial" {
		t.Errorf("expected StallTier='initial', got %q", stats.StallTier)
	}

	stats.RecordStall("active")
	if stats.StallCount != 2 {
		t.Errorf("expected StallCount=2, got %d", stats.StallCount)
	}
	if stats.StallTier != "active" {
		t.Errorf("expected StallTier='active', got %q", stats.StallTier)
	}
}

func TestStreamStats_RecordStallNilSafe(t *testing.T) {
	var stats *StreamStats
	stats.RecordStall("initial") // should not panic
}

func TestStreamStats_RecordRateLimitHit(t *testing.T) {
	stats, _ := NewStreamStats()
	stats.RecordRateLimitHit()
	stats.RecordRateLimitHit()
	if stats.RateLimitHits != 2 {
		t.Errorf("expected RateLimitHits=2, got %d", stats.RateLimitHits)
	}
}

func TestStreamStats_RecordRateLimitHitNilSafe(t *testing.T) {
	var stats *StreamStats
	stats.RecordRateLimitHit() // should not panic
}

func TestStreamStats_DiagnosticSnapshot(t *testing.T) {
	stats, _ := NewStreamStats()

	// Record some activity
	time.Sleep(50 * time.Millisecond)
	stats.RecordEvent()
	stats.RecordToolCall("Read", "/foo.go")
	stats.RecordToolCall("Edit", "/bar.go")
	stats.RecordStall("initial")
	stats.RecordStall("active")
	stats.RecordRateLimitHit()

	stallCount, stallTier, ttfe, toolCalls, rateLimitHits := stats.DiagnosticSnapshot()
	if stallCount != 2 {
		t.Errorf("expected stallCount=2, got %d", stallCount)
	}
	if stallTier != "active" {
		t.Errorf("expected stallTier='active', got %q", stallTier)
	}
	if ttfe < 50*time.Millisecond {
		t.Errorf("expected ttfe >= 50ms, got %v", ttfe)
	}
	if toolCalls != 2 {
		t.Errorf("expected toolCalls=2, got %d", toolCalls)
	}
	if rateLimitHits != 1 {
		t.Errorf("expected rateLimitHits=1, got %d", rateLimitHits)
	}
}

func TestStreamStats_DiagnosticSnapshotNilSafe(t *testing.T) {
	var stats *StreamStats
	stallCount, stallTier, ttfe, toolCalls, rateLimitHits := stats.DiagnosticSnapshot()
	if stallCount != 0 || stallTier != "" || ttfe != 0 || toolCalls != 0 || rateLimitHits != 0 {
		t.Error("expected all zeros for nil stats")
	}
}

func TestParseAndLogEvent_RateLimitDetection(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}
	stats, _ := NewStreamStats()

	// Rate limit event
	line := []byte(`{"type":"error","subtype":"overloaded"}`)
	ParseAndLogEvent(sl, stats, line)
	sl.Close()

	if stats.RateLimitHits != 1 {
		t.Errorf("expected RateLimitHits=1, got %d", stats.RateLimitHits)
	}

	content, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}
	if !strings.Contains(string(content), "RATE_LIMIT") {
		t.Errorf("expected RATE_LIMIT in log, got: %s", string(content))
	}
}

func TestParseAndLogEvent_RateLimitDetectionNilLogger(t *testing.T) {
	stats, _ := NewStreamStats()

	// Rate limit should be detected even without stream logger
	line := []byte(`{"type":"error","subtype":"rate_limit"}`)
	ParseAndLogEvent(nil, stats, line)

	if stats.RateLimitHits != 1 {
		t.Errorf("expected RateLimitHits=1 with nil logger, got %d", stats.RateLimitHits)
	}
}

func TestParseAndLogEvent_NonRateLimitError(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewStreamLogger(dir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}
	stats, _ := NewStreamStats()

	// Non-rate-limit error
	line := []byte(`{"type":"error","subtype":"invalid_request"}`)
	ParseAndLogEvent(sl, stats, line)
	sl.Close()

	if stats.RateLimitHits != 0 {
		t.Errorf("expected RateLimitHits=0 for non-rate-limit error, got %d", stats.RateLimitHits)
	}

	content, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}
	if !strings.Contains(string(content), "ERROR:") {
		t.Errorf("expected ERROR in log, got: %s", string(content))
	}
}

func TestIsRateLimitEvent(t *testing.T) {
	tests := []struct {
		subtype string
		want    bool
	}{
		{"overloaded", true},
		{"rate_limit", true},
		{"rate_limited", true},
		{"invalid_request", false},
		{"", false},
		{"timeout", false},
	}
	for _, tt := range tests {
		t.Run(tt.subtype, func(t *testing.T) {
			event := StreamEvent{Type: "error", Subtype: tt.subtype}
			if got := isRateLimitEvent(event); got != tt.want {
				t.Errorf("isRateLimitEvent(subtype=%q) = %v, want %v", tt.subtype, got, tt.want)
			}
		})
	}
}
