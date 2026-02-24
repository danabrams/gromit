package escalation

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestHandlerLogCallbackProducesOutput verifies that the Handler's log method
// actually invokes the LogFn callback with formatted messages during
// escalation operations.
//
// Expected failure: Handler has a logFn field and log() method, but
// HandleStallTimeout does not call h.log() — it just updates fields silently.
// After the local methods are removed from process.go and logging moves
// into the Handler, HandleStallTimeout (and other methods) should log their
// actions through the LogFn callback (e.g. "Stall timeout detected, escalating
// from low to medium").
func TestHandlerLogCallbackProducesOutput(t *testing.T) {
	cfg := newTestConfig()
	var logMessages []string
	logFn := func(format string, args ...interface{}) {
		logMessages = append(logMessages, format)
	}
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, logFn, nil)

	bc := newTestBeadContext()
	bc.Tier = "low"
	bc.Model = "haiku"
	bc.RetriesThisModel = 3
	bc.MaxRetries = 2

	// HandleStallTimeout should log something about the stall and escalation
	h.HandleStallTimeout(context.Background(), bc)

	if len(logMessages) == 0 {
		t.Error("Handler.HandleStallTimeout should produce log output via LogFn — " +
			"after local methods are removed from process.go, the Handler must log " +
			"retry/escalation events through its LogFn callback")
	}
}

// TestHandlerLogCallbackOnAnalyzeFailure verifies that AnalyzeAndHandleFailure
// produces log messages through the LogFn callback when it analyzes a failure
// and decides to retry or escalate.
//
// Expected failure: AnalyzeAndHandleFailure does not currently call h.log()
// to report analysis results or retry decisions. The Runner's local
// analyzeAndHandleFailure method (process.go:321-386) produces log output like
// "Build failed, running failure analysis..." and "Analysis: category=X,
// recoverable=Y". After the local method is removed, the Handler must produce
// equivalent log output through LogFn.
func TestHandlerLogCallbackOnAnalyzeFailure(t *testing.T) {
	cfg := newTestConfig()
	var logMessages []string
	logFn := func(format string, args ...interface{}) {
		logMessages = append(logMessages, format)
	}

	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    "logic_error",
				Recoverable: true,
				RootCause:   "missing import",
			}, nil
		},
	}
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, logFn, nil)

	bc := newTestBeadContext()
	bc.Tier = "low"

	claudeResult := &provider.Result{Success: false, Output: "build failed: missing import"}
	h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)

	if len(logMessages) == 0 {
		t.Error("Handler.AnalyzeAndHandleFailure should produce log output via LogFn — " +
			"should log analysis category, root cause, and retry/escalation decisions")
	}
}

// TestHandlerLogCallbackOnEscalation verifies that HandleEscalation produces
// log output through LogFn when escalating to a new tier.
//
// Expected failure: HandleEscalation does not currently call h.log().
// The Runner's local handleEscalation method (process.go:390-420) logs
// "Escalating from tier X to Y". After the local method is removed, the
// Handler must produce equivalent log output through LogFn.
func TestHandlerLogCallbackOnEscalation(t *testing.T) {
	cfg := newTestConfig()
	var logMessages []string
	logFn := func(format string, args ...interface{}) {
		logMessages = append(logMessages, format)
	}
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, logFn, nil)

	bc := newTestBeadContext()
	bc.Tier = "low"
	bc.Model = "haiku"

	claudeResult := &provider.Result{Success: false, Output: "build failed"}
	h.HandleEscalation(context.Background(), bc, claudeResult)

	if len(logMessages) == 0 {
		t.Error("Handler.HandleEscalation should produce log output via LogFn — " +
			"should log 'Escalating from tier X to Y' when a higher tier is available")
	}
}

// TestHandlerLogCallbackOnDecomposition verifies that AttemptDecomposition
// produces log output through LogFn when decomposing a task.
//
// Expected failure: AttemptDecomposition does not currently call h.log().
// The Runner's local attemptDecomposition method (process.go:425-448) logs
// "Task successfully decomposed into N sub-tasks". After the local method
// is removed, the Handler must produce equivalent log output through LogFn.
func TestHandlerLogCallbackOnDecomposition(t *testing.T) {
	cfg := newTestConfig()
	var logMessages []string
	logFn := func(format string, args ...interface{}) {
		logMessages = append(logMessages, format)
	}
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			return []runtypes.SubTask{{Title: "sub-1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
			return nil
		},
		logFn,
		nil,
	)

	bc := newTestBeadContext()
	h.AttemptDecomposition(context.Background(), bc, "test failure")

	if len(logMessages) == 0 {
		t.Error("Handler.AttemptDecomposition should produce log output via LogFn — " +
			"should log decomposition attempts and results")
	}
}
