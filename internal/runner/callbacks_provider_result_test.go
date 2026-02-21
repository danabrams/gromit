package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func setupInvokeFnWithProvider(t *testing.T, streamRunFn func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)) (escalation.InvokeFn, *runtypes.BeadContext) {
	t.Helper()

	var buf bytes.Buffer
	mockProvider := &mockProviderWithRouterTracking{
		streamRunFn: streamRunFn,
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg:      &config.Config{},
		router:   mockRouter,
		invoker:  newInvokerForTest(mockRouter, &buf, nil),
		output:   &buf,
		beads:    &mockBeadClient{},
		renderer: &mockPromptRenderer{},
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-2", Title: "Test"},
		Tier:        provider.TierMedium,
		Result:      &IterationResult{},
		BuildPrompt: "prompt",
		ParentCtx:   context.Background(),
	}

	return r.makeInvokeFn(), bc
}

// Expected failure: escalation.InvocationResult.ProviderResult does not exist yet
func TestMakeInvokeFn_PropagatesProviderResult(t *testing.T) {
	expected := &provider.Result{
		Success:         true,
		Output:          "provider output",
		ExitCode:        0,
		Model:           "test-model",
		ReasoningEffort: "low",
		CostUSD:         1.5,
		InputTokens:     12,
		OutputTokens:    34,
	}

	invokeFn, bc := setupInvokeFnWithProvider(t, func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		return expected, nil
	})

	result, err := invokeFn(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil invocation result")
	}
	if result.ProviderResult == nil {
		t.Fatal("expected ProviderResult to be set")
	}
	if result.ProviderResult != expected {
		t.Fatalf("ProviderResult = %+v, want %+v", result.ProviderResult, expected)
	}
	if bc.Result.Provider != "test-provider" {
		t.Fatalf("bc.Result.Provider = %q, want %q", bc.Result.Provider, "test-provider")
	}
	if bc.Result.FailureCategory != "" {
		t.Fatalf("bc.Result.FailureCategory = %q, want empty on success", bc.Result.FailureCategory)
	}
	if bc.Result.ReasoningEffort != "low" {
		t.Fatalf("bc.Result.ReasoningEffort = %q, want %q", bc.Result.ReasoningEffort, "low")
	}
}

// Expected failure: escalation.InvocationResult.ProviderResult does not exist yet
func TestMakeInvokeFn_PropagatesProviderResult_OnInvocationError(t *testing.T) {
	expected := &provider.Result{
		Success:      false,
		Output:       "provider output",
		ExitCode:     2,
		Model:        "test-model",
		CostUSD:      0.1,
		InputTokens:  5,
		OutputTokens: 7,
	}

	invokeFn, bc := setupInvokeFnWithProvider(t, func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		return expected, errors.New("boom")
	})

	result, err := invokeFn(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected error for invocation failure")
	}
	if result == nil {
		t.Fatal("expected non-nil invocation result")
	}
	if result.TimeoutType != "invocation" {
		t.Fatalf("TimeoutType = %q, want %q", result.TimeoutType, "invocation")
	}
	if result.ProviderResult == nil {
		t.Fatal("expected ProviderResult to be set on failure")
	}
	if result.ProviderResult != expected {
		t.Fatalf("ProviderResult = %+v, want %+v", result.ProviderResult, expected)
	}
	if bc.Result.Provider != "test-provider" {
		t.Fatalf("bc.Result.Provider = %q, want %q", bc.Result.Provider, "test-provider")
	}
	if bc.Result.FailureCategory != "" {
		t.Fatalf("bc.Result.FailureCategory = %q, want empty", bc.Result.FailureCategory)
	}
}

func TestMakeInvokeFn_DeadlineExceededSetsInvocationTimeout(t *testing.T) {
	invokeFn, bc := setupInvokeFnWithProvider(t, func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		return &provider.Result{Success: false, Model: "test-model"}, context.DeadlineExceeded
	})

	result, err := invokeFn(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected error for invocation deadline")
	}
	if result == nil {
		t.Fatal("expected non-nil invocation result")
	}
	if result.TimeoutType != "invocation" {
		t.Fatalf("TimeoutType = %q, want %q", result.TimeoutType, "invocation")
	}
	if bc.Result.TimeoutType != "invocation" {
		t.Fatalf("bc.Result.TimeoutType = %q, want %q", bc.Result.TimeoutType, "invocation")
	}
}

func TestMakeInvokeFn_PropagatesFailureCategoryToIterationResult(t *testing.T) {
	expected := &provider.Result{
		Success:         false,
		Output:          "provider output",
		ExitCode:        2,
		Model:           "test-model",
		FailureCategory: provider.FailureCategoryTransportDisconnect,
	}

	invokeFn, bc := setupInvokeFnWithProvider(t, func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		return expected, errors.New("boom")
	})

	_, _ = invokeFn(context.Background(), bc, "prompt")

	if bc.Result.FailureCategory != provider.FailureCategoryTransportDisconnect {
		t.Fatalf("bc.Result.FailureCategory = %q, want %q", bc.Result.FailureCategory, provider.FailureCategoryTransportDisconnect)
	}
}

func TestMakeInvokeFn_SetsBuildFailurePhaseOnScopeTooLarge(t *testing.T) {
	invokeFn, bc := setupInvokeFnWithProvider(t, func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		return &provider.Result{
			Success:  false,
			Output:   "SCOPE_TOO_LARGE: requires touching too many files",
			ExitCode: 1,
			Model:    "test-model",
		}, nil
	})

	_, err := invokeFn(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected scope-too-large error")
	}
	if bc.Result.FailurePhase != failurephase.Build {
		t.Fatalf("FailurePhase = %q, want %q", bc.Result.FailurePhase, failurephase.Build)
	}
}

func TestMakeInvokeFn_SetsBuildFailurePhaseOnUsageLimit(t *testing.T) {
	invokeFn, bc := setupInvokeFnWithProvider(t, func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
		return &provider.Result{
			Success:  false,
			Output:   "Error: usage limit exceeded",
			ExitCode: 1,
			Model:    "test-model",
		}, nil
	})

	_, err := invokeFn(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected usage-limit error")
	}
	if bc.Result.FailurePhase != failurephase.Build {
		t.Fatalf("FailurePhase = %q, want %q", bc.Result.FailurePhase, failurephase.Build)
	}
}

func TestMakeInvokeFn_EstimatesCostWhenZeroCostButTokensPresent(t *testing.T) {
	mockProvider := &mockProviderWithRouterTracking{
		name: "openai",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success:      true,
				Output:       "ok",
				ExitCode:     0,
				Model:        "test-model",
				CostUSD:      0,
				InputTokens:  2000,
				OutputTokens: 1000,
			}, nil
		},
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	var buf bytes.Buffer
	r := &Runner{
		cfg: &config.Config{},
		providerCostDefs: map[string]config.ProviderDef{
			"openai": {
				CostPer1kInput:  0.010,
				CostPer1kOutput: 0.030,
			},
		},
		router:  mockRouter,
		invoker: newInvokerForTest(mockRouter, &buf, nil),
		output:  &buf,
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-cost", Title: "Cost Test"},
		Tier:        provider.TierMedium,
		Result:      &IterationResult{},
		BuildPrompt: "prompt",
		ParentCtx:   context.Background(),
	}

	invokeFn := r.makeInvokeFn()
	_, err := invokeFn(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: 2000/1000 * 0.010 + 1000/1000 * 0.030 = 0.020 + 0.030 = 0.050
	wantCost := 0.050
	if bc.Result.CostUSD != wantCost {
		t.Fatalf("bc.Result.CostUSD = %v, want %v", bc.Result.CostUSD, wantCost)
	}
}

func TestMakeInvokeFn_SkipsCostEstimationWhenCostAlreadyPresent(t *testing.T) {
	mockProvider := &mockProviderWithRouterTracking{
		name: "openai",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success:      true,
				Output:       "ok",
				ExitCode:     0,
				Model:        "test-model",
				CostUSD:      1.23,
				InputTokens:  2000,
				OutputTokens: 1000,
			}, nil
		},
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	var buf bytes.Buffer
	r := &Runner{
		cfg: &config.Config{},
		providerCostDefs: map[string]config.ProviderDef{
			"openai": {
				CostPer1kInput:  0.010,
				CostPer1kOutput: 0.030,
			},
		},
		router:  mockRouter,
		invoker: newInvokerForTest(mockRouter, &buf, nil),
		output:  &buf,
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-cost2", Title: "Cost Test"},
		Tier:        provider.TierMedium,
		Result:      &IterationResult{},
		BuildPrompt: "prompt",
		ParentCtx:   context.Background(),
	}

	invokeFn := r.makeInvokeFn()
	_, err := invokeFn(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use the provider-reported cost, not the estimate
	if bc.Result.CostUSD != 1.23 {
		t.Fatalf("bc.Result.CostUSD = %v, want 1.23 (provider-reported cost)", bc.Result.CostUSD)
	}
}

// TestMakeInvokeFn_EstimatesCostWithProviderNameIndirection verifies that cost estimation
// works when the runtime provider name ("codex") differs from the config key ("openai").
// This is the core bug being fixed: the providerCostDefs map is keyed by p.Name().
func TestMakeInvokeFn_EstimatesCostWithProviderNameIndirection(t *testing.T) {
	mockProvider := &mockProviderWithRouterTracking{
		name: "codex", // runtime name differs from config key "openai"
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success:      true,
				Output:       "ok",
				ExitCode:     0,
				Model:        "test-model",
				CostUSD:      0,
				InputTokens:  2000,
				OutputTokens: 1000,
			}, nil
		},
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	var buf bytes.Buffer
	r := &Runner{
		cfg: &config.Config{},
		providerCostDefs: map[string]config.ProviderDef{
			"codex": { // keyed by runtime name, not config key
				CostPer1kInput:  0.003,
				CostPer1kOutput: 0.012,
			},
		},
		router:  mockRouter,
		invoker: newInvokerForTest(mockRouter, &buf, nil),
		output:  &buf,
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-indirection", Title: "Name Indirection Test"},
		Tier:        provider.TierMedium,
		Result:      &IterationResult{},
		BuildPrompt: "prompt",
		ParentCtx:   context.Background(),
	}

	invokeFn := r.makeInvokeFn()
	_, err := invokeFn(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: 2000/1000 * 0.003 + 1000/1000 * 0.012 = 0.006 + 0.012 = 0.018
	wantCost := 0.018
	const epsilon = 1e-9
	diff := bc.Result.CostUSD - wantCost
	if diff < -epsilon || diff > epsilon {
		t.Fatalf("bc.Result.CostUSD = %v, want ~%v", bc.Result.CostUSD, wantCost)
	}
}

// TestMakeInvokeFn_NoCostEstimationWhenPricingNotConfigured verifies that cost estimation
// returns 0 when no pricing is configured for the provider.
func TestMakeInvokeFn_NoCostEstimationWhenPricingNotConfigured(t *testing.T) {
	mockProvider := &mockProviderWithRouterTracking{
		name: "codex",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success:      true,
				Output:       "ok",
				ExitCode:     0,
				Model:        "test-model",
				CostUSD:      0,
				InputTokens:  2000,
				OutputTokens: 1000,
			}, nil
		},
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	var buf bytes.Buffer
	r := &Runner{
		cfg:              &config.Config{},
		providerCostDefs: map[string]config.ProviderDef{}, // no pricing configured
		router:           mockRouter,
		invoker:          newInvokerForTest(mockRouter, &buf, nil),
		output:           &buf,
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-nopricing", Title: "No Pricing Test"},
		Tier:        provider.TierMedium,
		Result:      &IterationResult{},
		BuildPrompt: "prompt",
		ParentCtx:   context.Background(),
	}

	invokeFn := r.makeInvokeFn()
	_, err := invokeFn(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bc.Result.CostUSD != 0 {
		t.Fatalf("bc.Result.CostUSD = %v, want 0 (no pricing configured)", bc.Result.CostUSD)
	}
}

func TestMakeInvokeFn_RefreshesScopedTestCommandFromGitDiffBeforeBuildInvocation(t *testing.T) {
	var capturedPrompt string

	mockProvider := &mockProviderWithRouterTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			capturedPrompt = prompt
			return &provider.Result{Success: true, Output: "ok", ExitCode: 0, Model: "test-model"}, nil
		},
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	var buf bytes.Buffer
	renderer := &mockPromptRenderer{
		RenderBuildFn: func(ctx *prompt.Context) (string, error) {
			return "Self-check command: " + ctx.ScopedTestCommand, nil
		},
	}
	r := &Runner{
		cfg:      &config.Config{},
		router:   mockRouter,
		invoker:  newInvokerForTest(mockRouter, &buf, nil),
		output:   &buf,
		renderer: renderer,
		gitDiffFn: func(fromCommit string) (string, error) {
			return "diff --git a/internal/runner/process.go b/internal/runner/process.go\n" +
				"diff --git a/internal/config/config.go b/internal/config/config.go\n", nil
		},
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-3", Title: "Test"},
		Tier:        provider.TierMedium,
		Result:      &IterationResult{},
		BuildPrompt: "base prompt",
		PromptCtx:   &prompt.Context{},
		StartCommit: "abc123",
		ParentCtx:   context.Background(),
	}

	invokeFn := r.makeInvokeFn()
	_, err := invokeFn(context.Background(), bc, "ignored")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(capturedPrompt, "go test ./internal/runner/... ./internal/config/...") {
		t.Fatalf("provider prompt = %q, want scoped go test command derived from git diff", capturedPrompt)
	}
}
