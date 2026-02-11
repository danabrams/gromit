package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeTimeouts_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts on empty dir: %v", err)
	}

	// Should return zero-value TimeoutAnalysis
	if analysis.TotalIterations != 0 {
		t.Errorf("expected 0 total iterations, got %d", analysis.TotalIterations)
	}
	if analysis.TotalTimeouts != 0 {
		t.Errorf("expected 0 total timeouts, got %d", analysis.TotalTimeouts)
	}
	if len(analysis.ByModel) != 0 {
		t.Errorf("expected empty ByModel map, got %d entries", len(analysis.ByModel))
	}
}

func TestAnalyzeTimeouts_NonexistentDirectory(t *testing.T) {
	dir := "/nonexistent/path/that/does/not/exist"

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts on nonexistent dir: %v", err)
	}

	// Should return zero-value TimeoutAnalysis without error
	if analysis.TotalIterations != 0 {
		t.Errorf("expected 0 total iterations, got %d", analysis.TotalIterations)
	}
}

func TestAnalyzeTimeouts_SingleModelSingleTimeout(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":120000,"timeout_type":"stall","time_to_first_event_ms":5000,"tool_call_count":3,"stall_count":1,"stall_tier":"initial","rate_limit_hits":0,"error":"stall timeout"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts: %v", err)
	}

	if analysis.TotalIterations != 1 {
		t.Errorf("expected 1 total iteration, got %d", analysis.TotalIterations)
	}
	if analysis.TotalTimeouts != 1 {
		t.Errorf("expected 1 total timeout, got %d", analysis.TotalTimeouts)
	}

	// Expected failure: ModelTimeoutStats struct does not exist yet
	sonnetStats, exists := analysis.ByModel["sonnet"]
	if !exists {
		t.Fatal("expected sonnet model in ByModel map")
	}

	if sonnetStats.TotalIterations != 1 {
		t.Errorf("sonnet: expected 1 total iteration, got %d", sonnetStats.TotalIterations)
	}
	if sonnetStats.TimeoutCount != 1 {
		t.Errorf("sonnet: expected 1 timeout, got %d", sonnetStats.TimeoutCount)
	}
	if sonnetStats.StallTimeouts != 1 {
		t.Errorf("sonnet: expected 1 stall timeout, got %d", sonnetStats.StallTimeouts)
	}
	if sonnetStats.BeadTimeouts != 0 {
		t.Errorf("sonnet: expected 0 bead timeouts, got %d", sonnetStats.BeadTimeouts)
	}
	if sonnetStats.InvocationTimeouts != 0 {
		t.Errorf("sonnet: expected 0 invocation timeouts, got %d", sonnetStats.InvocationTimeouts)
	}
	if sonnetStats.AvgTimeToFirstEventMs != 5000 {
		t.Errorf("sonnet: expected avg time-to-first-event 5000ms, got %d", sonnetStats.AvgTimeToFirstEventMs)
	}
	if sonnetStats.AvgToolCallCount != 3 {
		t.Errorf("sonnet: expected avg tool call count 3, got %d", sonnetStats.AvgToolCallCount)
	}
	if sonnetStats.RateLimitCorrelation != 0 {
		t.Errorf("sonnet: expected rate limit correlation 0, got %d", sonnetStats.RateLimitCorrelation)
	}
}

func TestAnalyzeTimeouts_MultipleModels(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":120000,"timeout_type":"stall","time_to_first_event_ms":5000,"tool_call_count":3,"stall_count":1,"stall_tier":"initial","rate_limit_hits":0,"error":"stall timeout"}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":60000,"time_to_first_event_ms":2000,"tool_call_count":5}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"opus","success":false,"validated":false,"escalated":false,"duration_ms":600000,"timeout_type":"invocation","time_to_first_event_ms":10000,"tool_call_count":8,"rate_limit_hits":2,"error":"invocation timeout"}
{"timestamp":"2026-02-05T12:03:00Z","iteration":4,"bead_id":"b4","bead_title":"Task 4","model":"haiku","success":true,"validated":true,"escalated":false,"duration_ms":15000,"time_to_first_event_ms":1000,"tool_call_count":2}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts: %v", err)
	}

	if analysis.TotalIterations != 4 {
		t.Errorf("expected 4 total iterations, got %d", analysis.TotalIterations)
	}
	if analysis.TotalTimeouts != 2 {
		t.Errorf("expected 2 total timeouts, got %d", analysis.TotalTimeouts)
	}
	if len(analysis.ByModel) != 3 {
		t.Errorf("expected 3 models, got %d", len(analysis.ByModel))
	}

	sonnetStats := analysis.ByModel["sonnet"]
	if sonnetStats.TotalIterations != 2 {
		t.Errorf("sonnet: expected 2 iterations, got %d", sonnetStats.TotalIterations)
	}
	if sonnetStats.TimeoutCount != 1 {
		t.Errorf("sonnet: expected 1 timeout, got %d", sonnetStats.TimeoutCount)
	}
	if sonnetStats.AvgTimeToFirstEventMs != 3500 {
		t.Errorf("sonnet: expected avg time-to-first-event 3500ms, got %d", sonnetStats.AvgTimeToFirstEventMs)
	}
	if sonnetStats.AvgToolCallCount != 4 {
		t.Errorf("sonnet: expected avg tool call count 4, got %d", sonnetStats.AvgToolCallCount)
	}

	opusStats := analysis.ByModel["opus"]
	if opusStats.TotalIterations != 1 {
		t.Errorf("opus: expected 1 iteration, got %d", opusStats.TotalIterations)
	}
	if opusStats.TimeoutCount != 1 {
		t.Errorf("opus: expected 1 timeout, got %d", opusStats.TimeoutCount)
	}
	if opusStats.InvocationTimeouts != 1 {
		t.Errorf("opus: expected 1 invocation timeout, got %d", opusStats.InvocationTimeouts)
	}
	if opusStats.RateLimitCorrelation != 1 {
		t.Errorf("opus: expected 1 rate limit correlation, got %d", opusStats.RateLimitCorrelation)
	}

	haikuStats := analysis.ByModel["haiku"]
	if haikuStats.TotalIterations != 1 {
		t.Errorf("haiku: expected 1 iteration, got %d", haikuStats.TotalIterations)
	}
	if haikuStats.TimeoutCount != 0 {
		t.Errorf("haiku: expected 0 timeouts, got %d", haikuStats.TimeoutCount)
	}
}

func TestAnalyzeTimeouts_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	log1 := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":120000,"timeout_type":"stall","time_to_first_event_ms":5000,"tool_call_count":3,"stall_count":1,"stall_tier":"initial","rate_limit_hits":0,"error":"stall timeout"}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":60000,"time_to_first_event_ms":2000,"tool_call_count":5}
`
	log2 := `{"timestamp":"2026-02-05T13:00:00Z","iteration":1,"bead_id":"b3","bead_title":"Task 3","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":300000,"timeout_type":"stall","time_to_first_event_ms":8000,"tool_call_count":2,"stall_count":2,"stall_tier":"active","rate_limit_hits":1,"error":"stall timeout"}
`

	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(log1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-130000.jsonl"), []byte(log2), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts: %v", err)
	}

	if analysis.TotalIterations != 3 {
		t.Errorf("expected 3 total iterations, got %d", analysis.TotalIterations)
	}
	if analysis.TotalTimeouts != 2 {
		t.Errorf("expected 2 total timeouts, got %d", analysis.TotalTimeouts)
	}

	sonnetStats := analysis.ByModel["sonnet"]
	if sonnetStats.TotalIterations != 3 {
		t.Errorf("sonnet: expected 3 iterations across both files, got %d", sonnetStats.TotalIterations)
	}
	if sonnetStats.TimeoutCount != 2 {
		t.Errorf("sonnet: expected 2 timeouts across both files, got %d", sonnetStats.TimeoutCount)
	}
	if sonnetStats.RateLimitCorrelation != 1 {
		t.Errorf("sonnet: expected 1 rate limit correlation, got %d", sonnetStats.RateLimitCorrelation)
	}
}

func TestAnalyzeTimeouts_MalformedJSONSkipped(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":120000,"timeout_type":"stall","time_to_first_event_ms":5000,"tool_call_count":3,"stall_count":1,"stall_tier":"initial","rate_limit_hits":0,"error":"stall timeout"}
this is not valid json and should be skipped
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":60000,"time_to_first_event_ms":2000,"tool_call_count":5}
incomplete json line without closing brace
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"opus","success":false,"validated":false,"escalated":false,"duration_ms":600000,"timeout_type":"invocation","time_to_first_event_ms":10000,"tool_call_count":8,"rate_limit_hits":2,"error":"invocation timeout"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts should not error on malformed JSON: %v", err)
	}

	if analysis.TotalIterations != 1 {
		t.Errorf("expected 1 iteration (stops at first malformed line), got %d", analysis.TotalIterations)
	}
}

func TestAnalyzeTimeouts_TimeoutTypeBreakdown(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":120000,"timeout_type":"stall","time_to_first_event_ms":5000,"tool_call_count":3,"stall_count":1,"stall_tier":"initial","rate_limit_hits":0,"error":"stall timeout"}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":1200000,"timeout_type":"bead","time_to_first_event_ms":2000,"tool_call_count":15,"error":"bead timeout"}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":600000,"timeout_type":"invocation","time_to_first_event_ms":3000,"tool_call_count":7,"error":"invocation timeout"}
{"timestamp":"2026-02-05T12:03:00Z","iteration":4,"bead_id":"b4","bead_title":"Task 4","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":300000,"timeout_type":"stall","time_to_first_event_ms":10000,"tool_call_count":2,"stall_count":3,"stall_tier":"active","rate_limit_hits":0,"error":"stall timeout"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts: %v", err)
	}

	sonnetStats := analysis.ByModel["sonnet"]
	if sonnetStats.StallTimeouts != 2 {
		t.Errorf("sonnet: expected 2 stall timeouts, got %d", sonnetStats.StallTimeouts)
	}
	if sonnetStats.BeadTimeouts != 1 {
		t.Errorf("sonnet: expected 1 bead timeout, got %d", sonnetStats.BeadTimeouts)
	}
	if sonnetStats.InvocationTimeouts != 1 {
		t.Errorf("sonnet: expected 1 invocation timeout, got %d", sonnetStats.InvocationTimeouts)
	}
	if sonnetStats.TimeoutCount != 4 {
		t.Errorf("sonnet: expected 4 total timeouts, got %d", sonnetStats.TimeoutCount)
	}
}

func TestAnalyzeTimeouts_ZeroToolCallTimeouts(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":120000,"timeout_type":"stall","time_to_first_event_ms":120000,"tool_call_count":0,"stall_count":1,"stall_tier":"initial","rate_limit_hits":0,"error":"stall timeout"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts: %v", err)
	}

	sonnetStats := analysis.ByModel["sonnet"]
	if sonnetStats.AvgToolCallCount != 0 {
		t.Errorf("sonnet: expected avg tool call count 0, got %d", sonnetStats.AvgToolCallCount)
	}
	if sonnetStats.AvgTimeToFirstEventMs != 120000 {
		t.Errorf("sonnet: expected avg time-to-first-event 120000ms, got %d", sonnetStats.AvgTimeToFirstEventMs)
	}
}

func TestAnalyzeTimeouts_OnlySuccesses(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":60000,"time_to_first_event_ms":2000,"tool_call_count":5}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":45000,"time_to_first_event_ms":1500,"tool_call_count":4}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts: %v", err)
	}

	if analysis.TotalIterations != 2 {
		t.Errorf("expected 2 total iterations, got %d", analysis.TotalIterations)
	}
	if analysis.TotalTimeouts != 0 {
		t.Errorf("expected 0 total timeouts, got %d", analysis.TotalTimeouts)
	}

	sonnetStats := analysis.ByModel["sonnet"]
	if sonnetStats.TotalIterations != 2 {
		t.Errorf("sonnet: expected 2 iterations, got %d", sonnetStats.TotalIterations)
	}
	if sonnetStats.TimeoutCount != 0 {
		t.Errorf("sonnet: expected 0 timeouts, got %d", sonnetStats.TimeoutCount)
	}
	if sonnetStats.StallTimeouts != 0 {
		t.Errorf("sonnet: expected 0 stall timeouts, got %d", sonnetStats.StallTimeouts)
	}
	if sonnetStats.AvgTimeToFirstEventMs != 1750 {
		t.Errorf("sonnet: expected avg time-to-first-event 1750ms, got %d", sonnetStats.AvgTimeToFirstEventMs)
	}
}

func TestAnalyzeTimeouts_SingleIterationLog(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"haiku","success":false,"validated":false,"escalated":false,"duration_ms":120000,"timeout_type":"stall","time_to_first_event_ms":5000,"tool_call_count":1,"stall_count":1,"stall_tier":"initial","rate_limit_hits":0,"error":"stall timeout"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts: %v", err)
	}

	if analysis.TotalIterations != 1 {
		t.Errorf("expected 1 total iteration, got %d", analysis.TotalIterations)
	}
	if analysis.TotalTimeouts != 1 {
		t.Errorf("expected 1 total timeout, got %d", analysis.TotalTimeouts)
	}

	haikuStats := analysis.ByModel["haiku"]
	if haikuStats.TotalIterations != 1 {
		t.Errorf("haiku: expected 1 iteration, got %d", haikuStats.TotalIterations)
	}
	if haikuStats.TimeoutCount != 1 {
		t.Errorf("haiku: expected 1 timeout, got %d", haikuStats.TimeoutCount)
	}
}

func TestAnalyzeTimeouts_RateLimitWithoutRecovery(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"opus","success":false,"validated":false,"escalated":false,"duration_ms":600000,"timeout_type":"invocation","time_to_first_event_ms":30000,"tool_call_count":2,"rate_limit_hits":5,"error":"invocation timeout"}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts: %v", err)
	}

	opusStats := analysis.ByModel["opus"]
	if opusStats.RateLimitCorrelation != 1 {
		t.Errorf("opus: expected 1 rate limit correlation (timeout with rate_limit_hits > 0), got %d", opusStats.RateLimitCorrelation)
	}
	if opusStats.TimeoutCount != 1 {
		t.Errorf("opus: expected 1 timeout, got %d", opusStats.TimeoutCount)
	}
}

func TestAnalyzeTimeouts_MissingDiagnosticFields(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":120000,"error":"timeout"}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":60000}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts: %v", err)
	}

	if analysis.TotalIterations != 2 {
		t.Errorf("expected 2 total iterations, got %d", analysis.TotalIterations)
	}

	sonnetStats := analysis.ByModel["sonnet"]
	if sonnetStats.TotalIterations != 2 {
		t.Errorf("sonnet: expected 2 iterations, got %d", sonnetStats.TotalIterations)
	}
	if sonnetStats.TimeoutCount > 2 {
		t.Errorf("sonnet: timeout count should not exceed total iterations, got %d", sonnetStats.TimeoutCount)
	}
}

func TestAnalyzeTimeouts_AverageCalculations(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":false,"validated":false,"escalated":false,"duration_ms":120000,"timeout_type":"stall","time_to_first_event_ms":1000,"tool_call_count":2,"stall_count":1,"stall_tier":"initial","rate_limit_hits":0,"error":"stall timeout"}
{"timestamp":"2026-02-05T12:01:00Z","iteration":2,"bead_id":"b2","bead_title":"Task 2","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":60000,"time_to_first_event_ms":2000,"tool_call_count":4}
{"timestamp":"2026-02-05T12:02:00Z","iteration":3,"bead_id":"b3","bead_title":"Task 3","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":45000,"time_to_first_event_ms":3000,"tool_call_count":6}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts: %v", err)
	}

	sonnetStats := analysis.ByModel["sonnet"]
	if sonnetStats.AvgTimeToFirstEventMs != 2000 {
		t.Errorf("sonnet: expected avg time-to-first-event 2000ms, got %d", sonnetStats.AvgTimeToFirstEventMs)
	}
	if sonnetStats.AvgToolCallCount != 4 {
		t.Errorf("sonnet: expected avg tool call count 4, got %d", sonnetStats.AvgToolCallCount)
	}
}

func TestAnalyzeTimeouts_IgnoresNonLogFiles(t *testing.T) {
	dir := t.TempDir()

	logContent := `{"timestamp":"2026-02-05T12:00:00Z","iteration":1,"bead_id":"b1","bead_title":"Task 1","model":"sonnet","success":true,"validated":true,"escalated":false,"duration_ms":60000,"time_to_first_event_ms":2000,"tool_call_count":5}
`
	if err := os.WriteFile(filepath.Join(dir, "run-20260205-120000.jsonl"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "validation-20260205-120000.log"), []byte("some validation output"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other-file.txt"), []byte("some content"), 0644); err != nil {
		t.Fatal(err)
	}

	analysis, err := AnalyzeTimeouts(dir)
	if err != nil {
		t.Fatalf("AnalyzeTimeouts: %v", err)
	}

	if analysis.TotalIterations != 1 {
		t.Errorf("expected 1 iteration (only from run-*.jsonl), got %d", analysis.TotalIterations)
	}
}
