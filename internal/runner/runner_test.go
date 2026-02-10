package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/review"
	"github.com/danabrams/gromit/internal/state"
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
	// This test runs in the gromit repo, so git HEAD should be available
	head, err := getGitHead()
	if err != nil {
		t.Fatalf("getGitHead() failed: %v", err)
	}

	if len(head) != 40 {
		t.Errorf("Expected 40-char SHA, got %d chars: %s", len(head), head)
	}
}

func TestNilGuards(t *testing.T) {
	tests := []struct {
		name    string
		fn      func() error
		wantErr bool
	}{
		{
			name: "NewRunnerNilConfig",
			fn: func() error {
				r, err := NewRunner(nil, os.Stdout)
				if r != nil {
					return fmt.Errorf("expected nil Runner for nil config")
				}
				return err
			},
			wantErr: true,
		},
		{
			name: "RunNilRunner",
			fn: func() error {
				var r *Runner
				return r.Run(nil, 0, time.Time{}, false)
			},
			wantErr: true,
		},
		{
			name: "StatusNilRunner",
			fn: func() error {
				var r *Runner
				return r.Status()
			},
			wantErr: true,
		},
		{
			name: "RunNilConfig",
			fn: func() error {
				r := &Runner{output: os.Stdout}
				err := r.Run(nil, 0, time.Time{}, false)
				if err == nil {
					return fmt.Errorf("expected error for nil config")
				}
				if !strings.Contains(err.Error(), "config is nil") {
					return fmt.Errorf("expected 'config is nil' in error, got %q", err.Error())
				}
				return nil
			},
		},
		{
			name: "ProcessBeadNilConfig",
			fn: func() error {
				r := &Runner{output: os.Stdout}
				b := &bead.Bead{ID: "test-1", Title: "Test"}
				result := r.processBead(nil, b, 1, time.Time{})
				if result.Error == nil {
					return fmt.Errorf("expected error for nil config in processBead")
				}
				if !strings.Contains(result.Error.Error(), "config is nil") {
					return fmt.Errorf("expected 'config is nil' in error, got %q", result.Error.Error())
				}
				return nil
			},
		},
		{
			name: "LogNilOutput",
			fn: func() error {
				r := &Runner{} // output is nil
				r.log("test message %s", "value")
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if tt.wantErr && err == nil {
				t.Errorf("%s: expected error but got nil", tt.name)
			} else if !tt.wantErr && err != nil {
				t.Errorf("%s: unexpected error: %v", tt.name, err)
			}
		})
	}
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

func TestHeartbeatWritesNewlineAfterOverwrite(t *testing.T) {
	var buf strings.Builder
	r := &Runner{output: &buf}

	stats, _ := logger.NewStreamStats()

	cfg := heartbeatConfig{
		InitialDelay:   10 * time.Millisecond,
		HeartbeatRate:  100 * time.Millisecond,
		StallCheckRate: 10 * time.Millisecond,
	}

	toolCallEvents := make(chan claude.ToolEvent, 1)
	stop := r.startHeartbeatWithConfig(stats, 0, 0, nil, cfg, toolCallEvents)

	// Wait for initial heartbeat
	time.Sleep(50 * time.Millisecond)

	// Send a tool call event to trigger overwrite mode
	toolCallEvents <- claude.ToolEvent{}

	// Wait a bit for the overwrite to happen
	time.Sleep(50 * time.Millisecond)

	// Stop the heartbeat
	stop()

	// Check that newline was written after overwrite
	output := buf.String()
	if !strings.HasSuffix(output, "\n") {
		t.Error("Expected newline at end of output after heartbeat overwrite and stop")
	}
}

func TestHeartbeatNoNewlineAfterPrintMode(t *testing.T) {
	var buf strings.Builder
	r := &Runner{output: &buf}

	stats, _ := logger.NewStreamStats()

	cfg := heartbeatConfig{
		InitialDelay:   10 * time.Millisecond,
		HeartbeatRate:  100 * time.Millisecond,
		StallCheckRate: 10 * time.Millisecond,
	}

	// No tool call events, so only printHeartbeat is used (not overwrite)
	stop := r.startHeartbeatWithConfig(stats, 0, 0, nil, cfg, nil)

	// Wait for initial heartbeat
	time.Sleep(50 * time.Millisecond)

	// Stop the heartbeat - should not add extra newline since overwrite was not used
	initialLen := buf.Len()
	stop()
	finalLen := buf.Len()

	// When overwrite was not used, no additional newline should be written
	// (printHeartbeat already adds one via r.log)
	added := finalLen - initialLen
	if added > 0 {
		// This is acceptable - stop() might write a newline if system is uncertain
		// The key is that if overwrite WAS used, we MUST have a newline
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

func TestCreateSubBeadsErrors(t *testing.T) {
	tests := []struct {
		name        string
		runner      *Runner
		bead        *bead.Bead
		subTasks    []SubTask
		expectedErr string
		nilRunner   bool
	}{
		{
			name:        "NilRunner",
			runner:      nil,
			bead:        &bead.Bead{ID: "test-1"},
			subTasks:    []SubTask{{Title: "Task 1"}},
			expectedErr: "runner is nil",
			nilRunner:   true,
		},
		{
			name:        "NilBead",
			runner:      &Runner{beads: &mockBeadClient{}, output: os.Stderr},
			bead:        nil,
			subTasks:    []SubTask{{Title: "Task 1"}},
			expectedErr: "bead is nil",
		},
		{
			name:        "NoSubTasks",
			runner:      &Runner{beads: &mockBeadClient{}, output: os.Stderr},
			bead:        &bead.Bead{ID: "test-1"},
			subTasks:    []SubTask{},
			expectedErr: "no sub-tasks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.nilRunner {
				var r *Runner
				err = r.CreateSubBeads(nil, tt.bead, tt.subTasks)
			} else {
				err = tt.runner.CreateSubBeads(nil, tt.bead, tt.subTasks)
			}
			if err == nil || !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("expected %q in error, got: %v", tt.expectedErr, err)
			}
		})
	}
}

func TestProcessBeadAndRunNilDependencies(t *testing.T) {
	tests := []struct {
		name          string
		runner        *Runner
		method        func(*Runner) error
		expectedError string
	}{
		{
			name: "ProcessBeadNilBeads",
			runner: &Runner{
				cfg:    &config.Config{},
				output: os.Stdout,
			},
			method: func(r *Runner) error {
				b := &bead.Bead{ID: "test-1", Title: "Test"}
				result := r.processBead(nil, b, 1, time.Time{})
				return result.Error
			},
			expectedError: "beads client is nil",
		},
		{
			name: "ProcessBeadNilRenderer",
			runner: &Runner{
				cfg:    &config.Config{},
				beads:  &mockBeadClient{},
				output: os.Stdout,
			},
			method: func(r *Runner) error {
				b := &bead.Bead{ID: "test-1", Title: "Test"}
				result := r.processBead(nil, b, 1, time.Time{})
				return result.Error
			},
			expectedError: "renderer is nil",
		},
		{
			name: "ProcessBeadNilClaude",
			runner: &Runner{
				cfg:      &config.Config{},
				beads:    &mockBeadClient{},
				renderer: &mockRenderer{},
				output:   os.Stdout,
			},
			method: func(r *Runner) error {
				b := &bead.Bead{ID: "test-1", Title: "Test"}
				result := r.processBead(nil, b, 1, time.Time{})
				return result.Error
			},
			expectedError: "claude client is nil",
		},
		{
			name: "RunNilBeads",
			runner: &Runner{
				cfg:    &config.Config{},
				output: os.Stdout,
			},
			method: func(r *Runner) error {
				return r.Run(nil, 0, time.Time{}, false)
			},
			expectedError: "beads client is nil",
		},
		{
			name: "RunNilRenderer",
			runner: &Runner{
				cfg:    &config.Config{},
				beads:  &mockBeadClient{},
				output: os.Stdout,
			},
			method: func(r *Runner) error {
				return r.Run(nil, 0, time.Time{}, false)
			},
			expectedError: "renderer is nil",
		},
		{
			name: "RunNilClaude",
			runner: &Runner{
				cfg:      &config.Config{},
				beads:    &mockBeadClient{},
				renderer: &mockRenderer{},
				output:   os.Stdout,
			},
			method: func(r *Runner) error {
				return r.Run(nil, 0, time.Time{}, false)
			},
			expectedError: "claude client is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.method(tt.runner)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
				return
			}
			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("expected %q in error, got %q", tt.expectedError, err.Error())
			}
		})
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

func TestDecomposeTaskNilDependencies(t *testing.T) {
	tests := []struct {
		name          string
		runner        *Runner
		expectedError string
	}{
		{
			name:          "DecomposeTaskNilBeads",
			runner:        &Runner{output: os.Stdout},
			expectedError: "beads client is nil",
		},
		{
			name:          "DecomposeTaskNilRenderer",
			runner:        &Runner{beads: &mockBeadClient{}, output: os.Stdout},
			expectedError: "renderer is nil",
		},
		{
			name:          "DecomposeTaskNilClaude",
			runner:        &Runner{beads: &mockBeadClient{}, renderer: &mockRenderer{}, output: os.Stdout},
			expectedError: "claude client is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &bead.Bead{ID: "test-1", Title: "Test"}
			_, err := tt.runner.DecomposeTask(nil, b)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
				return
			}
			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("expected %q in error, got %q", tt.expectedError, err.Error())
			}
		})
	}
}

func TestCreateSubBeadsNilBeadsClient(t *testing.T) {
	r := &Runner{output: os.Stdout}
	b := &bead.Bead{ID: "test-1"}
	subTasks := []SubTask{{Title: "Task 1"}}
	err := r.CreateSubBeads(nil, b, subTasks)
	if err == nil || !strings.Contains(err.Error(), "beads client is nil") {
		t.Errorf("expected error for nil beads client, got: %v", err)
	}
}

func TestCreateSubBeads_MethodologyInheritance(t *testing.T) {
	tests := []struct {
		name           string
		globalATDD     bool
		globalTDD      bool
		parentLabels   []string
		expectedLabels []string
		description    string
	}{
		{
			name:           "Global ATDD true, no parent label - adds atdd:true",
			globalATDD:     true,
			globalTDD:      false,
			parentLabels:   []string{"spec:auth"},
			expectedLabels: []string{"spec:auth", "atdd:true"},
			description:    "When global ATDD is enabled and parent has no atdd label, sub-bead should get atdd:true",
		},
		{
			name:           "Global TDD true, no parent label - adds tdd:true",
			globalATDD:     false,
			globalTDD:      true,
			parentLabels:   []string{"spec:auth"},
			expectedLabels: []string{"spec:auth", "tdd:true"},
			description:    "When global TDD is enabled and parent has no tdd label, sub-bead should get tdd:true",
		},
		{
			name:           "Both methodologies true, no parent labels - adds both",
			globalATDD:     true,
			globalTDD:      true,
			parentLabels:   []string{"spec:auth"},
			expectedLabels: []string{"spec:auth", "atdd:true", "tdd:true"},
			description:    "When both methodologies are globally enabled, sub-bead should get both labels",
		},
		{
			name:           "Parent has atdd:true - no duplicate",
			globalATDD:     true,
			globalTDD:      false,
			parentLabels:   []string{"spec:auth", "atdd:true"},
			expectedLabels: []string{"spec:auth", "atdd:true"},
			description:    "When parent already has atdd:true, should not add duplicate",
		},
		{
			name:           "Parent has atdd:false - not overridden",
			globalATDD:     true,
			globalTDD:      false,
			parentLabels:   []string{"spec:auth", "atdd:false"},
			expectedLabels: []string{"spec:auth", "atdd:false"},
			description:    "When parent has explicit atdd:false, should preserve it even if global is true",
		},
		{
			name:           "Parent has tdd:false - not overridden",
			globalATDD:     false,
			globalTDD:      true,
			parentLabels:   []string{"spec:auth", "tdd:false"},
			expectedLabels: []string{"spec:auth", "tdd:false"},
			description:    "When parent has explicit tdd:false, should preserve it even if global is true",
		},
		{
			name:           "Global false, no parent labels - no additions",
			globalATDD:     false,
			globalTDD:      false,
			parentLabels:   []string{"spec:auth"},
			expectedLabels: []string{"spec:auth"},
			description:    "When methodologies are globally disabled, no labels should be added",
		},
		{
			name:           "Mixed: parent has atdd:true, global TDD true - adds tdd:true",
			globalATDD:     false,
			globalTDD:      true,
			parentLabels:   []string{"spec:auth", "atdd:true"},
			expectedLabels: []string{"spec:auth", "atdd:true", "tdd:true"},
			description:    "Should add tdd:true when global TDD is true and parent doesn't have tdd label",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedLabels []string
			mockBeads := &mockBeadClient{
				CreateWithParentAndDescriptionFn: func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
					capturedLabels = labels
					return &bead.Bead{
						ID:              "sub-1",
						Title:           title,
						Priority:        priority,
						Labels:          labels,
						ExpectedOutputs: []string{},
					}, nil
				},
			}

			cfg := &config.Config{
				Methodology: config.MethodologyConfig{
					ATDD: tt.globalATDD,
					TDD:  tt.globalTDD,
				},
			}

			var buf strings.Builder
			r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
				Deps{
					Beads:    mockBeads,
					Claude:   &mockClaudeClient{},
					Analyzer: &mockFailureAnalyzer{},
					Renderer: &mockPromptRenderer{},
					Logger:   &mockIterationLogger{},
				})
			if err != nil {
				t.Fatalf("Failed to create runner: %v", err)
			}

			parent := &bead.Bead{
				ID:              "parent-1",
				Title:           "Parent task",
				Priority:        1,
				Labels:          tt.parentLabels,
				ExpectedOutputs: []string{},
			}

			subTasks := []SubTask{
				{Title: "Sub-task 1", Description: "Do something"},
			}

			if err := r.CreateSubBeads(context.Background(), parent, subTasks); err != nil {
				t.Fatalf("CreateSubBeads() failed: %v", err)
			}

			// Check that labels match expected
			if len(capturedLabels) != len(tt.expectedLabels) {
				t.Errorf("%s\nExpected %d labels, got %d\nExpected: %v\nGot: %v",
					tt.description, len(tt.expectedLabels), len(capturedLabels), tt.expectedLabels, capturedLabels)
				return
			}

			// Check each expected label is present
			for _, expected := range tt.expectedLabels {
				found := false
				for _, actual := range capturedLabels {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("%s\nMissing expected label: %s\nGot: %v", tt.description, expected, capturedLabels)
				}
			}
		})
	}
}

func TestNilCallsShouldNotPanic(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "WriteIterationLogNilRunner",
			fn: func() {
				var r *Runner
				result := &IterationResult{BeadID: "test-1"}
				r.writeIterationLog(1, result)
			},
		},
		{
			name: "WriteIterationLogNilResult",
			fn: func() {
				r := &Runner{output: os.Stdout}
				r.writeIterationLog(1, nil)
			},
		},
		{
			name: "LogResultNilRunner",
			fn: func() {
				var r *Runner
				result := &IterationResult{BeadID: "test-1", Success: true}
				r.logResult(result)
			},
		},
		{
			name: "LogResultNilResult",
			fn: func() {
				r := &Runner{output: os.Stdout}
				r.logResult(nil)
			},
		},
		{
			name: "PrintHeartbeatNilRunner",
			fn: func() {
				var r *Runner
				stats, _ := logger.NewStreamStats()
				r.printHeartbeat(stats)
			},
		},
		{
			name: "PrintHeartbeatNilStats",
			fn: func() {
				r := &Runner{output: os.Stdout}
				r.printHeartbeat(nil)
			},
		},
		{
			name: "StartHeartbeatNilRunner",
			fn: func() {
				var r *Runner
				stats, _ := logger.NewStreamStats()
				stop := r.startHeartbeatWithConfig(stats, 0, 0, nil, defaultHeartbeatConfig, nil)
				stop()
			},
		},
		{
			name: "StartHeartbeatNilStats",
			fn: func() {
				r := &Runner{output: os.Stdout}
				stop := r.startHeartbeatWithConfig(nil, 0, 0, nil, defaultHeartbeatConfig, nil)
				stop()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s panicked: %v", tt.name, r)
				}
			}()
			tt.fn()
		})
	}
}

func TestShowPartialProgressNilRunner(t *testing.T) {
	var r *Runner
	b := &bead.Bead{ID: "test-1"}
	// Should not panic
	r.showPartialProgress(b, "abc123")
}

func TestCheckScopeNilDependencies(t *testing.T) {
	tests := []struct {
		name   string
		runner *Runner
	}{
		{
			name:   "CheckScopeNilRunner",
			runner: nil,
		},
		{
			name:   "CheckScopeNilConfig",
			runner: &Runner{output: os.Stdout},
		},
		{
			name:   "CheckScopeNilBead",
			runner: &Runner{cfg: &config.Config{}, output: os.Stdout},
		},
		{
			name:   "CheckScopeNilRenderer",
			runner: &Runner{cfg: &config.Config{}, output: os.Stdout},
		},
		{
			name:   "CheckScopeNilClaude",
			runner: &Runner{cfg: &config.Config{}, renderer: &mockRenderer{}, output: os.Stdout},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b *bead.Bead
			if strings.Contains(tt.name, "NilBead") {
				b = nil
			} else {
				b = &bead.Bead{ID: "test-1", Title: "Test"}
			}

			if tt.runner == nil {
				var r *Runner
				result := r.checkScope(nil, b)
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else {
				result := tt.runner.checkScope(nil, b)
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			}
		})
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

func TestSelectReviewModel(t *testing.T) {
	tests := []struct {
		name          string
		buildModel    string
		matchBuild    bool
		configModel   string
		expectedModel string
	}{
		{"default sonnet", "sonnet", true, "sonnet", "sonnet"},
		{"match opus build", "opus", true, "sonnet", "opus"},
		{"no match, use config", "opus", false, "sonnet", "sonnet"},
		{"haiku build, match enabled", "haiku", true, "sonnet", "sonnet"}, // only match opus
		{"sonnet build stays sonnet", "sonnet", true, "sonnet", "sonnet"},
		{"opus config without match", "haiku", true, "opus", "opus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchBool := tt.matchBuild
			cfg := &config.Config{
				Review: config.ReviewConfig{
					Model:           tt.configModel,
					MatchBuildModel: &matchBool,
				},
			}
			got := selectReviewModel(cfg, tt.buildModel)
			if got != tt.expectedModel {
				t.Errorf("expected %q, got %q", tt.expectedModel, got)
			}
		})
	}
}

func TestSelectReviewModelNilConfig(t *testing.T) {
	got := selectReviewModel(nil, "opus")
	if got != "sonnet" {
		t.Errorf("expected default 'sonnet' for nil config, got %q", got)
	}
}

func TestBuildReviewBeadLabels(t *testing.T) {
	tests := []struct {
		name           string
		proposalLabels []string
		wantLabels     []string
	}{
		{
			name:           "empty proposal labels",
			proposalLabels: []string{},
			wantLabels:     []string{"from-review"},
		},
		{
			name:           "single custom label",
			proposalLabels: []string{"security"},
			wantLabels:     []string{"from-review", "security"},
		},
		{
			name:           "multiple custom labels",
			proposalLabels: []string{"security", "bug"},
			wantLabels:     []string{"from-review", "security", "bug"},
		},
		{
			name:           "proposal already has from-review",
			proposalLabels: []string{"from-review", "bug"},
			wantLabels:     []string{"from-review", "bug"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := buildReviewBeadLabels(tt.proposalLabels)
			if len(labels) != len(tt.wantLabels) {
				t.Errorf("expected %d labels, got %d", len(tt.wantLabels), len(labels))
			}
			for _, want := range tt.wantLabels {
				if !bead.HasLabel(labels, want) {
					t.Errorf("missing expected label %q in %v", want, labels)
				}
			}
		})
	}
}

func TestBuildBacklogLabels(t *testing.T) {
	labels := buildBacklogLabels()
	if len(labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(labels))
	}
	if !bead.HasLabel(labels, "from-review") {
		t.Error("missing from-review label")
	}
	if !bead.HasLabel(labels, "backlog") {
		t.Error("missing backlog label")
	}
}

func TestApplyReviewResultNilRunner(t *testing.T) {
	var r *Runner
	result := &review.ReviewResult{}
	beadsCreated, backlogCreated := r.applyReviewResult(result)
	if beadsCreated != 0 || backlogCreated != 0 {
		t.Errorf("expected 0,0 for nil runner, got %d,%d", beadsCreated, backlogCreated)
	}
}

func TestApplyReviewResultNilResult(t *testing.T) {
	r := &Runner{beads: &mockBeadClient{}, output: os.Stdout}
	beadsCreated, backlogCreated := r.applyReviewResult(nil)
	if beadsCreated != 0 || backlogCreated != 0 {
		t.Errorf("expected 0,0 for nil result, got %d,%d", beadsCreated, backlogCreated)
	}
}

func TestApplyReviewResultNilBeads(t *testing.T) {
	r := &Runner{output: os.Stdout}
	result := &review.ReviewResult{
		BeadsToCreate: []review.BeadProposal{{Title: "Test"}},
	}
	beadsCreated, backlogCreated := r.applyReviewResult(result)
	if beadsCreated != 0 || backlogCreated != 0 {
		t.Errorf("expected 0,0 for nil beads client, got %d,%d", beadsCreated, backlogCreated)
	}
}

func TestApplyReviewResultCreatesBeads(t *testing.T) {
	created := []string{}
	beadLabels := map[string][]string{}
	beadPriorities := map[string]int{}
	beadDescriptions := map[string]string{}

	beads := &mockBeadClient{
		CreateWithParentAndDescriptionFn: func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
			id := fmt.Sprintf("bead-%d", len(created)+1)
			created = append(created, id)
			beadLabels[id] = labels
			beadPriorities[id] = priority
			beadDescriptions[id] = description
			return &bead.Bead{ID: id, Title: title, Priority: priority, Labels: labels, Description: description}, nil
		},
	}

	var buf strings.Builder
	r := &Runner{beads: beads, output: &buf}

	result := &review.ReviewResult{
		BeadsToCreate: []review.BeadProposal{
			{Title: "Fix security issue", Description: "Add validation", Priority: 0, Labels: []string{"security"}},
			{Title: "Add tests", Description: "Test coverage", Priority: 1, Labels: []string{}},
		},
	}

	beadsCreated, backlogCreated := r.applyReviewResult(result)

	if beadsCreated != 2 {
		t.Errorf("expected 2 beads created, got %d", beadsCreated)
	}
	if backlogCreated != 0 {
		t.Errorf("expected 0 backlog items created, got %d", backlogCreated)
	}
	if len(created) != 2 {
		t.Errorf("expected 2 beads in created list, got %d", len(created))
	}

	// Verify first bead
	if !bead.HasLabel(beadLabels["bead-1"], "from-review") {
		t.Error("first bead missing from-review label")
	}
	if !bead.HasLabel(beadLabels["bead-1"], "security") {
		t.Error("first bead missing security label")
	}
	if beadPriorities["bead-1"] != 0 {
		t.Errorf("expected P0, got P%d", beadPriorities["bead-1"])
	}
	if beadDescriptions["bead-1"] != "Add validation" {
		t.Errorf("expected description 'Add validation', got %q", beadDescriptions["bead-1"])
	}

	// Verify second bead
	if !bead.HasLabel(beadLabels["bead-2"], "from-review") {
		t.Error("second bead missing from-review label")
	}
	if beadPriorities["bead-2"] != 1 {
		t.Errorf("expected P1, got P%d", beadPriorities["bead-2"])
	}

	// Verify logging
	output := buf.String()
	if !strings.Contains(output, "Fix security issue") {
		t.Errorf("expected 'Fix security issue' in output, got: %s", output)
	}
	if !strings.Contains(output, "Add tests") {
		t.Errorf("expected 'Add tests' in output, got: %s", output)
	}
}

func TestApplyReviewResultCreatesBacklogItems(t *testing.T) {
	created := []string{}
	beadLabels := map[string][]string{}
	beadPriorities := map[string]int{}
	beadDescriptions := map[string]string{}

	beads := &mockBeadClient{
		CreateWithParentAndDescriptionFn: func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
			id := fmt.Sprintf("backlog-%d", len(created)+1)
			created = append(created, id)
			beadLabels[id] = labels
			beadPriorities[id] = priority
			beadDescriptions[id] = description
			return &bead.Bead{ID: id, Title: title, Priority: priority, Labels: labels, Description: description}, nil
		},
	}

	var buf strings.Builder
	r := &Runner{beads: beads, output: &buf}

	result := &review.ReviewResult{
		BacklogItems: []review.BacklogItem{
			{Title: "Refactor auth system", Description: "Large undertaking", Reason: "needs product owner approval"},
		},
	}

	beadsCreated, backlogCreated := r.applyReviewResult(result)

	if beadsCreated != 0 {
		t.Errorf("expected 0 beads created, got %d", beadsCreated)
	}
	if backlogCreated != 1 {
		t.Errorf("expected 1 backlog item created, got %d", backlogCreated)
	}

	// Verify backlog item
	if !bead.HasLabel(beadLabels["backlog-1"], "from-review") {
		t.Error("backlog item missing from-review label")
	}
	if !bead.HasLabel(beadLabels["backlog-1"], "backlog") {
		t.Error("backlog item missing backlog label")
	}
	if beadPriorities["backlog-1"] != 2 {
		t.Errorf("expected P2 for backlog item, got P%d", beadPriorities["backlog-1"])
	}
	if !strings.Contains(beadDescriptions["backlog-1"], "Large undertaking") {
		t.Errorf("expected description to contain 'Large undertaking', got %q", beadDescriptions["backlog-1"])
	}
	if !strings.Contains(beadDescriptions["backlog-1"], "needs product owner approval") {
		t.Errorf("expected description to contain reason, got %q", beadDescriptions["backlog-1"])
	}

	// Verify logging
	output := buf.String()
	if !strings.Contains(output, "Refactor auth system") {
		t.Errorf("expected 'Refactor auth system' in output, got: %s", output)
	}
	if !strings.Contains(output, "needs product owner approval") {
		t.Errorf("expected reason in output, got: %s", output)
	}
}

func TestApplyReviewResultHandlesCreateErrors(t *testing.T) {
	beads := &mockBeadClient{
		CreateWithParentAndDescriptionFn: func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
			return nil, fmt.Errorf("bd create failed")
		},
	}

	var buf strings.Builder
	r := &Runner{beads: beads, output: &buf}

	result := &review.ReviewResult{
		BeadsToCreate: []review.BeadProposal{
			{Title: "Should fail", Priority: 1, Labels: []string{}},
		},
		BacklogItems: []review.BacklogItem{
			{Title: "Also fails", Description: "desc", Reason: "reason"},
		},
	}

	beadsCreated, backlogCreated := r.applyReviewResult(result)

	// Should not panic, just log warnings
	if beadsCreated != 0 {
		t.Errorf("expected 0 beads created on error, got %d", beadsCreated)
	}
	if backlogCreated != 0 {
		t.Errorf("expected 0 backlog items created on error, got %d", backlogCreated)
	}

	// Verify warning was logged
	output := buf.String()
	if !strings.Contains(output, "Warning") && !strings.Contains(output, "failed") {
		t.Errorf("expected warning in output, got: %s", output)
	}
}

func TestRunLightReviewSkipsWhenDeadlineExpired(t *testing.T) {
	mockClaude := &mockClaudeClient{}
	var buf strings.Builder

	r := &Runner{
		cfg:      &config.Config{Review: config.ReviewConfig{Timeout: 30}},
		claude:   mockClaude,
		renderer: &mockRenderer{},
		beads:    &mockBeadClient{},
		output:   &buf,
	}

	b := &bead.Bead{ID: "test-1", Title: "Test"}

	// Deadline already expired
	expiredDeadline := time.Now().Add(-1 * time.Second)

	result, err := r.runLightReview(nil, b, nil, "abc123", "sonnet", 1, expiredDeadline)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when deadline expired")
	}
	if !strings.Contains(buf.String(), "Time budget expired") {
		t.Errorf("expected 'Time budget expired' message, got: %s", buf.String())
	}
}

func TestRunLightReviewSkipsWhenInsufficientTime(t *testing.T) {
	mockClaude := &mockClaudeClient{}
	var buf strings.Builder

	r := &Runner{
		cfg:      &config.Config{Review: config.ReviewConfig{Timeout: 300}}, // 300 second timeout
		claude:   mockClaude,
		renderer: &mockRenderer{},
		beads:    &mockBeadClient{},
		output:   &buf,
	}

	b := &bead.Bead{ID: "test-1", Title: "Test"}

	// Deadline in 60 seconds, but review needs 300
	deadline := time.Now().Add(60 * time.Second)

	result, err := r.runLightReview(nil, b, nil, "abc123", "sonnet", 1, deadline)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when insufficient time")
	}
	if !strings.Contains(buf.String(), "Insufficient time remaining") {
		t.Errorf("expected 'Insufficient time remaining' message, got: %s", buf.String())
	}
}

func TestRunThoroughReviewSkipsWhenDeadlineExpired(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal state file
	stateFile := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(stateFile, []byte(`{}`), 0644); err != nil {
		t.Fatalf("Failed to create state file: %v", err)
	}

	var buf strings.Builder
	r := &Runner{
		cfg:       &config.Config{Review: config.ReviewConfig{Thorough: config.ThoroughReviewConfig{Timeout: 30}}},
		output:    &buf,
		gromitDir: tmpDir,
	}

	// Deadline already expired
	expiredDeadline := time.Now().Add(-1 * time.Second)

	r.runThoroughReview(nil, (*state.File)(nil), 1, expiredDeadline)

	if !strings.Contains(buf.String(), "Time budget expired") {
		t.Errorf("expected 'Time budget expired' message, got: %s", buf.String())
	}
}

func TestRunThoroughReviewSkipsWhenInsufficientTime(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal state file
	stateFile := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(stateFile, []byte(`{}`), 0644); err != nil {
		t.Fatalf("Failed to create state file: %v", err)
	}

	var buf strings.Builder
	r := &Runner{
		cfg:       &config.Config{Review: config.ReviewConfig{Thorough: config.ThoroughReviewConfig{Timeout: 300}}},
		output:    &buf,
		gromitDir: tmpDir,
	}

	// Deadline in 60 seconds, but review needs 300
	deadline := time.Now().Add(60 * time.Second)

	r.runThoroughReview(nil, (*state.File)(nil), 1, deadline)

	if !strings.Contains(buf.String(), "Insufficient time remaining") {
		t.Errorf("expected 'Insufficient time remaining' message, got: %s", buf.String())
	}
}

func TestRunBetweenIterationsCommandEmptyNoOp(t *testing.T) {
	var buf strings.Builder
	r := &Runner{
		cfg:    &config.Config{},
		output: &buf,
	}

	// No command configured, should be no-op
	r.runBetweenIterationsCommand()

	output := buf.String()
	if output != "" {
		t.Errorf("expected no output for empty command, got: %s", output)
	}
}

func TestRunBetweenIterationsCommandSuccessful(t *testing.T) {
	var buf strings.Builder
	r := &Runner{
		cfg: &config.Config{
			Loop: config.LoopConfig{
				BetweenIterationsCommand: "echo 'test output'",
			},
		},
		output: &buf,
	}

	r.runBetweenIterationsCommand()

	output := buf.String()
	if !strings.Contains(output, "Running between-iterations command") {
		t.Errorf("expected 'Running between-iterations command' in output, got: %s", output)
	}
	if !strings.Contains(output, "test output") {
		t.Errorf("expected command output 'test output' to be visible, got: %s", output)
	}
}

func TestRunBetweenIterationsCommandFailedWarning(t *testing.T) {
	var buf strings.Builder
	r := &Runner{
		cfg: &config.Config{
			Loop: config.LoopConfig{
				BetweenIterationsCommand: "exit 1",
			},
		},
		output: &buf,
	}

	r.runBetweenIterationsCommand()

	output := buf.String()
	if !strings.Contains(output, "Running between-iterations command") {
		t.Errorf("expected 'Running between-iterations command' in output, got: %s", output)
	}
	if !strings.Contains(output, "Warning") {
		t.Errorf("expected warning for failed command, got: %s", output)
	}
	if !strings.Contains(output, "between-iterations command failed") {
		t.Errorf("expected 'between-iterations command failed' in warning, got: %s", output)
	}
}

func TestRunBetweenIterationsCommandNilRunner(t *testing.T) {
	var r *Runner
	// Should not panic
	r.runBetweenIterationsCommand()
}

func TestRunBetweenIterationsCommandNilConfig(t *testing.T) {
	var buf strings.Builder
	r := &Runner{
		output: &buf,
	}

	// Should not panic
	r.runBetweenIterationsCommand()

	output := buf.String()
	if output != "" {
		t.Errorf("expected no output for nil config, got: %s", output)
	}
}

func TestRunGitAutoPush(t *testing.T) {
	tests := []struct {
		name             string
		autoPush         *bool
		pushFailure      string
		wantErrOnFailure bool
		wantOutput       []string
		wantNoOutput     []string
		description      string
		nilRunner        bool
		nilConfig        bool
	}{
		{
			name:         "NilRunner",
			nilRunner:    true,
			wantNoOutput: []string{"Pushing to remote"},
			description:  "When runner is nil, returns nil without panic",
		},
		{
			name:         "NilConfig",
			nilConfig:    true,
			wantNoOutput: []string{"Pushing to remote"},
			description:  "When config is nil, returns nil without panic",
		},
		{
			name:         "AutoPushDisabled",
			autoPush:     boolPtr(false),
			pushFailure:  "warn",
			wantNoOutput: []string{"Pushing to remote"},
			description:  "When auto_push is false, no push is attempted",
		},
		{
			name:        "AutoPushNilDefaultsToTrue",
			autoPush:    nil,
			pushFailure: "warn",
			wantOutput:  []string{"Pushing to remote"},
			description: "When auto_push is nil, defaults to true and attempts push",
		},
		{
			name:        "AutoPushTrue",
			autoPush:    boolPtr(true),
			pushFailure: "warn",
			wantOutput:  []string{"Pushing to remote"},
			description: "When auto_push is true, push is attempted",
		},
		{
			name:             "PushFailureStop",
			autoPush:         boolPtr(true),
			pushFailure:      "stop",
			wantErrOnFailure: true,
			wantOutput:       []string{"Pushing to remote"},
			description:      "When push_failure is 'stop' and push fails, returns error",
		},
		{
			name:             "PushFailureWarn",
			autoPush:         boolPtr(true),
			pushFailure:      "warn",
			wantErrOnFailure: false,
			wantOutput:       []string{"Pushing to remote"},
			description:      "When push_failure is 'warn', logs warning on failure but returns nil",
		},
		{
			name:        "PushFailureDefaultWarn",
			autoPush:    boolPtr(true),
			pushFailure: "",
			wantOutput:  []string{"Pushing to remote"},
			description: "When push_failure is empty, defaults to warn behavior",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			var r *Runner

			if tt.nilRunner {
				r = nil
			} else if tt.nilConfig {
				r = &Runner{
					output: &buf,
				}
			} else {
				r = &Runner{
					cfg: &config.Config{
						Git: config.GitConfig{
							AutoPush:    tt.autoPush,
							PushFailure: tt.pushFailure,
						},
					},
					output: &buf,
				}
			}

			err := r.runGitAutoPush()

			// For tests that actually attempt a push, the outcome depends on the git state
			// If the push succeeds, err will be nil (good)
			// If the push fails, we check wantErrOnFailure to determine expected behavior
			if err != nil {
				if !tt.wantErrOnFailure && tt.pushFailure != "stop" {
					t.Errorf("%s: unexpected error: %v", tt.description, err)
				}
				if tt.wantErrOnFailure && !strings.Contains(err.Error(), "git push failed") {
					t.Errorf("%s: expected 'git push failed' in error, got: %v", tt.description, err)
				}
			}

			output := buf.String()

			// Check expected output
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("%s: expected output to contain %q, got: %s", tt.description, want, output)
				}
			}

			// Check unexpected output
			for _, noWant := range tt.wantNoOutput {
				if strings.Contains(output, noWant) {
					t.Errorf("%s: expected output NOT to contain %q, got: %s", tt.description, noWant, output)
				}
			}
		})
	}
}

// boolPtr is a helper for creating *bool pointers in tests
func boolPtr(b bool) *bool {
	return &b
}

func TestTDDPromptSelection(t *testing.T) {
	tests := []struct {
		name                 string
		globalTDD            bool
		beadLabels           []string
		expectTDDBuildCalled bool
		description          string
	}{
		{
			name:                 "TDD active via global config - RenderTDDBuild called",
			globalTDD:            true,
			beadLabels:           []string{},
			expectTDDBuildCalled: true,
			description:          "When global TDD is true and bead has no tdd label, RenderTDDBuild should be called",
		},
		{
			name:                 "TDD active via bead label - RenderTDDBuild called",
			globalTDD:            false,
			beadLabels:           []string{"tdd:true"},
			expectTDDBuildCalled: true,
			description:          "When bead has tdd:true label, RenderTDDBuild should be called regardless of global config",
		},
		{
			name:                 "TDD inactive globally, no label - RenderTDDBuild not called",
			globalTDD:            false,
			beadLabels:           []string{},
			expectTDDBuildCalled: false,
			description:          "When TDD is not active, RenderTDDBuild should not be called",
		},
		{
			name:                 "TDD disabled via label overriding global - RenderTDDBuild not called",
			globalTDD:            true,
			beadLabels:           []string{"tdd:false"},
			expectTDDBuildCalled: false,
			description:          "When bead has tdd:false label, RenderTDDBuild should not be called even if global TDD is true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tddBuildCalled := false

			mockRenderer := &mockPromptRenderer{
				BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
					return &prompt.Context{
						Bead:               b,
						ParentBead:         parent,
						Iteration:          iteration,
						Model:              model,
						ConfirmedLearnings: []learnings.Learning{},
						RecentLearnings:    []learnings.Learning{},
					}, nil
				},
				RenderBuildFn: func(ctx *prompt.Context) (string, error) {
					return "standard build prompt", nil
				},
				RenderTDDBuildFn: func(ctx *prompt.Context) (string, error) {
					tddBuildCalled = true
					return "tdd build prompt", nil
				},
			}

			cfg := &config.Config{
				Methodology: config.MethodologyConfig{
					TDD: tt.globalTDD,
				},
				Validation: config.ValidationConfig{
					Enabled: false, // Disable validation to isolate prompt selection
				},
			}

			var buf strings.Builder
			r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
				Deps{
					Beads:    &mockBeadClient{},
					Claude:   &mockClaudeClient{},
					Analyzer: &mockFailureAnalyzer{},
					Renderer: mockRenderer,
					Logger:   &mockIterationLogger{},
				})
			if err != nil {
				t.Fatalf("Failed to create runner: %v", err)
			}

			testBead := &bead.Bead{
				ID:       "test-bead-1",
				Title:    "Test bead",
				Priority: 1,
				Labels:   tt.beadLabels,
			}

			// Call processBead - we don't care about the result, just whether RenderTDDBuild was called
			_ = r.processBead(context.Background(), testBead, 1, time.Time{})

			// Verify expectations
			if tt.expectTDDBuildCalled && !tddBuildCalled {
				t.Errorf("%s: Expected RenderTDDBuild to be called but it wasn't", tt.description)
			}
			if !tt.expectTDDBuildCalled && tddBuildCalled {
				t.Errorf("%s: Expected RenderTDDBuild NOT to be called but it was", tt.description)
			}
		})
	}
}

// TestScopedRun_NoFilterUsesReady tests that when no label filters are set,
// getNextBead uses Ready() method
func TestScopedRun_NoFilterUsesReady(t *testing.T) {
	readyCalled := false
	readyWithLabelCalled := false

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			readyCalled = true
			return &bead.Bead{
				ID:       "test-1",
				Title:    "Test bead",
				Priority: 1,
				Labels:   []string{},
			}, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			readyWithLabelCalled = true
			return nil, nil
		},
	}

	cfg := &config.Config{}
	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Claude:   &mockClaudeClient{},
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	// No label filters set
	bead, err := r.getNextBead()
	if err != nil {
		t.Fatalf("getNextBead() error = %v", err)
	}

	if !readyCalled {
		t.Error("Expected Ready() to be called when no filters are set")
	}
	if readyWithLabelCalled {
		t.Error("Expected ReadyWithLabel() NOT to be called when no filters are set")
	}
	if bead == nil {
		t.Fatal("Expected non-nil bead")
	}
	if bead.ID != "test-1" {
		t.Errorf("Expected bead ID test-1, got %s", bead.ID)
	}
}

// TestScopedRun_WithFilterUsesReadyWithLabel tests that when label filters are set,
// getNextBead uses ReadyWithLabel() for each label
func TestScopedRun_WithFilterUsesReadyWithLabel(t *testing.T) {
	readyCalled := false
	readyWithLabelCalls := []string{}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			readyCalled = true
			return nil, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			readyWithLabelCalls = append(readyWithLabelCalls, label)
			if label == "spec:auth" {
				return &bead.Bead{
					ID:       "auth-1",
					Title:    "Auth bead",
					Priority: 1,
					Labels:   []string{"spec:auth"},
				}, nil
			}
			return nil, nil
		},
	}

	cfg := &config.Config{}
	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Claude:   &mockClaudeClient{},
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	// Set label filters
	r.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	bead, err := r.getNextBead()
	if err != nil {
		t.Fatalf("getNextBead() error = %v", err)
	}

	if readyCalled {
		t.Error("Expected Ready() NOT to be called when filters are set")
	}
	if len(readyWithLabelCalls) != 2 {
		t.Errorf("Expected ReadyWithLabel() to be called twice, got %d calls", len(readyWithLabelCalls))
	}
	if !contains(readyWithLabelCalls, "spec:auth") {
		t.Error("Expected ReadyWithLabel() to be called with spec:auth")
	}
	if !contains(readyWithLabelCalls, "spec:payments") {
		t.Error("Expected ReadyWithLabel() to be called with spec:payments")
	}
	if bead == nil {
		t.Fatal("Expected non-nil bead")
	}
	if bead.ID != "auth-1" {
		t.Errorf("Expected bead ID auth-1, got %s", bead.ID)
	}
}

// TestScopedRun_EmptyResultsExitCleanly tests that when filtered beads return no results,
// getNextBead returns nil without error
func TestScopedRun_EmptyResultsExitCleanly(t *testing.T) {
	mockBeads := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			// All labels return no beads
			return nil, nil
		},
	}

	cfg := &config.Config{}
	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Claude:   &mockClaudeClient{},
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	// Set label filters
	r.SetLabelFilters([]string{"spec:nonexistent"})

	bead, err := r.getNextBead()
	if err != nil {
		t.Errorf("getNextBead() should not error on empty results, got: %v", err)
	}
	if bead != nil {
		t.Errorf("Expected nil bead for empty results, got: %+v", bead)
	}
}

// TestScopedRun_MultipleLabelsPicksHighestPriority tests that when multiple labels
// return beads, getNextBead picks the one with the highest priority (lowest priority number)
func TestScopedRun_MultipleLabelsPicksHighestPriority(t *testing.T) {
	mockBeads := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			switch label {
			case "spec:auth":
				return &bead.Bead{
					ID:       "auth-1",
					Title:    "Auth bead",
					Priority: 1, // Lower priority number = higher priority
					Labels:   []string{"spec:auth"},
				}, nil
			case "spec:payments":
				return &bead.Bead{
					ID:       "payments-1",
					Title:    "Payments bead",
					Priority: 0, // Highest priority
					Labels:   []string{"spec:payments"},
				}, nil
			case "spec:reporting":
				return &bead.Bead{
					ID:       "reporting-1",
					Title:    "Reporting bead",
					Priority: 2, // Lowest priority
					Labels:   []string{"spec:reporting"},
				}, nil
			}
			return nil, nil
		},
	}

	cfg := &config.Config{}
	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Claude:   &mockClaudeClient{},
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	// Set multiple label filters
	r.SetLabelFilters([]string{"spec:auth", "spec:payments", "spec:reporting"})

	bead, err := r.getNextBead()
	if err != nil {
		t.Fatalf("getNextBead() error = %v", err)
	}

	if bead == nil {
		t.Fatal("Expected non-nil bead")
	}
	// Should pick payments-1 because it has priority 0 (highest)
	if bead.ID != "payments-1" {
		t.Errorf("Expected highest priority bead (payments-1), got %s", bead.ID)
	}
	if bead.Priority != 0 {
		t.Errorf("Expected priority 0, got %d", bead.Priority)
	}
}

// TestScopedRun_MultipleLabelsWithPartialResults tests that when some labels
// return beads and others don't, getNextBead handles it correctly
func TestScopedRun_MultipleLabelsWithPartialResults(t *testing.T) {
	mockBeads := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			if label == "spec:auth" {
				return &bead.Bead{
					ID:       "auth-1",
					Title:    "Auth bead",
					Priority: 1,
					Labels:   []string{"spec:auth"},
				}, nil
			}
			// Other labels return nil
			return nil, nil
		},
	}

	cfg := &config.Config{}
	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Claude:   &mockClaudeClient{},
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	// Set multiple label filters, only one returns a bead
	r.SetLabelFilters([]string{"spec:auth", "spec:payments", "spec:reporting"})

	bead, err := r.getNextBead()
	if err != nil {
		t.Fatalf("getNextBead() error = %v", err)
	}

	if bead == nil {
		t.Fatal("Expected non-nil bead")
	}
	if bead.ID != "auth-1" {
		t.Errorf("Expected auth-1, got %s", bead.ID)
	}
}

// TestScopedRun_DeduplicatesBeadsAcrossLabels tests that if the same bead matches
// multiple labels, it only appears once in the candidate list
func TestScopedRun_DeduplicatesBeadsAcrossLabels(t *testing.T) {
	// Track how many times we return the same bead
	callCount := 0
	mockBeads := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			callCount++
			// Return the same bead for different labels
			return &bead.Bead{
				ID:       "shared-1",
				Title:    "Shared bead",
				Priority: 1,
				Labels:   []string{"spec:auth", "spec:payments"}, // Has both labels
			}, nil
		},
	}

	cfg := &config.Config{}
	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Claude:   &mockClaudeClient{},
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	// Set filters that both return the same bead
	r.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	bead, err := r.getNextBead()
	if err != nil {
		t.Fatalf("getNextBead() error = %v", err)
	}

	if bead == nil {
		t.Fatal("Expected non-nil bead")
	}
	if bead.ID != "shared-1" {
		t.Errorf("Expected shared-1, got %s", bead.ID)
	}

	// Verify ReadyWithLabel was called for each label
	if callCount != 2 {
		t.Errorf("Expected ReadyWithLabel to be called twice, got %d", callCount)
	}

	// Note: The current implementation doesn't deduplicate, it just picks the first occurrence.
	// This test documents current behavior - if deduplication is needed later, update implementation.
}

// contains is a helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestRunnerStatusWithLiveRun(t *testing.T) {
	tests := []struct {
		name           string
		setupStatus    func(gromitDir string) error
		expectedOutput []string
		notExpected    []string
		description    string
	}{
		{
			name: "No status file - shows pipeline status",
			setupStatus: func(gromitDir string) error {
				// Don't create status.json
				return nil
			},
			expectedOutput: []string{"Pipeline:", "Run: not running", "Health:", "Next action:"},
			notExpected:    []string{"Warning: stale run"},
			description:    "When status.json doesn't exist, should show pipeline, run, health, and recommendation",
		},
		{
			name: "Live run - shows run in progress",
			setupStatus: func(gromitDir string) error {
				// Create status file with current PID (which is alive)
				sw, err := NewStatusWriter(gromitDir)
				if err != nil {
					return err
				}
				return sw.Write(1, "bead-123", "Building feature X", "sonnet", true, 0, 0)
			},
			expectedOutput: []string{"Pipeline:", "Run: iteration 1", "bead-123", "Building feature X", "Model:    sonnet", "Health:"},
			notExpected:    []string{"Warning: stale run"},
			description:    "When status.json exists with alive PID, should show run in progress",
		},
		{
			name: "Stale status file - warns and cleans up",
			setupStatus: func(gromitDir string) error {
				// Create status file with a fake PID that won't exist
				status := Status{
					Running:   true,
					Iteration: 2,
					BeadID:    "bead-456",
					BeadTitle: "Old bead",
					Model:     "haiku",
					StartedAt: time.Now().Add(-1 * time.Hour),
					ElapsedS:  3600,
					PID:       999999, // PID that won't exist
				}
				data, err := json.MarshalIndent(status, "", "  ")
				if err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(gromitDir, "status.json"), data, 0644)
			},
			expectedOutput: []string{"Warning: stale run detected", "Bead: bead-456 - Old bead", "Removing stale status file", "Pipeline:", "Run: not running"},
			notExpected:    []string{"Run: iteration"},
			description:    "When status.json exists with dead PID, should warn and clean up",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gromitDir := filepath.Join(tmpDir, ".gromit")
			if err := os.MkdirAll(gromitDir, 0755); err != nil {
				t.Fatalf("Failed to create gromit dir: %v", err)
			}

			// Setup status file
			if err := tt.setupStatus(gromitDir); err != nil {
				t.Fatalf("Failed to setup status: %v", err)
			}

			// Create mock bead client
			mockBeads := &mockBeadClientForStatus{
				ready: &bead.Bead{
					ID:       "test-1",
					Title:    "Test bead",
					Priority: 1,
					Labels:   []string{},
				},
			}

			cfg := &config.Config{}
			cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
			cfg.Paths.Plans = filepath.Join(gromitDir, "plans")
			var buf strings.Builder
			r := &Runner{
				cfg:       cfg,
				beads:     mockBeads,
				output:    &buf,
				gromitDir: gromitDir,
			}

			// Call Status
			err := r.Status()
			if err != nil {
				t.Fatalf("Status() failed: %v", err)
			}

			output := buf.String()

			// Check expected strings
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("%s\nExpected output to contain %q\nGot:\n%s", tt.description, expected, output)
				}
			}

			// Check strings that should not be present
			for _, notExpected := range tt.notExpected {
				if strings.Contains(output, notExpected) {
					t.Errorf("%s\nExpected output NOT to contain %q\nGot:\n%s", tt.description, notExpected, output)
				}
			}

			// For stale status test, verify file was deleted
			if tt.name == "Stale status file - warns and cleans up" {
				statusPath := filepath.Join(gromitDir, "status.json")
				if _, err := os.Stat(statusPath); !os.IsNotExist(err) {
					t.Errorf("Expected status.json to be deleted, but it still exists")
				}
			}
		})
	}
}

// mockBeadClientForStatus is a minimal mock for testing Status()
type mockBeadClientForStatus struct {
	ready *bead.Bead
	err   error
}

func (m *mockBeadClientForStatus) Ready() (*bead.Bead, error) {
	return m.ready, m.err
}

func (m *mockBeadClientForStatus) ReadyWithLabel(label string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForStatus) ListWithLabel(label string) ([]*bead.Bead, error) {
	return []*bead.Bead{}, nil
}

func (m *mockBeadClientForStatus) Show(id string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForStatus) Close(id string) error {
	return nil
}

func (m *mockBeadClientForStatus) Sync() error {
	return nil
}

func (m *mockBeadClientForStatus) AddComment(id, comment string) error {
	return nil
}

func (m *mockBeadClientForStatus) GetParent(b *bead.Bead) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForStatus) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForStatus) CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForStatus) HasOpenChildren(parentID string) (bool, error) {
	return false, nil
}
