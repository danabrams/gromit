package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

func readStreamLog(t *testing.T, sl *logger.StreamLogger) string {
	t.Helper()
	data, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}
	return string(data)
}

func assertPromptDiagnosticsReconciled(t *testing.T, diag *prompt.PromptDiagnostics, reportedTokens, tokenDelta int) {
	t.Helper()
	if diag == nil {
		t.Fatal("expected PromptDiagnostics to be set")
	}
	if diag.ReportedTokens != reportedTokens {
		t.Fatalf("ReportedTokens = %d, want %d", diag.ReportedTokens, reportedTokens)
	}
	if diag.TokenDelta != tokenDelta {
		t.Fatalf("TokenDelta = %d, want %d", diag.TokenDelta, tokenDelta)
	}
}

func TestSummarizeATDDProviderOutput(t *testing.T) {
	if got := summarizeATDDProviderOutput("   "); got != "no provider output" {
		t.Fatalf("expected empty output sentinel, got %q", got)
	}

	if got := summarizeATDDProviderOutput("  failure details  "); got != "failure details" {
		t.Fatalf("expected trimmed output, got %q", got)
	}

	long := strings.Repeat("x", 1700)
	got := summarizeATDDProviderOutput(long)
	if !strings.Contains(got, "...[truncated]...") {
		t.Fatalf("expected truncated marker, got %q", got)
	}
	if len(got) >= len(long) {
		t.Fatalf("expected summarized output shorter than input, got len=%d input=%d", len(got), len(long))
	}
}

func TestIsATDDFallbackEligible(t *testing.T) {
	tests := []struct {
		name         string
		failureClass string
		want         bool
	}{
		{name: "transport disconnect", failureClass: provider.FailureCategoryTransportDisconnect, want: true},
		{name: "startup error", failureClass: provider.FailureCategoryStartupError, want: true},
		{name: "other", failureClass: provider.FailureCategoryOther, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isATDDFallbackEligible(tc.failureClass); got != tc.want {
				t.Fatalf("isATDDFallbackEligible(%q) = %t, want %t", tc.failureClass, got, tc.want)
			}
		})
	}
}

func TestMakeMethodologyExec_ATDDCapturesStreamCostData(t *testing.T) {
	mockProvider := &mockProviderWithRouterTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			if handler != nil {
				handler([]byte(`{"type":"result","subtype":"done","total_cost_usd":1.23,"input_tokens":10,"output_tokens":20}`))
			}
			return &provider.Result{Success: true}, nil
		},
	}

	streamLogger, err := logger.NewStreamLogger(t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamLogger: %v", err)
	}
	defer func() {
		_ = streamLogger.Close()
	}()

	r := &Runner{
		router:       provider.NewSingleProviderRouter(mockProvider),
		output:       io.Discard,
		streamLogger: streamLogger,
		renderer: &mockPromptRenderer{
			RenderAcceptanceTestsFn: func(ctx *prompt.Context) (string, error) {
				return "acceptance prompt", nil
			},
			LastDiagnosticsFn: func() *prompt.PromptDiagnostics {
				return &prompt.PromptDiagnostics{
					PromptType:      "acceptance_tests",
					EstimatedTokens: 8,
					SectionTokens: map[string]int{
						prompt.SectionRules: 8,
					},
				}
			},
		},
	}
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "b1"},
		Result:    &runtypes.IterationResult{},
		Tier:      provider.TierLow,
		PromptCtx: &prompt.Context{},
	}

	methExec := r.makeMethodologyExec()
	if methExec == nil {
		t.Fatal("expected makeMethodologyExec to return executor")
	}

	if err := methExec.RunAcceptanceTests(context.Background(), bc); err != nil {
		t.Fatalf("RunAcceptanceTests returned error: %v", err)
	}

	if bc.Result.InputTokens != 10 {
		t.Fatalf("InputTokens = %d, want 10", bc.Result.InputTokens)
	}
	if bc.Result.OutputTokens != 20 {
		t.Fatalf("OutputTokens = %d, want 20", bc.Result.OutputTokens)
	}
	if bc.Result.CostUSD != 1.23 {
		t.Fatalf("CostUSD = %.2f, want 1.23", bc.Result.CostUSD)
	}
	assertPromptDiagnosticsReconciled(t, bc.Result.PromptDiagnostics, 10, -2)
}

func TestMakeInvokeFn_ReconcilesPromptDiagnosticsAfterRetryRender(t *testing.T) {
	mockProvider := &mockProviderWithRouterTracking{
		streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success:      true,
				Model:        "model-a",
				InputTokens:  10,
				OutputTokens: 1,
			}, nil
		},
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)
	diag := &prompt.PromptDiagnostics{
		PromptType:      "build",
		EstimatedTokens: 12,
		SectionTokens: map[string]int{
			prompt.SectionTemplateStatic: 12,
		},
	}
	r := &Runner{
		cfg:     &config.Config{},
		router:  mockRouter,
		invoker: newInvokerForTest(mockRouter, io.Discard, nil),
		output:  io.Discard,
		renderer: &mockPromptRenderer{
			RenderBuildFn: func(ctx *prompt.Context) (string, error) {
				return "retry prompt", nil
			},
			LastDiagnosticsFn: func() *prompt.PromptDiagnostics {
				return diag
			},
		},
	}
	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "b2", Title: "Retry Prompt"},
		Tier:        provider.TierLow,
		Result:      &runtypes.IterationResult{},
		BuildPrompt: "old prompt",
		PromptCtx: &prompt.Context{
			IsRetry: true,
		},
		ParentCtx: context.Background(),
	}

	invResult, err := r.makeInvokeFn()(context.Background(), bc, "ignored")
	if err != nil {
		t.Fatalf("makeInvokeFn returned error: %v", err)
	}
	if invResult == nil || invResult.Result == nil {
		t.Fatal("expected invocation result")
	}
	assertPromptDiagnosticsReconciled(t, bc.Result.PromptDiagnostics, 10, 2)
}

func TestMakeInvokeFn_RecordsOutcomeWithProviderAndFailureCategory(t *testing.T) {
	cb := &provider.CircuitBreaker{}
	mockProvider := &mockProviderWithRouterTracking{
		name: "test-provider",
		streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success:         false,
				Model:           "model-a",
				FailureCategory: provider.FailureCategoryTransportDisconnect,
			}, nil
		},
	}
	mockRouter := provider.NewRouter(
		map[string]provider.Provider{"test-provider": mockProvider},
		map[string]string{"build": "test-provider"},
		map[string]int{"test-provider": 100},
		0,
		nil,
		cb,
	)
	r := &Runner{
		cfg:     &config.Config{},
		router:  mockRouter,
		invoker: newInvokerForTest(mockRouter, io.Discard, nil),
		output:  io.Discard,
	}
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "b-record"},
		Tier:      provider.TierLow,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
		ParentCtx: context.Background(),
	}

	invResult, err := r.makeInvokeFn()(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("makeInvokeFn returned error: %v", err)
	}
	if invResult == nil || invResult.ProviderResult == nil {
		t.Fatal("expected provider result from invocation")
	}
	if !cb.IsDegraded("test-provider") {
		t.Fatal("expected circuit breaker to degrade after transport_disconnect outcome")
	}
}

func TestMakeInvokeFn_EmitsInvocationMarkersOnSuccess(t *testing.T) {
	streamLogger, err := logger.NewStreamLogger(t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamLogger: %v", err)
	}
	defer func() {
		_ = streamLogger.Close()
	}()

	mockProvider := &mockProviderWithRouterTracking{
		name: "marker-provider",
		streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "marker-model"}, nil
		},
	}
	router := provider.NewSingleProviderRouter(mockProvider)
	r := &Runner{
		cfg:          &config.Config{},
		router:       router,
		invoker:      newInvokerForTest(router, io.Discard, streamLogger),
		output:       io.Discard,
		streamLogger: streamLogger,
	}
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "b-markers-success"},
		Tier:      provider.TierLow,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
		ParentCtx: context.Background(),
	}

	_, err = r.makeInvokeFn()(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("makeInvokeFn returned error: %v", err)
	}

	logText := readStreamLog(t, streamLogger)
	if !strings.Contains(logText, "INVOCATION_START provider=") {
		t.Fatalf("expected INVOCATION_START marker, got:\n%s", logText)
	}
	if !strings.Contains(logText, "INVOCATION_END provider=marker-provider model=test-haiku tier=low success=true duration=") {
		t.Fatalf("expected INVOCATION_END marker with provider/model/tier/success, got:\n%s", logText)
	}
	if !strings.Contains(logText, "failure_category=") {
		t.Fatalf("expected INVOCATION_END marker with failure_category field, got:\n%s", logText)
	}
}

func TestMakeInvokeFn_EmitsInvocationMarkersOnFailureWithoutProviderEvents(t *testing.T) {
	streamLogger, err := logger.NewStreamLogger(t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamLogger: %v", err)
	}
	defer func() {
		_ = streamLogger.Close()
	}()

	mockProvider := &mockProviderWithRouterTracking{
		name: "marker-provider",
		streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return nil, errors.New("provider boom")
		},
	}
	router := provider.NewSingleProviderRouter(mockProvider)
	r := &Runner{
		cfg:          &config.Config{},
		router:       router,
		invoker:      newInvokerForTest(router, io.Discard, streamLogger),
		output:       io.Discard,
		streamLogger: streamLogger,
	}
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "b-markers-failure"},
		Tier:      provider.TierLow,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
		ParentCtx: context.Background(),
	}

	_, err = r.makeInvokeFn()(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected invocation error")
	}

	logText := readStreamLog(t, streamLogger)
	if !strings.Contains(logText, "INVOCATION_START provider=") {
		t.Fatalf("expected INVOCATION_START marker, got:\n%s", logText)
	}
	if !strings.Contains(logText, "INVOCATION_END provider=marker-provider model=test-haiku tier=low success=false duration=") {
		t.Fatalf("expected INVOCATION_END failure marker, got:\n%s", logText)
	}
	if !strings.Contains(logText, "failure_category=") {
		t.Fatalf("expected INVOCATION_END marker with failure_category field, got:\n%s", logText)
	}
}

func TestMakeMethodologyExec_ATDDRecordsSuccessOutcomeWithEmptyFailureCategory(t *testing.T) {
	cb := &provider.CircuitBreaker{}
	invocations := 0
	mockProvider := &mockProviderWithRouterTracking{
		name: "test-provider",
		streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			invocations++
			if invocations == 1 {
				return &provider.Result{
					Success:         false,
					FailureCategory: provider.FailureCategoryTransportDisconnect,
					Output:          "transport failed",
				}, nil
			}
			return &provider.Result{
				Success: true,
				Output:  "ok",
			}, nil
		},
	}
	mockRouter := provider.NewRouter(
		map[string]provider.Provider{"test-provider": mockProvider},
		map[string]string{"build": "test-provider"},
		map[string]int{"test-provider": 100},
		0,
		nil,
		cb,
	)
	r := &Runner{
		router: mockRouter,
		output: io.Discard,
		renderer: &mockPromptRenderer{
			RenderAcceptanceTestsFn: func(ctx *prompt.Context) (string, error) {
				return "acceptance prompt", nil
			},
		},
	}
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "b-atdd-record"},
		Tier:      provider.TierLow,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
	}

	methExec := r.makeMethodologyExec()
	if methExec == nil {
		t.Fatal("expected makeMethodologyExec to return executor")
	}

	if err := methExec.RunAcceptanceTests(context.Background(), bc); err == nil {
		t.Fatal("expected first acceptance run to fail")
	}
	if !cb.IsDegraded("test-provider") {
		t.Fatal("expected circuit breaker to degrade after ATDD transport_disconnect")
	}

	for i := 0; i < 5; i++ {
		if err := methExec.RunAcceptanceTests(context.Background(), bc); err != nil {
			t.Fatalf("expected acceptance run %d to succeed: %v", i+2, err)
		}
	}
	if cb.IsDegraded("test-provider") {
		t.Fatal("expected five successful ATDD outcomes with empty failure category to recover degraded provider")
	}
}

func TestMakeMethodologyExec_EmitsATDDInvocationMarkersOnSuccess(t *testing.T) {
	streamLogger, err := logger.NewStreamLogger(t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamLogger: %v", err)
	}
	defer func() {
		_ = streamLogger.Close()
	}()

	mockProvider := &mockProviderWithRouterTracking{
		name: "test-provider",
		streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{Success: true}, nil
		},
	}
	r := &Runner{
		router:       provider.NewSingleProviderRouter(mockProvider),
		output:       io.Discard,
		streamLogger: streamLogger,
		renderer: &mockPromptRenderer{
			RenderAcceptanceTestsFn: func(ctx *prompt.Context) (string, error) {
				return "acceptance prompt", nil
			},
		},
	}
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "b-atdd-markers-success"},
		Tier:      provider.TierLow,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
	}

	if err := r.makeMethodologyExec().RunAcceptanceTests(context.Background(), bc); err != nil {
		t.Fatalf("RunAcceptanceTests returned error: %v", err)
	}

	logText := readStreamLog(t, streamLogger)
	if !strings.Contains(logText, "ATDD_INVOCATION_START provider=test-provider model=test-haiku tier=low") {
		t.Fatalf("expected ATDD_INVOCATION_START marker, got:\n%s", logText)
	}
	if !strings.Contains(logText, "ATDD_INVOCATION_END provider=test-provider model=test-haiku tier=low success=true duration=") {
		t.Fatalf("expected ATDD_INVOCATION_END success marker, got:\n%s", logText)
	}
	if !strings.Contains(logText, "failure_category=") {
		t.Fatalf("expected ATDD_INVOCATION_END marker with failure_category field, got:\n%s", logText)
	}
}

func TestMakeMethodologyExec_EmitsATDDInvocationMarkersOnFailureWithoutProviderEvents(t *testing.T) {
	streamLogger, err := logger.NewStreamLogger(t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamLogger: %v", err)
	}
	defer func() {
		_ = streamLogger.Close()
	}()

	mockProvider := &mockProviderWithRouterTracking{
		name: "test-provider",
		streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success:         false,
				FailureCategory: provider.FailureCategoryTransportDisconnect,
			}, nil
		},
	}
	r := &Runner{
		router:       provider.NewSingleProviderRouter(mockProvider),
		output:       io.Discard,
		streamLogger: streamLogger,
		renderer: &mockPromptRenderer{
			RenderAcceptanceTestsFn: func(ctx *prompt.Context) (string, error) {
				return "acceptance prompt", nil
			},
		},
	}
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "b-atdd-markers-failure"},
		Tier:      provider.TierLow,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
	}

	if err := r.makeMethodologyExec().RunAcceptanceTests(context.Background(), bc); err == nil {
		t.Fatal("expected RunAcceptanceTests to fail")
	}

	logText := readStreamLog(t, streamLogger)
	if !strings.Contains(logText, "ATDD_INVOCATION_START provider=test-provider model=test-haiku tier=low") {
		t.Fatalf("expected ATDD_INVOCATION_START marker, got:\n%s", logText)
	}
	if !strings.Contains(logText, "ATDD_INVOCATION_END provider=test-provider model=test-haiku tier=low success=false duration=") {
		t.Fatalf("expected ATDD_INVOCATION_END failure marker, got:\n%s", logText)
	}
	if !strings.Contains(logText, "failure_category=transport_disconnect") {
		t.Fatalf("expected ATDD_INVOCATION_END failure_category=transport_disconnect, got:\n%s", logText)
	}
}

func TestMakeMethodologyExec_ATDDFallbacksOnStartupErrorAndLogsStructuredDecision(t *testing.T) {
	streamLogger, err := logger.NewStreamLogger(t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamLogger: %v", err)
	}
	defer func() {
		_ = streamLogger.Close()
	}()

	primaryCalls := 0
	primary := &mockProviderWithRouterTracking{
		name: "primary-provider",
		streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			primaryCalls++
			return nil, errors.New("failed to start codex command")
		},
	}
	fallbackCalls := 0
	fallback := &mockProviderWithRouterTracking{
		name: "fallback-provider",
		streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			fallbackCalls++
			return &provider.Result{Success: true}, nil
		},
	}
	router := provider.NewRouter(
		map[string]provider.Provider{
			primary.Name():  primary,
			fallback.Name(): fallback,
		},
		map[string]string{"build": primary.Name()},
		map[string]int{primary.Name(): 100, fallback.Name(): 100},
		time.Minute,
		nil,
		&provider.CircuitBreaker{},
	)
	var output bytes.Buffer
	r := &Runner{
		router:       router,
		output:       &output,
		streamLogger: streamLogger,
		renderer: &mockPromptRenderer{
			RenderAcceptanceTestsFn: func(ctx *prompt.Context) (string, error) {
				return "acceptance prompt", nil
			},
		},
	}
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "b-atdd-startup-fallback"},
		Tier:      provider.TierLow,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
	}

	if err := r.makeMethodologyExec().RunAcceptanceTests(context.Background(), bc); err != nil {
		t.Fatalf("RunAcceptanceTests returned error: %v", err)
	}
	if primaryCalls != 1 {
		t.Fatalf("primary calls = %d, want 1", primaryCalls)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallbackCalls)
	}
	if got := bc.Result.FallbackAttempts; got != 1 {
		t.Fatalf("FallbackAttempts = %d, want 1", got)
	}
	if got := bc.Result.FallbackSuccesses; got != 1 {
		t.Fatalf("FallbackSuccesses = %d, want 1", got)
	}
	if got := bc.Result.FallbackFailures; got != 0 {
		t.Fatalf("FallbackFailures = %d, want 0", got)
	}

	logText := output.String()
	if !strings.Contains(logText, "ATDD_FALLBACK_DECISION class=startup_error primary_provider=primary-provider primary_model=test-haiku fallback_provider=auto fallback_model=auto reason=primary_error") {
		t.Fatalf("expected structured fallback decision log, got:\n%s", logText)
	}
	if !strings.Contains(logText, "ATDD_FALLBACK_ATTEMPT class=startup_error primary_provider=primary-provider primary_model=test-haiku fallback_provider=fallback-provider fallback_model=test-haiku") {
		t.Fatalf("expected structured fallback attempt log, got:\n%s", logText)
	}
}

func TestMakeMethodologyExec_ATDDStartupFailureMarksProviderUnavailable(t *testing.T) {
	cb := &provider.CircuitBreaker{}
	primary := &mockProviderWithRouterTracking{
		name: "primary-provider",
		streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return nil, errors.New("failed to start codex command")
		},
	}
	router := provider.NewRouter(
		map[string]provider.Provider{primary.Name(): primary},
		map[string]string{"build": primary.Name()},
		map[string]int{primary.Name(): 100},
		time.Minute,
		nil,
		cb,
	)
	r := &Runner{
		router: router,
		output: io.Discard,
		renderer: &mockPromptRenderer{
			RenderAcceptanceTestsFn: func(ctx *prompt.Context) (string, error) {
				return "acceptance prompt", nil
			},
		},
	}
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "b-atdd-startup-unavailable"},
		Tier:      provider.TierLow,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
	}

	if err := r.makeMethodologyExec().RunAcceptanceTests(context.Background(), bc); err == nil {
		t.Fatal("expected RunAcceptanceTests to fail")
	}

	selected, _ := router.Select(atddBuildPhase, provider.TierLow)
	if selected != nil {
		t.Fatal("expected provider to be unavailable during cooldown after startup failure")
	}
}

func TestMakeMethodologyExec_WiresCoverageValidationCallbackAtLowTier(t *testing.T) {
	var gotTier string
	mockProvider := &mockProviderWithRouterTracking{
		runFn: func(ctx context.Context, promptText, tier string) (*provider.Result, error) {
			gotTier = tier
			return &provider.Result{
				Success: true,
				Output:  `{"covers": true, "reason": "criterion is covered by direct assertions."}`,
			}, nil
		},
	}
	r := &Runner{
		cfg:    &config.Config{},
		router: provider.NewSingleProviderRouter(mockProvider),
		renderer: &mockPromptRenderer{
			RenderCoverageValidFn: func(ctx *prompt.CoverageValidationContext) (string, error) {
				return "coverage-validation-prompt", nil
			},
		},
	}

	methExec := r.makeMethodologyExec()
	if methExec == nil {
		t.Fatal("expected makeMethodologyExec to return executor")
	}

	resp, err := methExec.ValidateCoverage(
		context.Background(),
		"func TestFeature(t *testing.T) {}",
		coverage.Criterion{Number: 1, Text: "feature works"},
	)
	if err != nil {
		t.Fatalf("ValidateCoverage returned error: %v", err)
	}
	if resp == nil || !resp.Covers {
		t.Fatalf("expected coverage response with Covers=true, got %#v", resp)
	}
	if gotTier != provider.TierLow {
		t.Fatalf("coverage validation tier = %q, want %q", gotTier, provider.TierLow)
	}
}

func TestMakeMethodologyExec_WiresDiagnosticDepsWithRendererAndLowTierRouterInvoke(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./internal/runner/..."},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	workDir := t.TempDir()
	var validateCommand string
	var validateWorkDir string
	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		validateCommand = command
		validateWorkDir = workDir
		return "ok", "", 0, nil
	}

	var gotDiagnosticCtx *prompt.DiagnosticContext
	const renderedDiagnosticPrompt = "diagnostic prompt text"
	var gotInvokeTier string
	var gotInvokePrompt string
	mockProvider := &mockProviderWithRouterTracking{
		runFn: func(ctx context.Context, promptText, tier string) (*provider.Result, error) {
			gotInvokeTier = tier
			gotInvokePrompt = promptText
			return &provider.Result{
				Success: true,
				Output:  "VERDICT: ALREADY_DONE",
			}, nil
		},
	}

	r := &Runner{
		cfg:              cfg,
		router:           provider.NewSingleProviderRouter(mockProvider),
		output:           io.Discard,
		validationRunner: validation.NewRunner(cfg, cmdRunner, nil, nil),
		gitDiffFn: func(fromCommit string) (string, error) {
			return "diff --git a/internal/runner/runner_test.go b/internal/runner/runner_test.go\n+new test", nil
		},
		renderer: &mockPromptRenderer{
			RenderATDDDiagnosticFn: func(ctx *prompt.DiagnosticContext) (string, error) {
				gotDiagnosticCtx = ctx
				return renderedDiagnosticPrompt, nil
			},
		},
	}

	methExec := r.makeMethodologyExec()
	if methExec == nil {
		t.Fatal("expected makeMethodologyExec to return executor")
	}

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:                 "b-diag-wiring",
			Title:              "Diagnostic wiring",
			Description:        "ensure diagnostic callback wiring",
			AcceptanceCriteria: "criterion 1",
		},
		StartCommit: "abc123",
		Tier:        provider.TierMedium,
		Result:      &runtypes.IterationResult{},
		PromptCtx:   &prompt.Context{WorkDir: workDir},
	}

	err := methExec.CheckTestsFailWithDiagnostic(context.Background(), bc)
	if !methodology.IsATDDAlreadyDone(err) {
		t.Fatalf("CheckTestsFailWithDiagnostic() error = %v, want ErrATDDAlreadyDone", err)
	}
	if gotDiagnosticCtx == nil {
		t.Fatal("expected RenderATDDDiagnostic callback to be invoked")
	}
	if gotDiagnosticCtx.BeadTitle != bc.Bead.Title {
		t.Fatalf("DiagnosticContext.BeadTitle = %q, want %q", gotDiagnosticCtx.BeadTitle, bc.Bead.Title)
	}
	if gotDiagnosticCtx.TestOutput != "VALIDATION_PASSED" {
		t.Fatalf("DiagnosticContext.TestOutput = %q, want %q", gotDiagnosticCtx.TestOutput, "VALIDATION_PASSED")
	}
	if gotInvokeTier != provider.TierLow {
		t.Fatalf("diagnostic invoke tier = %q, want %q", gotInvokeTier, provider.TierLow)
	}
	if gotInvokePrompt != renderedDiagnosticPrompt {
		t.Fatalf("diagnostic invoke prompt = %q, want %q", gotInvokePrompt, renderedDiagnosticPrompt)
	}
	if !strings.Contains(validateCommand, "-tags acceptance") {
		t.Fatalf("expected acceptance tags in validation command, got %q", validateCommand)
	}
	if validateWorkDir != workDir {
		t.Fatalf("validation workDir = %q, want %q", validateWorkDir, workDir)
	}
}

func TestMakeTDDOrchestrator_CoverageTrackerFlow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		covers            bool
		maxRejections     int
		wantCovered       int
		wantUntestable    int
		wantCoveredNumber int
		wantUntestableID  int
	}{
		{
			name:              "marks_criterion_covered",
			covers:            true,
			maxRejections:     2,
			wantCovered:       1,
			wantUntestable:    0,
			wantCoveredNumber: 2,
		},
		{
			name:             "records_rejection_for_targeted_criterion",
			covers:           false,
			maxRejections:    1,
			wantCovered:      0,
			wantUntestable:   1,
			wantUntestableID: 2,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Methodology: config.MethodologyConfig{
					MaxTDDCycles: 1,
				},
				Validation: config.ValidationConfig{
					Enabled:  true,
					Commands: []string{"go test ./internal/runner/..."},
				},
			}
			cfg.SetDefaults()
			cfg.NormalizeNilFields()

			var redSpecExcerpt string
			renderer := &mockPromptRenderer{
				RenderTDDRedFn: func(ctx *prompt.TDDRedContext) (string, error) {
					redSpecExcerpt = ctx.SpecExcerpt
					return "red-prompt", nil
				},
				RenderTDDGreenFn: func(ctx *prompt.TDDGreenContext) (string, error) {
					return "green-prompt", nil
				},
			}

			invocations := 0
			mockProvider := &mockProviderWithRouterTracking{
				streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
					invocations++
					if invocations == 1 {
						return &provider.Result{
							Success: true,
							Output:  `{"targeting": 2, "remaining": [1]}`,
						}, nil
					}
					return &provider.Result{
						Success: true,
						Output:  `{"targeting": 2, "remaining": []}`,
					}, nil
				},
			}
			router := provider.NewSingleProviderRouter(mockProvider)

			workDir := t.TempDir()
			commandCalls := 0
			var seenCommands []string
			var seenWorkDirs []string
			cmdRunner := func(ctx context.Context, command string, validationWorkDir string) (string, string, int, error) {
				commandCalls++
				seenCommands = append(seenCommands, command)
				seenWorkDirs = append(seenWorkDirs, validationWorkDir)
				if commandCalls == 1 {
					return "", "red failed as expected", 1, nil
				}
				return "ok", "", 0, nil
			}

			methExec := methodology.NewExecutor(cfg, io.Discard, nil, nil, nil)
			coverageCalls := 0
			gotCriterion := coverage.Criterion{}
			methExec.SetCoverageValidateFn(func(ctx context.Context, testCode string, criterion coverage.Criterion) (*coverage.ValidationResponse, error) {
				coverageCalls++
				gotCriterion = criterion
				return &coverage.ValidationResponse{Covers: tt.covers, Reason: "test"}, nil
			})

			r := &Runner{
				cfg:               cfg,
				router:            router,
				invoker:           newInvokerForTest(router, io.Discard, nil),
				renderer:          renderer,
				output:            io.Discard,
				methodologyExec:   methExec,
				validationRunner:  validation.NewRunner(cfg, cmdRunner, nil, nil),
				escalationHandler: escalation.NewHandler(cfg, nil, nil, nil, nil, nil, nil),
			}

			bc := &runtypes.BeadContext{
				Bead: &bead.Bead{
					ID:              "tdd-cov-1",
					Title:           "coverage run",
					ExpectedOutputs: []string{"criterion one", "criterion two"},
				},
				Tier:      provider.TierLow,
				PromptCtx: &prompt.Context{WorkDir: workDir},
				Result:    &runtypes.IterationResult{},
				ParentCtx: context.Background(),
			}
			criteria := []coverage.Criterion{
				{Number: 1, Text: "criterion one"},
				{Number: 2, Text: "criterion two"},
			}
			tracker := coverage.NewTracker(criteria, tt.maxRejections)

			orch := r.makeTDDOrchestrator()
			if err := orch.RunCycles(context.Background(), bc, tracker, criteria); err != nil {
				t.Fatalf("RunCycles returned error: %v", err)
			}

			if !strings.Contains(redSpecExcerpt, "## Coverage State") {
				t.Fatalf("red phase spec excerpt missing coverage state: %q", redSpecExcerpt)
			}
			if !strings.Contains(redSpecExcerpt, "Targeting criterion #1") {
				t.Fatalf("red phase spec excerpt missing first-target criterion: %q", redSpecExcerpt)
			}
			if coverageCalls != 1 {
				t.Fatalf("ValidateCoverage calls = %d, want 1", coverageCalls)
			}
			if gotCriterion.Number != 2 {
				t.Fatalf("ValidateCoverage criterion number = %d, want 2 (from self-report targeting)", gotCriterion.Number)
			}
			if commandCalls < 3 {
				t.Fatalf("validation command calls = %d, want at least 3 (red + green + final)", commandCalls)
			}
			if len(seenCommands) != commandCalls {
				t.Fatalf("seen commands count = %d, want %d", len(seenCommands), commandCalls)
			}
			for i, command := range seenCommands {
				if command != "go test ./internal/runner/..." {
					t.Fatalf("validation command[%d] = %q, want configured default command", i, command)
				}
			}
			for i, seenWorkDir := range seenWorkDirs {
				if seenWorkDir != workDir {
					t.Fatalf("validation workDir[%d] = %q, want %q", i, seenWorkDir, workDir)
				}
			}
			if len(tracker.CoveredCriteria()) != tt.wantCovered {
				t.Fatalf("covered criteria count = %d, want %d", len(tracker.CoveredCriteria()), tt.wantCovered)
			}
			if len(tracker.UntestableCriteria()) != tt.wantUntestable {
				t.Fatalf("untestable criteria count = %d, want %d", len(tracker.UntestableCriteria()), tt.wantUntestable)
			}
			if tt.wantCoveredNumber > 0 && tracker.CoveredCriteria()[0].Number != tt.wantCoveredNumber {
				t.Fatalf("covered criterion number = %d, want %d", tracker.CoveredCriteria()[0].Number, tt.wantCoveredNumber)
			}
			if tt.wantUntestableID > 0 && tracker.UntestableCriteria()[0].Number != tt.wantUntestableID {
				t.Fatalf("untestable criterion number = %d, want %d", tracker.UntestableCriteria()[0].Number, tt.wantUntestableID)
			}
			if bc.Result.CriteriaTotal != 2 {
				t.Fatalf("CriteriaTotal = %d, want 2", bc.Result.CriteriaTotal)
			}
			if bc.Result.CriteriaCovered != tt.wantCovered {
				t.Fatalf("CriteriaCovered = %d, want %d", bc.Result.CriteriaCovered, tt.wantCovered)
			}
			if bc.Result.CriteriaUntestable != tt.wantUntestable {
				t.Fatalf("CriteriaUntestable = %d, want %d", bc.Result.CriteriaUntestable, tt.wantUntestable)
			}
		})
	}
}
