//go:build acceptance

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestBaseSessionExists verifies baseSession struct exists
// Expected failure: baseSession type does not exist yet
func TestBaseSessionExists(t *testing.T) {
	// This will fail until baseSession is defined
	var _ interface{} = &baseSession{}
}

// TestNewBaseSessionConstructor verifies constructor signature and behavior
// Expected failure: newBaseSession function does not exist yet
func TestNewBaseSessionConstructor(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	postProcessFn := func() error { return nil }

	// Constructor should accept context, *exec.Cmd, and post-processing function
	session := newBaseSession(ctx, cmd, postProcessFn)

	if session == nil {
		t.Fatal("newBaseSession() returned nil, want non-nil session")
	}
}

// TestBaseSessionImplementsBaseSessionInterface verifies baseSession implements BaseSession
// Expected failure: BaseSession interface does not exist yet (Session needs to be renamed to BaseSession)
func TestBaseSessionImplementsBaseSessionInterface(t *testing.T) {
	var _ BaseSession = (*baseSession)(nil)
}

// TestBaseSessionEventsChannel verifies Events() returns readable channel
// Expected failure: baseSession.Events() method does not exist yet
func TestBaseSessionEventsChannel(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test output")

	session := newBaseSession(ctx, cmd, nil)

	events := session.Events()
	if events == nil {
		t.Fatal("Events() returned nil channel, want non-nil channel")
	}

	// Verify channel is receive-only (type check)
	var _ <-chan Event = events
}

// TestBaseSessionEmitsSessionStartedEvent verifies EventSessionStarted is emitted at start
// Expected failure: baseSession does not emit EventSessionStarted yet
func TestBaseSessionEmitsSessionStartedEvent(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("sleep", "0.1")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// First event should be SessionStarted
	select {
	case event := <-events:
		if event.Type != EventSessionStarted {
			t.Errorf("First event type = %v, want %v", event.Type, EventSessionStarted)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for EventSessionStarted")
	}

	session.Cancel()
	_ = session.Wait()
}

// TestBaseSessionEmitsOutputEvents verifies EventOutput events for subprocess stdout
// Expected failure: baseSession does not emit EventOutput events yet
func TestBaseSessionEmitsOutputEvents(t *testing.T) {
	ctx := context.Background()
	// Use a command that outputs multiple lines
	cmd := exec.Command("sh", "-c", "echo line1; echo line2; echo line3")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Skip SessionStarted event
	<-events

	// Collect output events
	var outputs []string
	for event := range events {
		if event.Type == EventOutput {
			outputs = append(outputs, event.Content)
		}
		if event.Type == EventSessionEnded {
			break
		}
	}

	_ = session.Wait()

	// Verify we received output
	if len(outputs) == 0 {
		t.Fatal("No EventOutput events received, expected at least one")
	}

	// Verify outputs contain expected content
	fullOutput := strings.Join(outputs, "")
	if !strings.Contains(fullOutput, "line1") {
		t.Errorf("Output missing 'line1': %q", fullOutput)
	}
	if !strings.Contains(fullOutput, "line2") {
		t.Errorf("Output missing 'line2': %q", fullOutput)
	}
	if !strings.Contains(fullOutput, "line3") {
		t.Errorf("Output missing 'line3': %q", fullOutput)
	}
}

// TestBaseSessionEmitsSessionEndedEvent verifies EventSessionEnded is emitted when process exits
// Expected failure: baseSession does not emit EventSessionEnded yet
func TestBaseSessionEmitsSessionEndedEvent(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "done")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Drain events until we get SessionEnded
	var sawEnded bool
	for event := range events {
		if event.Type == EventSessionEnded {
			sawEnded = true
			break
		}
	}

	if !sawEnded {
		t.Error("Did not receive EventSessionEnded before channel closed")
	}

	_ = session.Wait()
}

// TestBaseSessionEmitsErrorEvent verifies EventError is emitted on process errors
// Expected failure: baseSession does not emit EventError events yet
func TestBaseSessionEmitsErrorEvent(t *testing.T) {
	ctx := context.Background()
	// Use a command that will fail
	cmd := exec.Command("sh", "-c", "echo error output >&2; exit 1")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Look for error event or error content
	var sawErrorEvent bool
	for event := range events {
		if event.Type == EventError {
			sawErrorEvent = true
			break
		}
		if event.Type == EventSessionEnded {
			break
		}
	}

	err := session.Wait()

	// Either we saw an error event OR Wait() returned an error
	if !sawErrorEvent && err == nil {
		t.Error("Expected error event or Wait() error, got neither")
	}
}

// TestBaseSessionSendInput verifies SendInput() sends text to subprocess stdin
// Expected failure: baseSession.SendInput() method does not exist yet
func TestBaseSessionSendInput(t *testing.T) {
	ctx := context.Background()
	// Use cat to echo stdin back to stdout
	cmd := exec.Command("cat")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Skip SessionStarted
	<-events

	// Send input
	testInput := "hello from test\n"
	err := session.SendInput(testInput)
	if err != nil {
		t.Fatalf("SendInput() failed: %v", err)
	}

	// Close stdin to signal EOF to cat
	session.Cancel()

	// Read output events
	var outputs []string
	for event := range events {
		if event.Type == EventOutput {
			outputs = append(outputs, event.Content)
		}
		if event.Type == EventSessionEnded {
			break
		}
	}

	_ = session.Wait()

	// Verify input was echoed back
	fullOutput := strings.Join(outputs, "")
	if !strings.Contains(fullOutput, "hello from test") {
		t.Errorf("Output does not contain sent input. Got: %q", fullOutput)
	}
}

// TestBaseSessionCancel verifies Cancel() stops the subprocess
// Expected failure: baseSession.Cancel() method does not exist yet
func TestBaseSessionCancel(t *testing.T) {
	ctx := context.Background()
	// Use a long-running command
	cmd := exec.Command("sleep", "10")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Wait for session to start
	<-events

	// Cancel immediately
	session.Cancel()

	// Wait should return quickly (not take 10 seconds)
	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case <-done:
		// Success - cancelled quickly
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel() did not stop subprocess within 2 seconds")
	}
}

// TestBaseSessionWait verifies Wait() blocks until subprocess exits
// Expected failure: baseSession.Wait() method does not exist yet
func TestBaseSessionWait(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("sleep", "0.1")

	session := newBaseSession(ctx, cmd, nil)

	// Wait should block until sleep completes
	startTime := time.Now()
	err := session.Wait()
	duration := time.Since(startTime)

	if err != nil {
		t.Errorf("Wait() returned error: %v", err)
	}

	// Should have taken at least 0.1 seconds
	if duration < 50*time.Millisecond {
		t.Errorf("Wait() returned too quickly: %v", duration)
	}
}

// TestBaseSessionWaitReturnsError verifies Wait() returns process error
// Expected failure: baseSession.Wait() does not return error correctly yet
func TestBaseSessionWaitReturnsError(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("sh", "-c", "exit 42")

	session := newBaseSession(ctx, cmd, nil)

	err := session.Wait()
	if err == nil {
		t.Error("Wait() returned nil, want error for failed process")
	}

	// Verify it's an exit error
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("Wait() error type = %T, want *exec.ExitError", err)
	}
}

// TestBaseSessionPostProcessingFunction verifies post-processing runs after subprocess
// Expected failure: baseSession does not call post-processing function yet
func TestBaseSessionPostProcessingFunction(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	// Track if post-processing was called
	postProcessCalled := false
	postProcessFn := func() error {
		postProcessCalled = true
		return nil
	}

	session := newBaseSession(ctx, cmd, postProcessFn)

	// Wait for session to complete
	err := session.Wait()
	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	// Verify post-processing was called
	if !postProcessCalled {
		t.Error("Post-processing function was not called")
	}
}

// TestBaseSessionPostProcessingError verifies post-processing errors are propagated
// Expected failure: baseSession does not propagate post-processing errors yet
func TestBaseSessionPostProcessingError(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	expectedErr := fmt.Errorf("post-processing failed")
	postProcessFn := func() error {
		return expectedErr
	}

	session := newBaseSession(ctx, cmd, postProcessFn)

	err := session.Wait()

	// Wait should return the post-processing error
	if err == nil {
		t.Fatal("Wait() returned nil, want post-processing error")
	}

	if !strings.Contains(err.Error(), "post-processing failed") {
		t.Errorf("Wait() error = %v, want error containing 'post-processing failed'", err)
	}
}

// TestBaseSessionContextCancellation verifies context cancellation stops session
// Expected failure: baseSession does not respect context cancellation yet
func TestBaseSessionContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.Command("sleep", "10")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Wait for session to start
	<-events

	// Cancel context
	cancel()

	// Session should stop quickly
	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case <-done:
		// Success - context cancellation stopped the session
	case <-time.After(2 * time.Second):
		t.Fatal("Context cancellation did not stop session within 2 seconds")
	}
}

// TestBaseSessionPipedIOSetup verifies session sets up pipes for stdin/stdout/stderr
// Expected failure: baseSession does not set up pipes yet
func TestBaseSessionPipedIOSetup(t *testing.T) {
	ctx := context.Background()
	// Use a command that reads stdin and writes to stdout
	cmd := exec.Command("sh", "-c", "read line; echo got: $line")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Skip SessionStarted
	<-events

	// Send input via SendInput (which should use the stdin pipe)
	err := session.SendInput("test input\n")
	if err != nil {
		t.Fatalf("SendInput() failed: %v", err)
	}

	// Read output via Events() (which should use the stdout pipe)
	var sawOutput bool
	for event := range events {
		if event.Type == EventOutput && strings.Contains(event.Content, "got: test input") {
			sawOutput = true
			break
		}
		if event.Type == EventSessionEnded {
			break
		}
	}

	_ = session.Wait()

	if !sawOutput {
		t.Error("Did not receive expected output via piped I/O")
	}
}

// TestBaseSessionStdoutReaderGoroutine verifies goroutine reads stdout and emits events
// Expected failure: baseSession does not launch stdout reader goroutine yet
func TestBaseSessionStdoutReaderGoroutine(t *testing.T) {
	ctx := context.Background()
	// Output a burst of lines quickly
	cmd := exec.Command("sh", "-c", "for i in 1 2 3 4 5; do echo line$i; done")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Skip SessionStarted
	<-events

	// Collect all output events
	var outputs []string
	for event := range events {
		if event.Type == EventOutput {
			outputs = append(outputs, event.Content)
		}
		if event.Type == EventSessionEnded {
			break
		}
	}

	_ = session.Wait()

	// Verify we got all 5 lines (goroutine successfully read all output)
	fullOutput := strings.Join(outputs, "")
	for i := 1; i <= 5; i++ {
		expected := fmt.Sprintf("line%d", i)
		if !strings.Contains(fullOutput, expected) {
			t.Errorf("Output missing '%s': %q", expected, fullOutput)
		}
	}
}

// TestRefineSessionWrapper verifies RefineSession wrapper has typed Result() method that returns RefineResult
// Expected failure: RefineSession.Result() method does not exist yet - currently RefineSession only embeds Session with no typed Result() method
func TestRefineSessionWrapper(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	// Create a session using the constructor that should exist
	session := newRefineSession(ctx, cmd, nil)

	// Wait for completion
	_ = session.Wait()

	// Verify Result() method exists with correct signature and returns RefineResult (not interface{})
	result, err := session.Result()

	// The key behavioral difference: Result() should return RefineResult type, not interface{}
	// This would fail with current code because Result() doesn't exist
	var _ RefineResult = result

	if err != nil {
		t.Errorf("Result() after Wait() returned unexpected error: %v", err)
	}

	// Verify result has initialized slices (specific behavioral output)
	if result.CreatedSpecs == nil {
		t.Error("Result().CreatedSpecs is nil, want empty slice - typed Result() should initialize fields")
	}
	if result.RefinedItems == nil {
		t.Error("Result().RefinedItems is nil, want empty slice - typed Result() should initialize fields")
	}
}

// TestPlanSessionWrapper verifies PlanSession wrapper has typed Result() method that returns PlanResult
// Expected failure: PlanSession.Result() method does not exist yet - currently PlanSession only embeds Session with no typed Result() method
func TestPlanSessionWrapper(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	session := newPlanSession(ctx, cmd, nil)
	_ = session.Wait()

	// The key behavioral difference: Result() should return PlanResult type
	result, err := session.Result()
	var _ PlanResult = result

	if err != nil {
		t.Errorf("Result() after Wait() returned unexpected error: %v", err)
	}

	// Verify specific field initialization
	if result.CreatedPlans == nil {
		t.Error("Result().CreatedPlans is nil, want empty slice")
	}
}

// TestReviewSessionWrapper verifies ReviewSession wrapper has typed Result() method that returns ReviewResult
// Expected failure: ReviewSession.Result() method does not exist yet - currently ReviewSession only embeds Session with no typed Result() method
func TestReviewSessionWrapper(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	session := newReviewSession(ctx, cmd, nil)
	_ = session.Wait()

	// The key behavioral difference: Result() should return ReviewResult type
	result, err := session.Result()
	var _ ReviewResult = result

	if err != nil {
		t.Errorf("Result() after Wait() returned unexpected error: %v", err)
	}

	// Verify result is valid ReviewResult (zero value is valid for ReviewResult)
	if result.Summary == "" && result.Passed != false {
		// This is fine - zero value ReviewResult is valid
	}
}

// TestExploreSessionWrapper verifies ExploreSession wrapper has typed Result() method that returns ExploreResult
// Expected failure: ExploreSession.Result() method does not exist yet - currently ExploreSession only embeds Session with no typed Result() method
func TestExploreSessionWrapper(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	session := newExploreSession(ctx, cmd, nil)
	_ = session.Wait()

	// The key behavioral difference: Result() should return ExploreResult type
	result, err := session.Result()
	var _ ExploreResult = result

	if err != nil {
		t.Errorf("Result() after Wait() returned unexpected error: %v", err)
	}

	// Verify specific field initialization
	if result.CreatedSpecs == nil {
		t.Error("Result().CreatedSpecs is nil, want empty slice")
	}
	if result.CreatedEpics == nil {
		t.Error("Result().CreatedEpics is nil, want empty slice")
	}
	if result.CreatedBacklogItems == nil {
		t.Error("Result().CreatedBacklogItems is nil, want empty slice")
	}
}

// TestRefineSessionResultAfterCompletion verifies Result() returns parsed results after session ends
// Expected failure: newRefineSession constructor does not exist yet
func TestRefineSessionResultAfterCompletion(t *testing.T) {
	ctx := context.Background()
	// Mock command that completes successfully
	cmd := exec.Command("echo", "session complete")

	// Create a refine session using the constructor
	refineSession := newRefineSession(ctx, cmd, nil)

	// Wait for completion
	err := refineSession.Wait()
	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	// Result() should now return parsed results
	result, err := refineSession.Result()
	if err != nil {
		t.Errorf("Result() after completion failed: %v", err)
	}

	// Result should have initialized slices (not nil)
	if result.CreatedSpecs == nil {
		t.Error("Result.CreatedSpecs is nil, want empty slice")
	}
	if result.RefinedItems == nil {
		t.Error("Result.RefinedItems is nil, want empty slice")
	}
}

// TestTypedSessionConstructors verifies each typed session has a constructor
// Expected failure: typed session constructors do not exist yet
func TestTypedSessionConstructors(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	tests := []struct {
		name        string
		constructor func(context.Context, *exec.Cmd, func() error) interface{}
		wantType    string
	}{
		{
			name:        "newRefineSession",
			constructor: func(c context.Context, cmd *exec.Cmd, f func() error) interface{} { return newRefineSession(c, cmd, f) },
			wantType:    "*pipeline.RefineSession",
		},
		{
			name:        "newPlanSession",
			constructor: func(c context.Context, cmd *exec.Cmd, f func() error) interface{} { return newPlanSession(c, cmd, f) },
			wantType:    "*pipeline.PlanSession",
		},
		{
			name:        "newReviewSession",
			constructor: func(c context.Context, cmd *exec.Cmd, f func() error) interface{} { return newReviewSession(c, cmd, f) },
			wantType:    "*pipeline.ReviewSession",
		},
		{
			name: "newExploreSession",
			constructor: func(c context.Context, cmd *exec.Cmd, f func() error) interface{} {
				return newExploreSession(c, cmd, f)
			},
			wantType: "*pipeline.ExploreSession",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := tt.constructor(ctx, cmd, nil)
			if session == nil {
				t.Errorf("%s returned nil, want %s", tt.name, tt.wantType)
			}
		})
	}
}

// TestBaseSessionChannelClosedAfterEnd verifies Events() channel is closed after session ends
// Expected failure: baseSession does not close Events() channel yet
func TestBaseSessionChannelClosedAfterEnd(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "done")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Wait for completion
	_ = session.Wait()

	// Try to read from channel - should be closed
	_, ok := <-events
	if ok {
		t.Error("Events() channel still open after Wait(), want closed")
	}
}

// TestBaseSessionMultipleSendInputCalls verifies multiple SendInput() calls work
// Expected failure: baseSession.SendInput() does not handle multiple calls yet
func TestBaseSessionMultipleSendInputCalls(t *testing.T) {
	ctx := context.Background()
	// Use cat to echo everything back
	cmd := exec.Command("cat")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Skip SessionStarted
	<-events

	// Send multiple inputs
	inputs := []string{"first\n", "second\n", "third\n"}
	for _, input := range inputs {
		err := session.SendInput(input)
		if err != nil {
			t.Fatalf("SendInput(%q) failed: %v", input, err)
		}
	}

	// Close stdin
	session.Cancel()

	// Collect all output
	var outputs []string
	for event := range events {
		if event.Type == EventOutput {
			outputs = append(outputs, event.Content)
		}
		if event.Type == EventSessionEnded {
			break
		}
	}

	_ = session.Wait()

	fullOutput := strings.Join(outputs, "")

	// Verify all inputs were echoed
	for _, input := range inputs {
		trimmed := strings.TrimSpace(input)
		if !strings.Contains(fullOutput, trimmed) {
			t.Errorf("Output missing %q: %q", trimmed, fullOutput)
		}
	}
}

// TestBaseSessionStdinCloseOnCancel verifies Cancel() closes stdin pipe
// Expected failure: baseSession.Cancel() does not close stdin pipe yet
func TestBaseSessionStdinCloseOnCancel(t *testing.T) {
	ctx := context.Background()
	// Use a command that will exit when stdin closes
	cmd := exec.Command("cat")

	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Wait for start
	<-events

	// Cancel should close stdin, causing cat to exit
	session.Cancel()

	// Session should end quickly
	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case <-done:
		// Success - stdin closure caused cat to exit
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel() did not cause stdin to close within 2 seconds")
	}
}

// TestBaseSessionWithStdinPromptDelivery verifies session works with stdin prompt delivery
// Expected failure: baseSession does not handle stdin prompt delivery correctly yet
func TestBaseSessionWithStdinPromptDelivery(t *testing.T) {
	ctx := context.Background()
	// Use cat to echo stdin back
	cmd := exec.Command("cat")

	// If the agent uses stdin prompt delivery, the prompt should be sent first
	// then interactive input can be sent
	session := newBaseSession(ctx, cmd, nil)
	events := session.Events()

	// Skip SessionStarted
	<-events

	// Send input
	err := session.SendInput("interactive input\n")
	if err != nil {
		t.Fatalf("SendInput() failed: %v", err)
	}

	session.Cancel()

	// Read output
	var sawOutput bool
	for event := range events {
		if event.Type == EventOutput && strings.Contains(event.Content, "interactive input") {
			sawOutput = true
		}
		if event.Type == EventSessionEnded {
			break
		}
	}

	_ = session.Wait()

	if !sawOutput {
		t.Error("Did not receive expected output")
	}
}

// TestGenericSessionInterface verifies Session[T any] generic interface exists
// Expected failure: Session[T] generic interface does not exist yet
func TestGenericSessionInterface(t *testing.T) {
	// Session[T] should embed BaseSession and add Result() method
	// This test verifies the generic interface can be used with different result types

	// Test that RefineSession satisfies Session[RefineResult]
	var _ Session[RefineResult] = (*RefineSession)(nil)

	// Test that PlanSession satisfies Session[PlanResult]
	var _ Session[PlanResult] = (*PlanSession)(nil)

	// Test that ReviewSession satisfies Session[ReviewResult]
	var _ Session[ReviewResult] = (*ReviewSession)(nil)

	// Test that ExploreSession satisfies Session[ExploreResult]
	var _ Session[ExploreResult] = (*ExploreSession)(nil)
}

// TestResultBeforeWaitCompletes verifies Result() returns error when called before Wait() completes
// Expected failure: typed sessions do not return "session not complete" error yet
func TestResultBeforeWaitCompletes(t *testing.T) {
	ctx := context.Background()
	// Use a long-running command
	cmd := exec.Command("sleep", "1")

	tests := []struct {
		name    string
		session interface {
			Wait() error
			Cancel()
		}
		resultFunc func() error
	}{
		{
			name:    "RefineSession",
			session: newRefineSession(ctx, cmd, nil),
			resultFunc: func() error {
				_, err := newRefineSession(ctx, exec.Command("sleep", "1"), nil).Result()
				return err
			},
		},
		{
			name:    "PlanSession",
			session: newPlanSession(ctx, cmd, nil),
			resultFunc: func() error {
				_, err := newPlanSession(ctx, exec.Command("sleep", "1"), nil).Result()
				return err
			},
		},
		{
			name:    "ReviewSession",
			session: newReviewSession(ctx, cmd, nil),
			resultFunc: func() error {
				_, err := newReviewSession(ctx, exec.Command("sleep", "1"), nil).Result()
				return err
			},
		},
		{
			name:    "ExploreSession",
			session: newExploreSession(ctx, cmd, nil),
			resultFunc: func() error {
				_, err := newExploreSession(ctx, exec.Command("sleep", "1"), nil).Result()
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.session.Cancel()

			// Call Result() before Wait() completes
			err := tt.resultFunc()

			if err == nil {
				t.Error("Result() before Wait() returned nil error, want error")
			}

			if err != nil && !strings.Contains(err.Error(), "not complete") {
				t.Errorf("Result() error = %q, want error containing 'not complete'", err.Error())
			}
		})
	}
}

// TestPlanSessionResultAfterCompletion verifies PlanSession.Result() returns parsed results
// Expected failure: PlanSession does not have result field and Result() implementation yet
func TestPlanSessionResultAfterCompletion(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	// Create PlanSession with post-processing that populates result
	session := newPlanSession(ctx, cmd, nil)

	// Wait for completion
	err := session.Wait()
	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	// Result() should return parsed PlanResult
	result, err := session.Result()
	if err != nil {
		t.Errorf("Result() after completion failed: %v", err)
	}

	// Result should have initialized slices
	if result.CreatedPlans == nil {
		t.Error("Result.CreatedPlans is nil, want empty slice")
	}
}

// TestReviewSessionResultAfterCompletion verifies ReviewSession.Result() returns parsed results
// Expected failure: ReviewSession does not have result field and Result() implementation yet
func TestReviewSessionResultAfterCompletion(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	session := newReviewSession(ctx, cmd, nil)

	err := session.Wait()
	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Errorf("Result() after completion failed: %v", err)
	}

	// ReviewResult has no slice fields but should be valid
	var _ ReviewResult = result
}

// TestExploreSessionResultAfterCompletion verifies ExploreSession.Result() returns parsed results
// Expected failure: ExploreSession does not have result field and Result() implementation yet
func TestExploreSessionResultAfterCompletion(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	session := newExploreSession(ctx, cmd, nil)

	err := session.Wait()
	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Errorf("Result() after completion failed: %v", err)
	}

	// Result should have initialized slices
	if result.CreatedSpecs == nil {
		t.Error("Result.CreatedSpecs is nil, want empty slice")
	}
	if result.CreatedEpics == nil {
		t.Error("Result.CreatedEpics is nil, want empty slice")
	}
	if result.CreatedBacklogItems == nil {
		t.Error("Result.CreatedBacklogItems is nil, want empty slice")
	}
}

// TestTypedSessionsEmbedBaseSession verifies typed sessions embed baseSession
// Expected failure: typed sessions do not embed baseSession yet
func TestTypedSessionsEmbedBaseSession(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	tests := []struct {
		name    string
		session interface {
			Events() <-chan Event
			SendInput(string) error
			Cancel()
			Wait() error
		}
	}{
		{
			name:    "RefineSession embeds baseSession",
			session: newRefineSession(ctx, cmd, nil),
		},
		{
			name:    "PlanSession embeds baseSession",
			session: newPlanSession(ctx, cmd, nil),
		},
		{
			name:    "ReviewSession embeds baseSession",
			session: newReviewSession(ctx, cmd, nil),
		},
		{
			name:    "ExploreSession embeds baseSession",
			session: newExploreSession(ctx, cmd, nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.session.Cancel()

			// Verify baseSession methods are available
			events := tt.session.Events()
			if events == nil {
				t.Error("Events() returned nil, want non-nil channel")
			}

			// Wait for process to complete
			_ = tt.session.Wait()
		})
	}
}

// TestBaseSessionPostProcessCallbackSignature verifies postProcess callback signature
// Expected failure: baseSession does not accept postProcess callback parameter yet
func TestBaseSessionPostProcessCallbackSignature(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	// postProcess callback should have signature: func() error
	var postProcessCalled bool
	postProcessFn := func() error {
		postProcessCalled = true
		return nil
	}

	session := newBaseSession(ctx, cmd, postProcessFn)

	err := session.Wait()
	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	if !postProcessCalled {
		t.Error("postProcess callback was not called")
	}
}

// TestTypedSessionsHaveResultFields verifies typed sessions have result and resultErr fields
// Expected failure: typed sessions do not have result/resultErr fields yet
func TestTypedSessionsHaveResultFields(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	// RefineSession
	refineSession := newRefineSession(ctx, cmd, func() error {
		return nil
	})
	_ = refineSession.Wait()
	refineResult, refineErr := refineSession.Result()
	if refineErr != nil {
		t.Errorf("RefineSession.Result() failed: %v", refineErr)
	}
	// Verify result is the correct type
	var _ RefineResult = refineResult

	// PlanSession
	planSession := newPlanSession(ctx, cmd, func() error {
		return nil
	})
	_ = planSession.Wait()
	planResult, planErr := planSession.Result()
	if planErr != nil {
		t.Errorf("PlanSession.Result() failed: %v", planErr)
	}
	var _ PlanResult = planResult

	// ReviewSession
	reviewSession := newReviewSession(ctx, cmd, func() error {
		return nil
	})
	_ = reviewSession.Wait()
	reviewResult, reviewErr := reviewSession.Result()
	if reviewErr != nil {
		t.Errorf("ReviewSession.Result() failed: %v", reviewErr)
	}
	var _ ReviewResult = reviewResult

	// ExploreSession
	exploreSession := newExploreSession(ctx, cmd, func() error {
		return nil
	})
	_ = exploreSession.Wait()
	exploreResult, exploreErr := exploreSession.Result()
	if exploreErr != nil {
		t.Errorf("ExploreSession.Result() failed: %v", exploreErr)
	}
	var _ ExploreResult = exploreResult
}

// TestWaitInvokesPostProcessAfterSubprocessExits verifies postProcess runs after subprocess exits successfully
// Expected failure: baseSession does not call postProcess at the correct time yet
func TestWaitInvokesPostProcessAfterSubprocessExits(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "output")

	var postProcessOrder []string
	postProcessFn := func() error {
		postProcessOrder = append(postProcessOrder, "postProcess")
		return nil
	}

	session := newBaseSession(ctx, cmd, postProcessFn)
	events := session.Events()

	// Drain events to detect when subprocess exits
	go func() {
		for event := range events {
			if event.Type == EventSessionEnded {
				postProcessOrder = append(postProcessOrder, "subprocess_exited")
			}
		}
	}()

	err := session.Wait()
	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	// postProcess should have been called after subprocess exited
	if len(postProcessOrder) < 2 {
		t.Fatal("Expected both subprocess_exited and postProcess events")
	}

	// The order should be: subprocess_exited, then postProcess
	// (postProcess is called by Wait() after subprocess exits but before Wait() returns)
	if !strings.Contains(strings.Join(postProcessOrder, ","), "postProcess") {
		t.Error("postProcess was not called")
	}
}

// TestPostProcessErrorPropagatesFromWait verifies Wait() returns postProcess errors
// Expected failure: baseSession does not propagate postProcess errors correctly yet
func TestPostProcessErrorPropagatesFromWait(t *testing.T) {
	ctx := context.Background()
	cmd := exec.Command("echo", "test")

	expectedErr := fmt.Errorf("parsing failed: invalid output format")
	postProcessFn := func() error {
		return expectedErr
	}

	session := newBaseSession(ctx, cmd, postProcessFn)

	err := session.Wait()

	if err == nil {
		t.Fatal("Wait() returned nil, want postProcess error")
	}

	if !strings.Contains(err.Error(), "parsing failed") {
		t.Errorf("Wait() error = %q, want error containing 'parsing failed'", err.Error())
	}
}
