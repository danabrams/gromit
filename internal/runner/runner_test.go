package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/danabrams/ralph-runner/internal/config"
	"github.com/danabrams/ralph-runner/internal/logger"
)

func TestCheckExpectedOutputs(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name            string
		expectedOutputs []string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:            "empty list returns empty string",
			expectedOutputs: []string{},
			wantNotContains: []string{"Expected outputs:"},
		},
		{
			name:            "existing file shows checkmark",
			expectedOutputs: []string{testFile},
			wantContains:    []string{"Expected outputs:", "✓", testFile, "(exists)"},
		},
		{
			name:            "missing file shows X",
			expectedOutputs: []string{filepath.Join(tmpDir, "missing.go")},
			wantContains:    []string{"Expected outputs:", "✗", "missing.go", "(not found)"},
		},
		{
			name:            "mixed existing and missing files",
			expectedOutputs: []string{testFile, filepath.Join(tmpDir, "missing.go")},
			wantContains:    []string{"Expected outputs:", "✓", testFile, "✗", "missing.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkExpectedOutputs(tt.expectedOutputs)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\nGot: %s", want, got)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("output should not contain %q\nGot: %s", notWant, got)
				}
			}
		})
	}
}

func TestCheckExpectedOutputsEmpty(t *testing.T) {
	got := checkExpectedOutputs(nil)
	if got != "" {
		t.Errorf("nil input should return empty string, got %q", got)
	}
}

func TestBeadExpectedOutputsField(t *testing.T) {
	b := &bead.Bead{
		ID:              "test-123",
		Title:           "Test Bead",
		ExpectedOutputs: []string{"file1.go", "file2.go"},
	}

	if len(b.ExpectedOutputs) != 2 {
		t.Errorf("Expected 2 outputs, got %d", len(b.ExpectedOutputs))
	}

	if b.ExpectedOutputs[0] != "file1.go" {
		t.Errorf("Expected first output to be file1.go, got %s", b.ExpectedOutputs[0])
	}
}

func TestBeadExpectedOutputsJSON(t *testing.T) {
	jsonData := `{
		"id": "abc-123",
		"title": "Add feature",
		"expected_outputs": ["internal/foo/bar.go", "internal/foo/bar_test.go"]
	}`

	var b bead.Bead
	if err := json.Unmarshal([]byte(jsonData), &b); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if b.ID != "abc-123" {
		t.Errorf("Expected ID abc-123, got %s", b.ID)
	}

	if len(b.ExpectedOutputs) != 2 {
		t.Errorf("Expected 2 outputs, got %d", len(b.ExpectedOutputs))
	}

	if b.ExpectedOutputs[0] != "internal/foo/bar.go" {
		t.Errorf("Expected first output internal/foo/bar.go, got %s", b.ExpectedOutputs[0])
	}
}

func TestBeadExpectedOutputsJSONOmitted(t *testing.T) {
	jsonData := `{"id": "abc-123", "title": "Simple task"}`

	var b bead.Bead
	if err := json.Unmarshal([]byte(jsonData), &b); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if b.ExpectedOutputs != nil {
		t.Errorf("Expected nil ExpectedOutputs when omitted, got %v", b.ExpectedOutputs)
	}
}

func TestGetGitHead(t *testing.T) {
	// This test runs in the ralph-runner repo, so git HEAD should be available
	head, err := getGitHead()
	if err != nil {
		t.Fatalf("getGitHead() failed: %v", err)
	}

	if len(head) != 40 {
		t.Errorf("Expected 40-char SHA, got %d chars: %s", len(head), head)
	}
}

func TestNewRunnerNilConfig(t *testing.T) {
	r, err := NewRunner(nil, os.Stdout)
	if r != nil {
		t.Error("expected nil Runner for nil config")
	}
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestRunNilRunner(t *testing.T) {
	var r *Runner
	err := r.Run(nil, 0, false)
	if err == nil {
		t.Error("expected error for nil runner")
	}
}

func TestStatusNilRunner(t *testing.T) {
	var r *Runner
	err := r.Status()
	if err == nil {
		t.Error("expected error for nil runner")
	}
}

func TestRunNilConfig(t *testing.T) {
	r := &Runner{output: os.Stdout}
	err := r.Run(nil, 0, false)
	if err == nil {
		t.Error("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config is nil") {
		t.Errorf("expected 'config is nil' in error, got %q", err.Error())
	}
}

func TestProcessBeadNilConfig(t *testing.T) {
	r := &Runner{output: os.Stdout}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.processBead(nil, b, 1)
	if result.Error == nil {
		t.Error("expected error for nil config in processBead")
	}
	if !strings.Contains(result.Error.Error(), "config is nil") {
		t.Errorf("expected 'config is nil' in error, got %q", result.Error.Error())
	}
}

func TestLogNilOutput(t *testing.T) {
	r := &Runner{} // output is nil
	// Should not panic
	r.log("test message %s", "value")
}

func TestSelectModelNilBead(t *testing.T) {
	r := &Runner{}
	result := r.selectModel(nil)
	if result != "sonnet" {
		t.Errorf("expected 'sonnet' for nil bead, got %q", result)
	}
}

func TestShowPartialProgressNilBead(t *testing.T) {
	var buf strings.Builder
	r := &Runner{output: &buf}
	// Should not panic with nil bead
	r.showPartialProgress(nil, "abc123")
}

func TestStartHeartbeatStallDetection(t *testing.T) {
	var buf strings.Builder
	r := &Runner{output: &buf}

	stats, _ := logger.NewStreamStats()
	// Record one event so stall detection becomes active, then let it stall
	stats.RecordEvent()

	stallFired := make(chan struct{})
	onStall := func() {
		close(stallFired)
	}

	// Use very short intervals for testing
	cfg := heartbeatConfig{
		InitialDelay:   10 * time.Millisecond,
		HeartbeatRate:  50 * time.Millisecond,
		StallCheckRate: 10 * time.Millisecond,
	}

	// No tool activity, so initial timeout (50ms) should be used
	stop := r.startHeartbeatWithConfig(stats, 50*time.Millisecond, 200*time.Millisecond, onStall, cfg, nil)
	defer stop()

	select {
	case <-stallFired:
		// Good — stall was detected
	case <-time.After(2 * time.Second):
		t.Fatal("Stall was not detected within timeout")
	}

	output := buf.String()
	if !strings.Contains(output, "STALL DETECTED") {
		t.Errorf("Expected 'STALL DETECTED' in output, got: %s", output)
	}
	if !strings.Contains(output, "initial") {
		t.Errorf("Expected 'initial' tier in stall message, got: %s", output)
	}
}

func TestStartHeartbeatActiveStallTimeout(t *testing.T) {
	var buf strings.Builder
	r := &Runner{output: &buf}

	stats, _ := logger.NewStreamStats()
	// Record an event and a tool call so HasToolActivity() returns true
	stats.RecordEvent()
	stats.RecordToolCall("Read", "/some/file.go")

	stallFired := make(chan struct{})
	onStall := func() {
		close(stallFired)
	}

	cfg := heartbeatConfig{
		InitialDelay:   10 * time.Millisecond,
		HeartbeatRate:  50 * time.Millisecond,
		StallCheckRate: 10 * time.Millisecond,
	}

	// Initial timeout is very short (20ms) but should NOT fire because tool
	// activity has occurred — the active timeout (150ms) should be used instead.
	stop := r.startHeartbeatWithConfig(stats, 20*time.Millisecond, 150*time.Millisecond, onStall, cfg, nil)
	defer stop()

	// Wait long enough for initial timeout but not active timeout
	time.Sleep(80 * time.Millisecond)

	select {
	case <-stallFired:
		t.Fatal("Stall should not fire before active timeout; initial timeout should be ignored after tool activity")
	default:
		// Good — not fired yet
	}

	// Now wait for active timeout to fire
	select {
	case <-stallFired:
		// Good — stall was detected with active timeout
	case <-time.After(2 * time.Second):
		t.Fatal("Active stall timeout was not detected")
	}

	output := buf.String()
	if !strings.Contains(output, "STALL DETECTED") {
		t.Errorf("Expected 'STALL DETECTED' in output, got: %s", output)
	}
	if !strings.Contains(output, "active") {
		t.Errorf("Expected 'active' tier in stall message, got: %s", output)
	}
}

func TestStartHeartbeatNoStallBeforeFirstEvent(t *testing.T) {
	var buf strings.Builder
	r := &Runner{output: &buf}

	stats, _ := logger.NewStreamStats()
	// Don't record any events — stall detection should NOT fire during startup

	stallFired := false
	onStall := func() {
		stallFired = true
	}

	cfg := heartbeatConfig{
		InitialDelay:   10 * time.Millisecond,
		HeartbeatRate:  20 * time.Millisecond,
		StallCheckRate: 10 * time.Millisecond,
	}

	// Stall timeout is very short (30ms), but should not fire because no events recorded
	stop := r.startHeartbeatWithConfig(stats, 30*time.Millisecond, 60*time.Millisecond, onStall, cfg, nil)
	time.Sleep(150 * time.Millisecond)
	stop()

	if stallFired {
		t.Fatal("Stall should not fire before first stream event is received")
	}
}

func TestStartHeartbeatNoStallWhenEventsFlow(t *testing.T) {
	var buf strings.Builder
	r := &Runner{output: &buf}

	stats, _ := logger.NewStreamStats()

	stallFired := make(chan struct{})
	onStall := func() {
		close(stallFired)
	}

	cfg := heartbeatConfig{
		InitialDelay:   10 * time.Millisecond,
		HeartbeatRate:  50 * time.Millisecond,
		StallCheckRate: 20 * time.Millisecond,
	}

	stop := r.startHeartbeatWithConfig(stats, 100*time.Millisecond, 200*time.Millisecond, onStall, cfg, nil)

	// Keep recording events to prevent stall
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(10 * time.Millisecond):
				stats.RecordEvent()
			}
		}
	}()

	// Wait long enough that a stall would have fired if events weren't flowing
	time.Sleep(200 * time.Millisecond)
	close(done)
	stop()

	select {
	case <-stallFired:
		t.Fatal("Stall should not fire when events are flowing")
	default:
		// Good — no stall
	}
}

func TestStartHeartbeatStallDisabledWhenZero(t *testing.T) {
	var buf strings.Builder
	r := &Runner{output: &buf}

	stats, _ := logger.NewStreamStats()

	stallFired := false
	onStall := func() {
		stallFired = true
	}

	cfg := heartbeatConfig{
		InitialDelay:   10 * time.Millisecond,
		HeartbeatRate:  20 * time.Millisecond,
		StallCheckRate: 10 * time.Millisecond,
	}

	// stallTimeout=0 should disable stall detection
	stop := r.startHeartbeatWithConfig(stats, 0, 0, onStall, cfg, nil)
	time.Sleep(100 * time.Millisecond)
	stop()

	if stallFired {
		t.Fatal("Stall should not fire when stallTimeout is 0")
	}
}

func TestGetGitDiffStatSameCommit(t *testing.T) {
	// Diffing a commit against itself should produce empty output
	head, err := getGitHead()
	if err != nil {
		t.Skip("git not available")
	}

	stat, err := getGitDiffStat(head)
	if err != nil {
		t.Fatalf("getGitDiffStat() failed: %v", err)
	}

	// When comparing working tree to current HEAD, there may be uncommitted changes
	// but at minimum, the function should not error
	_ = stat
}

func TestGetGitDiff(t *testing.T) {
	// This test requires a git repo, so skip in CI if needed
	// Use the project's own repo for testing
	head, err := getGitHead()
	if err != nil {
		t.Skip("not in a git repo")
	}

	// Get diff from HEAD to HEAD (should be empty for committed state)
	diff, err := getGitDiff(head)
	if err != nil {
		t.Fatal(err)
	}
	// diff may or may not be empty depending on working tree state
	// just verify it doesn't error
	_ = diff
}

func TestRetryCounterBehavior(t *testing.T) {
	// This test documents the retry counter behavior:
	// - totalRetriesThisBead tracks the total number of retry attempts
	// - It is incremented on stall retries (line ~305)
	// - It is incremented on recoverable retries (line ~412)
	// - Escalation itself does NOT increment the counter (it's a model switch, not a retry)
	// - The counter is checked against MaxRetriesPerBead limit after each increment
	// - Before escalating, we check if the limit has been reached (line ~452)
	//
	// This ensures that:
	// 1. We don't retry the same operation infinitely
	// 2. We don't escalate if we've already exhausted our retry budget
	// 3. Escalation gives the new model a chance to try (with its own retry budget)
	//
	// Example flow with MaxRetriesPerBead=3:
	// - haiku fails, retry 1 (counter=1)
	// - haiku fails, retry 2 (counter=2)
	// - haiku exhausted, escalate to sonnet (counter=2, not incremented)
	// - sonnet fails, retry 1 (counter=3)
	// - counter >= 3, cannot escalate further
	//
	// This test is documentary - the actual logic is in processBead()
	t.Log("Retry counter behavior documented")
}

func TestParseDecomposeOutputValidJSON(t *testing.T) {
	output := `[
		{
			"title": "Set up database",
			"description": "Create schema",
			"depends_on": null,
			"acceptance_criteria": ["Schema created", "Migrations run"]
		},
		{
			"title": "Implement user model",
			"description": "Add User model",
			"depends_on": 0,
			"acceptance_criteria": ["Model created", "Tests pass"]
		}
	]`

	subTasks, err := parseDecomposeOutput(output)
	if err != nil {
		t.Fatalf("parseDecomposeOutput() failed: %v", err)
	}

	if len(subTasks) != 2 {
		t.Errorf("expected 2 sub-tasks, got %d", len(subTasks))
	}

	// Check first task
	if subTasks[0].Title != "Set up database" {
		t.Errorf("expected title 'Set up database', got %q", subTasks[0].Title)
	}
	if subTasks[0].DependsOn != nil {
		t.Errorf("expected DependsOn to be nil, got %v", subTasks[0].DependsOn)
	}
	if len(subTasks[0].AcceptanceCriteria) != 2 {
		t.Errorf("expected 2 acceptance criteria, got %d", len(subTasks[0].AcceptanceCriteria))
	}

	// Check second task
	if subTasks[1].Title != "Implement user model" {
		t.Errorf("expected title 'Implement user model', got %q", subTasks[1].Title)
	}
	if subTasks[1].DependsOn == nil || *subTasks[1].DependsOn != 0 {
		t.Errorf("expected DependsOn to be 0, got %v", subTasks[1].DependsOn)
	}
	if len(subTasks[1].AcceptanceCriteria) != 2 {
		t.Errorf("expected 2 acceptance criteria, got %d", len(subTasks[1].AcceptanceCriteria))
	}
}

func TestParseDecomposeOutputEmptyString(t *testing.T) {
	_, err := parseDecomposeOutput("")
	if err == nil {
		t.Error("expected error for empty string")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error message, got %q", err.Error())
	}
}

func TestParseDecomposeOutputInvalidJSON(t *testing.T) {
	_, err := parseDecomposeOutput("not valid json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("expected 'parsing' in error message, got %q", err.Error())
	}
}

func TestParseDecomposeOutputEmptyArray(t *testing.T) {
	_, err := parseDecomposeOutput("[]")
	if err == nil {
		t.Error("expected error for empty array")
	}
	if !strings.Contains(err.Error(), "no sub-tasks") {
		t.Errorf("expected 'no sub-tasks' in error message, got %q", err.Error())
	}
}

func TestParseDecomposeOutputSingleTask(t *testing.T) {
	output := `[
		{
			"title": "Single task",
			"description": "Do one thing",
			"depends_on": null,
			"acceptance_criteria": ["Done"]
		}
	]`

	subTasks, err := parseDecomposeOutput(output)
	if err != nil {
		t.Fatalf("parseDecomposeOutput() failed: %v", err)
	}

	if len(subTasks) != 1 {
		t.Errorf("expected 1 sub-task, got %d", len(subTasks))
	}

	if subTasks[0].Title != "Single task" {
		t.Errorf("expected title 'Single task', got %q", subTasks[0].Title)
	}
	if len(subTasks[0].AcceptanceCriteria) != 1 {
		t.Errorf("expected 1 acceptance criterion, got %d", len(subTasks[0].AcceptanceCriteria))
	}
}

func TestIsStuckBeadNilRunner(t *testing.T) {
	var r *Runner
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.isStuckBead(b)
	if result {
		t.Error("expected isStuckBead to return false for nil runner")
	}
}

func TestIsStuckBeadNilBead(t *testing.T) {
	tmpDir := t.TempDir()
	r := &Runner{
		cfg: &config.Config{
			Paths: config.PathsConfig{Logs: tmpDir},
			Loop:  config.LoopConfig{StuckBeadThreshold: 3},
		},
	}
	result := r.isStuckBead(nil)
	if result {
		t.Error("expected isStuckBead to return false for nil bead")
	}
}

func TestIsStuckBeadDisabledThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	r := &Runner{
		cfg: &config.Config{
			Paths: config.PathsConfig{Logs: tmpDir},
			Loop:  config.LoopConfig{StuckBeadThreshold: 0},
		},
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.isStuckBead(b)
	if result {
		t.Error("expected isStuckBead to return false when threshold is 0 (disabled)")
	}
}

func TestIsStuckBeadNegativeThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	r := &Runner{
		cfg: &config.Config{
			Paths: config.PathsConfig{Logs: tmpDir},
			Loop:  config.LoopConfig{StuckBeadThreshold: -1},
		},
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.isStuckBead(b)
	if result {
		t.Error("expected isStuckBead to return false when threshold is negative (disabled)")
	}
}

func TestIsStuckBeadNoHistory(t *testing.T) {
	tmpDir := t.TempDir()
	r := &Runner{
		cfg: &config.Config{
			Paths: config.PathsConfig{Logs: tmpDir},
			Loop:  config.LoopConfig{StuckBeadThreshold: 3},
		},
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.isStuckBead(b)
	if result {
		t.Error("expected isStuckBead to return false for bead with no history")
	}
}

func TestIsStuckBeadBelowThreshold(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a log entry with 2 failures (below threshold of 3)
	logFile := filepath.Join(tmpDir, "run-20240101-120000.jsonl")
	entries := []logger.IterationLog{
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: false},
	}

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("creating log file: %v", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			t.Fatalf("encoding entry: %v", err)
		}
	}

	r := &Runner{
		cfg: &config.Config{
			Paths: config.PathsConfig{Logs: tmpDir},
			Loop:  config.LoopConfig{StuckBeadThreshold: 3},
		},
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.isStuckBead(b)
	if result {
		t.Error("expected isStuckBead to return false when failures < threshold")
	}
}

func TestIsStuckBeadAtThreshold(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a log entry with exactly 3 failures (at threshold)
	logFile := filepath.Join(tmpDir, "run-20240101-120000.jsonl")
	entries := []logger.IterationLog{
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: false},
	}

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("creating log file: %v", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			t.Fatalf("encoding entry: %v", err)
		}
	}

	r := &Runner{
		cfg: &config.Config{
			Paths: config.PathsConfig{Logs: tmpDir},
			Loop:  config.LoopConfig{StuckBeadThreshold: 3},
		},
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.isStuckBead(b)
	if !result {
		t.Error("expected isStuckBead to return true when failures >= threshold")
	}
}

func TestIsStuckBeadAboveThreshold(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a log entry with 5 failures (above threshold of 3)
	logFile := filepath.Join(tmpDir, "run-20240101-120000.jsonl")
	entries := []logger.IterationLog{
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: false},
	}

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("creating log file: %v", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			t.Fatalf("encoding entry: %v", err)
		}
	}

	r := &Runner{
		cfg: &config.Config{
			Paths: config.PathsConfig{Logs: tmpDir},
			Loop:  config.LoopConfig{StuckBeadThreshold: 3},
		},
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.isStuckBead(b)
	if !result {
		t.Error("expected isStuckBead to return true when failures > threshold")
	}
}

func TestIsStuckBeadWithSuccesses(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a log with 3 failures and 1 success
	logFile := filepath.Join(tmpDir, "run-20240101-120000.jsonl")
	entries := []logger.IterationLog{
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: true},
	}

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("creating log file: %v", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			t.Fatalf("encoding entry: %v", err)
		}
	}

	r := &Runner{
		cfg: &config.Config{
			Paths: config.PathsConfig{Logs: tmpDir},
			Loop:  config.LoopConfig{StuckBeadThreshold: 3},
		},
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.isStuckBead(b)
	if !result {
		t.Error("expected isStuckBead to return true when failures >= threshold (regardless of successes)")
	}
}

func TestIsStuckBeadMultipleBeads(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a log with multiple beads, only one stuck
	logFile := filepath.Join(tmpDir, "run-20240101-120000.jsonl")
	entries := []logger.IterationLog{
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: false},
		{BeadID: "test-1", Success: false},
		{BeadID: "test-2", Success: false},
		{BeadID: "test-3", Success: true},
	}

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("creating log file: %v", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			t.Fatalf("encoding entry: %v", err)
		}
	}

	r := &Runner{
		cfg: &config.Config{
			Paths: config.PathsConfig{Logs: tmpDir},
			Loop:  config.LoopConfig{StuckBeadThreshold: 3},
		},
	}

	// test-1 is stuck (3 failures)
	result := r.isStuckBead(&bead.Bead{ID: "test-1"})
	if !result {
		t.Error("expected test-1 to be stuck")
	}

	// test-2 is not stuck (1 failure)
	result = r.isStuckBead(&bead.Bead{ID: "test-2"})
	if result {
		t.Error("expected test-2 to not be stuck")
	}

	// test-3 is not stuck (0 failures)
	result = r.isStuckBead(&bead.Bead{ID: "test-3"})
	if result {
		t.Error("expected test-3 to not be stuck")
	}
}

func TestCreateSubBeads_VerifyLogging(t *testing.T) {
	// Test that CreateSubBeads logs appropriately during processing
	// This test verifies the method's logging behavior by checking that
	// it attempts to create beads from sub-tasks

	buf := &strings.Builder{}
	r := &Runner{
		beads:  &mockBeadClient{},
		output: buf, // Capture logging output
	}

	b := &bead.Bead{
		ID:              "parent-123",
		Title:           "Parent Task",
		Priority:        1,
		Labels:          []string{"complexity:high"},
		ExpectedOutputs: []string{},
	}

	subTasks := []SubTask{
		{
			Title:       "Sub-task 1",
			Description: "First sub-task",
		},
	}

	err := r.CreateSubBeads(nil, b, subTasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that appropriate log messages were generated
	logOutput := buf.String()
	if !strings.Contains(logOutput, "Creating sub-bead") {
		t.Errorf("expected log message about creating sub-bead, got: %s", logOutput)
	}
}

func TestCreateSubBeads_NilRunner(t *testing.T) {
	var r *Runner
	b := &bead.Bead{ID: "test-1"}
	subTasks := []SubTask{{Title: "Task 1"}}

	err := r.CreateSubBeads(nil, b, subTasks)
	if err == nil || !strings.Contains(err.Error(), "runner is nil") {
		t.Errorf("expected error for nil runner, got: %v", err)
	}
}

func TestCreateSubBeads_NilBead(t *testing.T) {
	r := &Runner{beads: &mockBeadClient{}, output: os.Stderr}
	subTasks := []SubTask{{Title: "Task 1"}}

	err := r.CreateSubBeads(nil, nil, subTasks)
	if err == nil || !strings.Contains(err.Error(), "bead is nil") {
		t.Errorf("expected error for nil bead, got: %v", err)
	}
}

func TestCreateSubBeads_NoSubTasks(t *testing.T) {
	r := &Runner{beads: &mockBeadClient{}, output: os.Stderr}
	b := &bead.Bead{ID: "test-1"}
	var subTasks []SubTask

	err := r.CreateSubBeads(nil, b, subTasks)
	if err == nil || !strings.Contains(err.Error(), "no sub-tasks") {
		t.Errorf("expected error for no sub-tasks, got: %v", err)
	}
}

func TestProcessBeadNilBeads(t *testing.T) {
	r := &Runner{
		cfg:    &config.Config{},
		output: os.Stdout,
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.processBead(nil, b, 1)
	if result.Error == nil {
		t.Error("expected error for nil beads client")
	}
	if !strings.Contains(result.Error.Error(), "beads client is nil") {
		t.Errorf("expected 'beads client is nil' in error, got %q", result.Error.Error())
	}
}

func TestProcessBeadNilRenderer(t *testing.T) {
	r := &Runner{
		cfg:    &config.Config{},
		beads:  &mockBeadClient{},
		output: os.Stdout,
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.processBead(nil, b, 1)
	if result.Error == nil {
		t.Error("expected error for nil renderer")
	}
	if !strings.Contains(result.Error.Error(), "renderer is nil") {
		t.Errorf("expected 'renderer is nil' in error, got %q", result.Error.Error())
	}
}

func TestProcessBeadNilClaude(t *testing.T) {
	r := &Runner{
		cfg:      &config.Config{},
		beads:    &mockBeadClient{},
		renderer: &mockRenderer{},
		output:   os.Stdout,
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	result := r.processBead(nil, b, 1)
	if result.Error == nil {
		t.Error("expected error for nil claude client")
	}
	if !strings.Contains(result.Error.Error(), "claude client is nil") {
		t.Errorf("expected 'claude client is nil' in error, got %q", result.Error.Error())
	}
}

func TestRunNilBeads(t *testing.T) {
	r := &Runner{
		cfg:    &config.Config{},
		output: os.Stdout,
	}
	err := r.Run(nil, 0, false)
	if err == nil {
		t.Error("expected error for nil beads client")
	}
	if !strings.Contains(err.Error(), "beads client is nil") {
		t.Errorf("expected 'beads client is nil' in error, got %q", err.Error())
	}
}

func TestRunNilRenderer(t *testing.T) {
	r := &Runner{
		cfg:    &config.Config{},
		beads:  &mockBeadClient{},
		output: os.Stdout,
	}
	err := r.Run(nil, 0, false)
	if err == nil {
		t.Error("expected error for nil renderer")
	}
	if !strings.Contains(err.Error(), "renderer is nil") {
		t.Errorf("expected 'renderer is nil' in error, got %q", err.Error())
	}
}

func TestRunNilClaude(t *testing.T) {
	r := &Runner{
		cfg:      &config.Config{},
		beads:    &mockBeadClient{},
		renderer: &mockRenderer{},
		output:   os.Stdout,
	}
	err := r.Run(nil, 0, false)
	if err == nil {
		t.Error("expected error for nil claude client")
	}
	if !strings.Contains(err.Error(), "claude client is nil") {
		t.Errorf("expected 'claude client is nil' in error, got %q", err.Error())
	}
}

func TestStatusNilBeads(t *testing.T) {
	r := &Runner{
		cfg:    &config.Config{},
		output: os.Stdout,
	}
	err := r.Status()
	if err == nil {
		t.Error("expected error for nil beads client")
	}
	if !strings.Contains(err.Error(), "beads client is nil") {
		t.Errorf("expected 'beads client is nil' in error, got %q", err.Error())
	}
}

func TestSelectModelNilConfig(t *testing.T) {
	r := &Runner{}
	b := &bead.Bead{ID: "test-1", Title: "Test", Priority: 1}
	result := r.selectModel(b)
	if result != "sonnet" {
		t.Errorf("expected 'sonnet' for nil config, got %q", result)
	}
}

func TestDecomposeTaskNilBeads(t *testing.T) {
	r := &Runner{output: os.Stdout}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	_, err := r.DecomposeTask(nil, b)
	if err == nil {
		t.Error("expected error for nil beads client")
	}
	if !strings.Contains(err.Error(), "beads client is nil") {
		t.Errorf("expected 'beads client is nil' in error, got %q", err.Error())
	}
}

func TestDecomposeTaskNilRenderer(t *testing.T) {
	r := &Runner{
		beads:  &mockBeadClient{},
		output: os.Stdout,
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	_, err := r.DecomposeTask(nil, b)
	if err == nil {
		t.Error("expected error for nil renderer")
	}
	if !strings.Contains(err.Error(), "renderer is nil") {
		t.Errorf("expected 'renderer is nil' in error, got %q", err.Error())
	}
}

func TestDecomposeTaskNilClaude(t *testing.T) {
	r := &Runner{
		beads:    &mockBeadClient{},
		renderer: &mockRenderer{},
		output:   os.Stdout,
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	_, err := r.DecomposeTask(nil, b)
	if err == nil {
		t.Error("expected error for nil claude client")
	}
	if !strings.Contains(err.Error(), "claude client is nil") {
		t.Errorf("expected 'claude client is nil' in error, got %q", err.Error())
	}
}

func TestCreateSubBeads_NilBeadsClient(t *testing.T) {
	r := &Runner{output: os.Stdout}
	b := &bead.Bead{ID: "test-1"}
	subTasks := []SubTask{{Title: "Task 1"}}

	err := r.CreateSubBeads(nil, b, subTasks)
	if err == nil || !strings.Contains(err.Error(), "beads client is nil") {
		t.Errorf("expected error for nil beads client, got: %v", err)
	}
}

func TestWriteIterationLogNilRunner(t *testing.T) {
	var r *Runner
	result := &IterationResult{BeadID: "test-1"}
	// Should not panic
	r.writeIterationLog(1, result)
}

func TestWriteIterationLogNilResult(t *testing.T) {
	r := &Runner{output: os.Stdout}
	// Should not panic
	r.writeIterationLog(1, nil)
}

func TestLogResultNilRunner(t *testing.T) {
	var r *Runner
	result := &IterationResult{BeadID: "test-1", Success: true}
	// Should not panic
	r.logResult(result)
}

func TestLogResultNilResult(t *testing.T) {
	r := &Runner{output: os.Stdout}
	// Should not panic
	r.logResult(nil)
}

func TestPrintHeartbeatNilRunner(t *testing.T) {
	var r *Runner
	stats, _ := logger.NewStreamStats()
	// Should not panic
	r.printHeartbeat(stats)
}

func TestPrintHeartbeatNilStats(t *testing.T) {
	r := &Runner{output: os.Stdout}
	// Should not panic
	r.printHeartbeat(nil)
}

func TestStartHeartbeatNilRunner(t *testing.T) {
	var r *Runner
	stats, _ := logger.NewStreamStats()
	stop := r.startHeartbeatWithConfig(stats, 0, 0, nil, defaultHeartbeatConfig, nil)
	// Should return a no-op function
	stop()
}

func TestStartHeartbeatNilStats(t *testing.T) {
	r := &Runner{output: os.Stdout}
	stop := r.startHeartbeatWithConfig(nil, 0, 0, nil, defaultHeartbeatConfig, nil)
	// Should return a no-op function
	stop()
}

func TestShowPartialProgressNilRunner(t *testing.T) {
	var r *Runner
	b := &bead.Bead{ID: "test-1"}
	// Should not panic
	r.showPartialProgress(b, "abc123")
}

func TestCheckScopeNilRunner(t *testing.T) {
	var r *Runner
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	// Should not panic, should return nil
	result := r.checkScope(nil, b)
	if result != nil {
		t.Errorf("expected nil for nil runner, got %v", result)
	}
}

func TestCheckScopeNilConfig(t *testing.T) {
	r := &Runner{output: os.Stdout}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	// Should not panic, should return nil
	result := r.checkScope(nil, b)
	if result != nil {
		t.Errorf("expected nil for nil config, got %v", result)
	}
}

func TestCheckScopeNilBead(t *testing.T) {
	r := &Runner{
		cfg:    &config.Config{},
		output: os.Stdout,
	}
	// Should not panic, should return nil
	result := r.checkScope(nil, nil)
	if result != nil {
		t.Errorf("expected nil for nil bead, got %v", result)
	}
}

func TestCheckScopeNilRenderer(t *testing.T) {
	r := &Runner{
		cfg:    &config.Config{},
		output: os.Stdout,
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	// Should not panic, should return nil
	result := r.checkScope(nil, b)
	if result != nil {
		t.Errorf("expected nil for nil renderer, got %v", result)
	}
}

func TestCheckScopeNilClaude(t *testing.T) {
	r := &Runner{
		cfg:      &config.Config{},
		renderer: &mockRenderer{},
		output:   os.Stdout,
	}
	b := &bead.Bead{ID: "test-1", Title: "Test"}
	// Should not panic, should return nil
	result := r.checkScope(nil, b)
	if result != nil {
		t.Errorf("expected nil for nil claude client, got %v", result)
	}
}

func TestParseDecomposeOutputWithLeadingText(t *testing.T) {
	output := `Here's the decomposed task:

[
	{
		"title": "Set up database",
		"description": "Create schema",
		"depends_on": null,
		"acceptance_criteria": ["Schema created"]
	},
	{
		"title": "Implement user model",
		"description": "Add User model",
		"depends_on": 0,
		"acceptance_criteria": ["Model created"]
	}
]`

	subTasks, err := parseDecomposeOutput(output)
	if err != nil {
		t.Fatalf("parseDecomposeOutput() failed: %v", err)
	}

	if len(subTasks) != 2 {
		t.Errorf("expected 2 sub-tasks, got %d", len(subTasks))
	}

	if subTasks[0].Title != "Set up database" {
		t.Errorf("expected title 'Set up database', got %q", subTasks[0].Title)
	}
}

func TestParseDecomposeOutputWithTrailingText(t *testing.T) {
	output := `[
	{
		"title": "Task 1",
		"description": "Do something",
		"depends_on": null,
		"acceptance_criteria": ["Done"]
	}
]

This is the decomposition. Let me know if you'd like me to adjust it.`

	subTasks, err := parseDecomposeOutput(output)
	if err != nil {
		t.Fatalf("parseDecomposeOutput() failed: %v", err)
	}

	if len(subTasks) != 1 {
		t.Errorf("expected 1 sub-task, got %d", len(subTasks))
	}

	if subTasks[0].Title != "Task 1" {
		t.Errorf("expected title 'Task 1', got %q", subTasks[0].Title)
	}
}

func TestParseDecomposeOutputWithSurroundingText(t *testing.T) {
	output := `Here are the sub-tasks:

[
	{
		"title": "First",
		"description": "First task",
		"depends_on": null,
		"acceptance_criteria": ["A"]
	}
]

All done!`

	subTasks, err := parseDecomposeOutput(output)
	if err != nil {
		t.Fatalf("parseDecomposeOutput() failed: %v", err)
	}

	if len(subTasks) != 1 {
		t.Errorf("expected 1 sub-task, got %d", len(subTasks))
	}

	if subTasks[0].Title != "First" {
		t.Errorf("expected title 'First', got %q", subTasks[0].Title)
	}
}

func TestParseDecomposeOutputWithNestedQuotes(t *testing.T) {
	output := `The tasks are:
[
	{
		"title": "Fix bug in parser",
		"description": "The parser has a known issue",
		"depends_on": null,
		"acceptance_criteria": ["Tests pass"]
	}
]`

	subTasks, err := parseDecomposeOutput(output)
	if err != nil {
		t.Fatalf("parseDecomposeOutput() failed: %v", err)
	}

	if len(subTasks) != 1 {
		t.Errorf("expected 1 sub-task, got %d", len(subTasks))
	}

	if subTasks[0].Title != "Fix bug in parser" {
		t.Errorf("expected title 'Fix bug in parser', got %q", subTasks[0].Title)
	}
}

func TestParseDecomposeOutputNoJSONArray(t *testing.T) {
	output := `This is just plain text with no JSON array.
It explains the tasks but doesn't provide JSON.`

	_, err := parseDecomposeOutput(output)
	if err == nil {
		t.Error("expected error when no JSON array is found")
	}
	if !strings.Contains(err.Error(), "no JSON array found") {
		t.Errorf("expected 'no JSON array found' in error, got %q", err.Error())
	}
}

func TestParseDecomposeOutputMissingClosingBracket(t *testing.T) {
	output := `Here's the data:
[
	{
		"title": "Task 1",
		"description": "Incomplete array`

	_, err := parseDecomposeOutput(output)
	if err == nil {
		t.Error("expected error when closing bracket is missing")
	}
	if !strings.Contains(err.Error(), "malformed JSON array") {
		t.Errorf("expected 'malformed JSON array' in error, got %q", err.Error())
	}
}

func TestParseDecomposeOutputComplexNestedStructure(t *testing.T) {
	output := `Here's the breakdown:

[
	{
		"title": "Parse JSON arrays",
		"description": "Handle nested braces and arrays properly",
		"depends_on": null,
		"acceptance_criteria": ["Handles braces", "Handles arrays", "Handles special characters"]
	},
	{
		"title": "Test edge cases",
		"description": "Test with various inputs",
		"depends_on": 0,
		"acceptance_criteria": ["Edge case 1", "Edge case 2"]
	}
]

These tasks should handle complex nesting scenarios.`

	subTasks, err := parseDecomposeOutput(output)
	if err != nil {
		t.Fatalf("parseDecomposeOutput() failed: %v", err)
	}

	if len(subTasks) != 2 {
		t.Errorf("expected 2 sub-tasks, got %d", len(subTasks))
	}

	if subTasks[0].Title != "Parse JSON arrays" {
		t.Errorf("expected title 'Parse JSON arrays', got %q", subTasks[0].Title)
	}

	if len(subTasks[0].AcceptanceCriteria) != 3 {
		t.Errorf("expected 3 acceptance criteria, got %d", len(subTasks[0].AcceptanceCriteria))
	}

	if subTasks[1].DependsOn == nil || *subTasks[1].DependsOn != 0 {
		t.Errorf("expected DependsOn to be 0, got %v", subTasks[1].DependsOn)
	}
}

func TestSubTaskNormalizeNilFields(t *testing.T) {
	tests := []struct {
		name    string
		subTask *SubTask
	}{
		{
			name:    "nil AcceptanceCriteria",
			subTask: &SubTask{Title: "Test"},
		},
		{
			name:    "already non-nil",
			subTask: &SubTask{Title: "Test", AcceptanceCriteria: []string{"criteria1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.subTask.normalizeNilFields()
			if tt.subTask.AcceptanceCriteria == nil {
				t.Error("AcceptanceCriteria should not be nil after normalization")
			}
		})
	}
}

func TestSubTaskNormalizeNilFieldsOnNilSubTask(t *testing.T) {
	var s *SubTask
	s.normalizeNilFields() // Should not panic
}

func TestParseDecomposeOutputNormalizesNilFields(t *testing.T) {
	// JSON where acceptance_criteria is missing
	output := `[
		{
			"title": "Task without criteria",
			"description": "No acceptance_criteria field",
			"depends_on": null
		}
	]`

	subTasks, err := parseDecomposeOutput(output)
	if err != nil {
		t.Fatalf("parseDecomposeOutput() failed: %v", err)
	}
	if subTasks[0].AcceptanceCriteria == nil {
		t.Error("AcceptanceCriteria should not be nil after parseDecomposeOutput")
	}
}

func TestParseDecomposeOutputNormalizesExplicitNull(t *testing.T) {
	// JSON where acceptance_criteria is explicitly null
	output := `[
		{
			"title": "Task with null criteria",
			"description": "Explicit null",
			"depends_on": null,
			"acceptance_criteria": null
		}
	]`

	subTasks, err := parseDecomposeOutput(output)
	if err != nil {
		t.Fatalf("parseDecomposeOutput() failed: %v", err)
	}
	if subTasks[0].AcceptanceCriteria == nil {
		t.Error("AcceptanceCriteria should not be nil after parseDecomposeOutput (was JSON null)")
	}
}

func TestParseDecomposeOutputWithSurroundingTextNormalizesNilFields(t *testing.T) {
	output := `Here are the tasks:
[
	{
		"title": "Extracted task",
		"description": "From surrounding text",
		"depends_on": null
	}
]
Done.`

	subTasks, err := parseDecomposeOutput(output)
	if err != nil {
		t.Fatalf("parseDecomposeOutput() failed: %v", err)
	}
	if subTasks[0].AcceptanceCriteria == nil {
		t.Error("AcceptanceCriteria should not be nil after parseDecomposeOutput with surrounding text")
	}
}
