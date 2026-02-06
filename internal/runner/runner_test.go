package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/danabrams/ralph-runner/internal/config"
	"github.com/danabrams/ralph-runner/internal/logger"
	"github.com/danabrams/ralph-runner/internal/review"
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
	// This test runs in the ralph-runner repo, so git HEAD should be available
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
		name     string
		fn       func() error
		wantErr  bool
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
				result := r.processBead(nil, b, 1)
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
		name         string
		runner       *Runner
		bead         *bead.Bead
		subTasks     []SubTask
		expectedErr  string
		nilRunner    bool
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
				result := r.processBead(nil, b, 1)
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
				result := r.processBead(nil, b, 1)
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
				result := r.processBead(nil, b, 1)
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
		{
			name: "StatusNilBeads",
			runner: &Runner{
				cfg:    &config.Config{},
				output: os.Stdout,
			},
			method: func(r *Runner) error {
				return r.Status()
			},
			expectedError: "beads client is nil",
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
