package validate

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// fakeCommandRunner is a test double for CommandRunner.
type fakeCommandRunner struct {
	results []commandRunResult
	callIdx int
}

type commandRunResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func (f *fakeCommandRunner) Run(ctx context.Context, command, workDir string) (string, string, int, error) {
	if f.callIdx >= len(f.results) {
		return "", "", 0, nil
	}
	result := f.results[f.callIdx]
	f.callIdx++
	return result.stdout, result.stderr, result.exitCode, result.err
}

// TestValidate_CleanPass_ReturnsProceed verifies that when all commands exit 0,
// the stage returns Proceed with no ValidationFailures.
func TestValidate_CleanPass_ReturnsProceed(t *testing.T) {
	runner := &fakeCommandRunner{
		results: []commandRunResult{
			{stdout: "", stderr: "", exitCode: 0, err: nil},
		},
	}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	in := pipeline.Input{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test bead"},
		Config: cfg,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}
	if len(out.ValidationFailures) != 0 {
		t.Errorf("ValidationFailures = %v, want empty", out.ValidationFailures)
	}
}

func TestValidate_EmitsValidationStartAndPassEvents(t *testing.T) {
	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	runner := &fakeCommandRunner{
		results: []commandRunResult{
			{stdout: "", stderr: "", exitCode: 0, err: nil},
		},
	}
	stage := New(runner, io.Discard)

	beadID := "validate-event-success"
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	in := pipeline.Input{
		Bead:    &bead.Bead{ID: beadID, Title: "Event bead"},
		Config:  cfg,
		Emitter: emitter,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Fatalf("Decision = %v, want Proceed", out.Decision)
	}

	emitted := drainEvents(t, ch)
	var (
		startIdx = -1
		passIdx  = -1
		startEvt *events.ValidationStartEvent
		passEvt  *events.ValidationPassEvent
	)
	for idx, evt := range emitted {
		switch e := evt.(type) {
		case *events.ValidationStartEvent:
			startIdx = idx
			startEvt = e
		case *events.ValidationPassEvent:
			passIdx = idx
			passEvt = e
		}
	}
	if startEvt == nil {
		t.Fatal("expected ValidationStartEvent to be emitted")
	}
	if passEvt == nil {
		t.Fatal("expected ValidationPassEvent to be emitted")
	}
	if startIdx > passIdx {
		t.Fatalf("ValidationPassEvent emitted before ValidationStartEvent (%d > %d)", passIdx, startIdx)
	}
	if startEvt.BeadID != beadID {
		t.Fatalf("ValidationStartEvent.BeadID = %q, want %q", startEvt.BeadID, beadID)
	}
	if len(startEvt.Commands) != len(cfg.Validation.Commands) {
		t.Fatalf("ValidationStartEvent.Commands = %v, want %v", startEvt.Commands, cfg.Validation.Commands)
	}
	if passEvt.Duration < 0 {
		t.Fatalf("ValidationPassEvent.Duration = %v, want non-negative", passEvt.Duration)
	}
}

func TestValidate_EmitsValidationFailEventOnFailure(t *testing.T) {
	emitter := events.NewEmitter()
	defer emitter.Close()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	runner := &fakeCommandRunner{
		results: []commandRunResult{
			{
				stdout:   "should fail",
				stderr:   "failure detail",
				exitCode: 1,
				err:      nil,
			},
		},
	}
	stage := New(runner, io.Discard)

	beadID := "validate-event-fail"
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	in := pipeline.Input{
		Bead:    &bead.Bead{ID: beadID, Title: "Fail bead"},
		Config:  cfg,
		Emitter: emitter,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Block {
		t.Fatalf("Decision = %v, want Block on validation failure", out.Decision)
	}

	emitted := drainEvents(t, ch)
	var (
		startIdx = -1
		failIdx  = -1
		failEvt  *events.ValidationFailEvent
		passEvt  *events.ValidationPassEvent
	)
	for idx, evt := range emitted {
		switch e := evt.(type) {
		case *events.ValidationStartEvent:
			startIdx = idx
		case *events.ValidationFailEvent:
			failIdx = idx
			failEvt = e
		case *events.ValidationPassEvent:
			passEvt = e
		}
	}

	if startIdx == -1 {
		t.Fatal("expected ValidationStartEvent to be emitted")
	}
	if failIdx == -1 {
		t.Fatal("expected ValidationFailEvent to be emitted")
	}
	if startIdx > failIdx {
		t.Fatalf("ValidationFailEvent emitted before ValidationStartEvent (%d < %d)", failIdx, startIdx)
	}
	if passEvt != nil {
		t.Fatalf("unexpected ValidationPassEvent emitted on failure")
	}
	if failEvt.BeadID != beadID {
		t.Fatalf("ValidationFailEvent.BeadID = %q, want %q", failEvt.BeadID, beadID)
	}
	expectedOutput := strings.Join(out.ValidationFailures, "\n")
	if failEvt.Output != expectedOutput {
		t.Fatalf("ValidationFailEvent.Output = %q, want %q", failEvt.Output, expectedOutput)
	}
}

// TestValidate_SingleCommandFailure_ReturnsBlockWithSummaries verifies that when any
// command fails (exit code 1), the stage returns Block with ValidationFailures populated.
func TestValidate_SingleCommandFailure_ReturnsBlockWithSummaries(t *testing.T) {
	runner := &fakeCommandRunner{
		results: []commandRunResult{
			{
				stdout:   "test output",
				stderr:   "test error",
				exitCode: 1,
				err:      nil,
			},
		},
	}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	in := pipeline.Input{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test bead"},
		Config: cfg,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Block {
		t.Errorf("Decision = %v, want Block on validation failure", out.Decision)
	}
	if len(out.ValidationFailures) == 0 {
		t.Errorf("ValidationFailures is empty; want non-empty summaries")
	}
}

// TestValidate_NilBead_ReturnsProceed verifies that when bead is nil,
// the stage returns Proceed without running commands.
func TestValidate_NilBead_ReturnsProceed(t *testing.T) {
	runner := &fakeCommandRunner{}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	in := pipeline.Input{
		Bead:   nil,
		Config: cfg,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed for nil bead", out.Decision)
	}
	if runner.callIdx != 0 {
		t.Errorf("CommandRunner.Run was called; want no calls for nil bead")
	}
}

// TestValidate_DisabledInConfig_ReturnsProceed verifies that when validation is disabled,
// the stage returns Proceed without running commands.
func TestValidate_DisabledInConfig_ReturnsProceed(t *testing.T) {
	runner := &fakeCommandRunner{}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  false,
			Commands: []string{"go test ./..."},
		},
	}
	in := pipeline.Input{
		Bead:   &bead.Bead{ID: "test-1", Title: "Test bead"},
		Config: cfg,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed when validation disabled", out.Decision)
	}
	if runner.callIdx != 0 {
		t.Errorf("CommandRunner.Run was called; want no calls when disabled")
	}
}

// TestValidate_MultiCommandPass_AllPassReturnsProceed verifies that when multiple commands
// are configured and all exit 0, the stage returns Proceed with no ValidationFailures.
func TestValidate_MultiCommandPass_AllPassReturnsProceed(t *testing.T) {
	runner := &fakeCommandRunner{
		results: []commandRunResult{
			{stdout: "test 1 passed", stderr: "", exitCode: 0, err: nil},
			{stdout: "test 2 passed", stderr: "", exitCode: 0, err: nil},
			{stdout: "lint passed", stderr: "", exitCode: 0, err: nil},
		},
	}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./...", "go test ./tests/...", "golangci-lint run"},
		},
	}
	in := pipeline.Input{
		Bead:   &bead.Bead{ID: "test-multi", Title: "Multi command test"},
		Config: cfg,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed when all commands pass", out.Decision)
	}
	if len(out.ValidationFailures) != 0 {
		t.Errorf("ValidationFailures = %v, want empty", out.ValidationFailures)
	}
	// Verify all three commands were executed
	if runner.callIdx != 3 {
		t.Errorf("Expected 3 command executions, got %d", runner.callIdx)
	}
}

// TestValidate_MultiCommandPartialFailure_StopsAtFirstFailureReturnBlock verifies that when
// multiple commands are configured, the stage stops at the first failure and returns Block
// with summary only from the failing command, not from previously passing commands.
func TestValidate_MultiCommandPartialFailure_StopsAtFirstFailureReturnBlock(t *testing.T) {
	runner := &fakeCommandRunner{
		results: []commandRunResult{
			{stdout: "ok      github.com/example/test 0.123s", stderr: "", exitCode: 0, err: nil},
			{stdout: "--- FAIL: TestSomething (0.001s)\nFAIL\tgithub.com/example/test\nFAIL", stderr: "", exitCode: 1, err: nil},
			{stdout: "should not run", stderr: "", exitCode: 0, err: nil}, // Should not be called
		},
	}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./...", "go test ./tests/...", "golangci-lint run"},
		},
	}
	in := pipeline.Input{
		Bead:   &bead.Bead{ID: "test-partial-fail", Title: "Partial failure test"},
		Config: cfg,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Block {
		t.Errorf("Decision = %v, want Block on second command failure", out.Decision)
	}
	if len(out.ValidationFailures) == 0 {
		t.Fatalf("ValidationFailures is empty; want non-empty summaries")
	}
	// Verify only two commands were executed (stopped at second failure)
	if runner.callIdx != 2 {
		t.Errorf("Expected 2 command executions (stopped at failure), got %d", runner.callIdx)
	}
	// Verify ValidationFailures contain output only from the failed command
	summary := out.ValidationFailures[0]
	if len(summary) == 0 {
		t.Errorf("ValidationFailure summary is empty")
	}
	// The summary should indicate the second command failed
	if !contains(summary, "FAIL") {
		t.Errorf("Summary missing FAIL indicator: %s", summary)
	}
}

// TestValidate_CommandTimeout_ProducesBlockWithTimeoutMessage verifies that when a command
// exceeds the configured timeout, it is treated as a failure returning Block with a
// timeout-indicating message in ValidationFailures.
func TestValidate_CommandTimeout_ProducesBlockWithTimeoutMessage(t *testing.T) {
	runner := &fakeCommandRunner{
		results: []commandRunResult{
			// First command times out (context.DeadlineExceeded error)
			{stdout: "", stderr: "context deadline exceeded", exitCode: 0, err: context.DeadlineExceeded},
		},
	}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	in := pipeline.Input{
		Bead:   &bead.Bead{ID: "test-timeout", Title: "Timeout test"},
		Config: cfg,
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Block {
		t.Errorf("Decision = %v, want Block on timeout", out.Decision)
	}
	if len(out.ValidationFailures) == 0 {
		t.Fatalf("ValidationFailures is empty; want timeout message")
	}
	// Verify the timeout message is present
	summary := out.ValidationFailures[0]
	if !contains(summary, "timeout") && !contains(summary, "deadline exceeded") {
		t.Errorf("Summary missing timeout indication: %s", summary)
	}
}

// contains is a helper to check if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func drainEvents(t *testing.T, ch chan events.Event) []events.Event {
	t.Helper()
	var emitted []events.Event
	timeout := time.After(25 * time.Millisecond)
	for {
		select {
		case evt := <-ch:
			emitted = append(emitted, evt)
		case <-timeout:
			return emitted
		}
	}
}
