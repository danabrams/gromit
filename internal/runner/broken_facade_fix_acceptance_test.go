package runner

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestAnalyzeAndHandleFailure_SetsPromptContextOnRecoverableRetry verifies that
// when AnalyzeAndHandleFailure decides to retry a recoverable failure, it sets
// bc.PromptCtx.IsRetry=true and bc.PromptCtx.FailureContext to the analysis
// suggestion, so the retry prompt includes failure context for the next attempt.
//
// Expected failure: the escalation.Handler.AnalyzeAndHandleFailure method
// currently increments retry counters (RetriesThisModel, TotalRetriesThisBead)
// but does NOT set bc.PromptCtx.IsRetry or bc.PromptCtx.FailureContext. The
// old runner-local method set these fields; the escalation extraction lost
// that behavior. The fix must add these assignments to the recoverable retry
// path in handler.go.
func TestAnalyzeAndHandleFailure_SetsPromptContextOnRecoverableRetry(t *testing.T) {
	mockAnalyzerObj := &mockFailureAnalyzer{
		AnalyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategorySyntax,
				Recoverable: true,
				RootCause:   "missing import",
				Suggestion:  "Add the missing import statement",
			}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{AnalysisTimeout: 30}},
		&buf, t.TempDir(),
		Deps{
			Beads:    &mockBeadClient{},
			Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
			Analyzer: mockAnalyzerObj,
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})

	bc := &runtypes.BeadContext{
		Bead:              &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:            &IterationResult{},
		Model:             "sonnet",
		PromptCtx:         &prompt.Context{Model: "sonnet", ConfirmedLearnings: []learnings.Learning{}, RecentLearnings: []learnings.Learning{}},
		RetriesThisModel:  0,
		MaxRetries:        2,
		MaxRetriesPerBead: 5,
	}

	claudeResult := &claude.Result{Success: false, Output: "compile error: missing import"}
	continueLoop := r.escalationHandler.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)

	if !continueLoop {
		t.Fatal("expected continueLoop=true for recoverable failure")
	}
	if bc.RetriesThisModel != 1 {
		t.Errorf("expected RetriesThisModel=1, got %d", bc.RetriesThisModel)
	}

	// Expected failure: handler does not set IsRetry on PromptCtx.
	// The fix must add: bc.PromptCtx.IsRetry = true in the recoverable retry path.
	if !bc.PromptCtx.IsRetry {
		t.Error("expected IsRetry=true after recoverable failure; " +
			"handler must set bc.PromptCtx.IsRetry so the retry prompt indicates this is a retry")
	}

	// Expected failure: handler does not set FailureContext on PromptCtx.
	// The fix must add: bc.PromptCtx.FailureContext = analysis.Suggestion
	if bc.PromptCtx.FailureContext != "Add the missing import statement" {
		t.Errorf("expected FailureContext=%q, got %q; "+
			"handler must set bc.PromptCtx.FailureContext to analysis.Suggestion",
			"Add the missing import statement", bc.PromptCtx.FailureContext)
	}
}

// TestProcessBead_ValidationUsesInjectedCmdRunnerViaDeps verifies that when
// NewRunnerWithDeps is called with a CmdRunner in Deps, the validation.Runner
// uses that injected CmdRunner for direct validation commands, not the
// hardcoded defaultCmdRunner.
//
// Expected failure: Deps struct does not have a CmdRunner field. The
// NewRunnerWithDeps constructor passes defaultCmdRunner directly to
// validation.NewRunner (runner.go line 343). The fix must:
// 1. Add CmdRunner field to Deps struct
// 2. Wire Deps.CmdRunner into validation.NewRunner in NewRunnerWithDeps
// 3. Fall back to defaultCmdRunner when Deps.CmdRunner is nil
func TestProcessBead_ValidationUsesInjectedCmdRunnerViaDeps(t *testing.T) {
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build ok"}, nil
		},
	}

	cmdRunCount := 0
	injectedCmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		cmdRunCount++
		return "ok", "", 0, nil
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		&config.Config{
			Claude:     config.ClaudeConfig{BeadTimeout: 60},
			Validation: config.ValidationConfig{Enabled: true, Commands: []string{"go test ./..."}},
			Models:     config.ModelsConfig{Validation: "haiku"},
		},
		&buf, t.TempDir(),
		Deps{
			Beads:    &mockBeadClient{},
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
			// Expected failure: CmdRunner field does not exist on Deps struct.
			// The fix adds this field so tests can inject mock command runners
			// that flow through to validation.Runner at construction time.
			CmdRunner: injectedCmdRunner,
		})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	b := &bead.Bead{ID: "val-inject-test", Title: "Validation Injection Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if !result.Validated {
		t.Error("expected Validated=true when validation commands pass")
	}
	// The injected CmdRunner must be used by validation.Runner.RunDirect,
	// not the hardcoded defaultCmdRunner.
	if cmdRunCount != 1 {
		t.Errorf("expected injected CmdRunner called 1 time for validation, got %d", cmdRunCount)
	}
}

// TestProcessBead_RetryPromptIncludesFailureContext verifies that after a
// recoverable build failure, the retry prompt includes the failure context
// from analysis (IsRetry=true, FailureContext set), so the next invocation
// has the analysis suggestion available.
//
// This tests the end-to-end behavior through processBead: the escalation
// handler's AnalyzeAndHandleFailure sets IsRetry/FailureContext on the
// PromptCtx, and the retry invocation sees those values.
//
// Expected failure: the escalation handler does not set IsRetry or
// FailureContext, so when the retry invocation reads PromptCtx, these fields
// are still at their zero values (false and "").
func TestProcessBead_RetryPromptIncludesFailureContext(t *testing.T) {
	callCount := 0
	var capturedIsRetry bool
	var capturedFailureContext string

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			callCount++
			if callCount == 1 {
				return &claude.Result{Success: false, Output: "compile error: missing import", ExitCode: 1}, nil
			}
			// On retry, capture the state of the prompt context by checking
			// if the prompt mentions failure context (the renderer would include it)
			return &claude.Result{Success: true, Output: "fixed"}, nil
		},
	}

	mockAnalyzerObj := &mockFailureAnalyzer{
		AnalyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategorySyntax,
				Recoverable: true,
				RootCause:   "missing import",
				Suggestion:  "Add the missing import statement",
			}, nil
		},
	}

	// Custom renderer that captures PromptCtx state on retry builds
	mockRend := &mockPromptRenderer{
		RenderBuildFn: func(ctx *prompt.Context) (string, error) {
			if ctx.IsRetry {
				capturedIsRetry = true
				capturedFailureContext = ctx.FailureContext
			}
			return "mock build prompt", nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude:     config.ClaudeConfig{BeadTimeout: 60, AnalysisTimeout: 30},
			Escalation: config.EscalationConfig{Enabled: true, Chain: []string{"sonnet", "opus"}, MaxRetriesPerModel: 1, MaxRetriesPerBead: 5},
			Models:     config.ModelsConfig{P1: "sonnet"},
		},
		&buf, t.TempDir(),
		Deps{
			Beads:    &mockBeadClient{},
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: mockAnalyzerObj,
			Renderer: mockRend,
			Logger:   &mockIterationLogger{},
		})

	b := &bead.Bead{ID: "retry-ctx-test", Title: "Retry Context Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if !result.Success {
		t.Errorf("expected success after retry, got error: %v", result.Error)
	}
	if callCount < 2 {
		t.Fatalf("expected at least 2 invocations (fail + retry), got %d", callCount)
	}

	// Expected failure: IsRetry is not set by the escalation handler on
	// recoverable retry, so RenderBuild never sees IsRetry=true.
	if !capturedIsRetry {
		t.Error("expected IsRetry=true in PromptCtx during retry RenderBuild; " +
			"the escalation handler must set bc.PromptCtx.IsRetry=true on recoverable retry")
	}
	if capturedFailureContext != "Add the missing import statement" {
		t.Errorf("expected FailureContext=%q in retry RenderBuild, got %q; "+
			"the escalation handler must set bc.PromptCtx.FailureContext to analysis.Suggestion",
			"Add the missing import statement", capturedFailureContext)
	}
}
