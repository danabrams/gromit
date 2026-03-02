package runner

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestNewBuildExecutionInvoker_ConfiguresPreserveProviderStreamFromConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.SetDefaults()

	handlerWasNil := false
	mp := &mockStreamingProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			handlerWasNil = handler == nil
			return &provider.Result{Success: true, Model: "model"}, nil
		},
	}
	router := provider.NewSingleProviderRouter(mp)

	invoker := newBuildExecutionInvoker(cfg, router, io.Discard, nil)
	bc := newInvokerTestBeadContext()

	if _, err := invoker.Execute(context.Background(), bc, "prompt"); err != nil {
		t.Fatalf("unexpected Execute error: %v", err)
	}
	if !handlerWasNil {
		t.Fatal("expected nil event handler when preserve_provider_output is enabled in config")
	}
}

func TestNewBuildExecutionInvoker_RespectsPreserveProviderOutputDisabled(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Stream.PreserveProviderOutput = boolRef(false)

	handlerWasNil := false
	mp := &mockStreamingProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			handlerWasNil = handler == nil
			return &provider.Result{Success: true, Model: "model"}, nil
		},
	}
	router := provider.NewSingleProviderRouter(mp)

	invoker := newBuildExecutionInvoker(cfg, router, io.Discard, nil)
	bc := newInvokerTestBeadContext()

	if _, err := invoker.Execute(context.Background(), bc, "prompt"); err != nil {
		t.Fatalf("unexpected Execute error: %v", err)
	}
	if handlerWasNil {
		t.Fatal("expected non-nil event handler when preserve_provider_output is disabled in config")
	}
}

func newInvokerTestBeadContext() *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		BuildPrompt: "test prompt",
		Result:      &runtypes.IterationResult{},
	}
}

func boolRef(value bool) *bool {
	return &value
}

type mockStreamingProvider struct {
	streamRunFn func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
}

func (m *mockStreamingProvider) Name() string {
	return "mock-provider"
}

func (m *mockStreamingProvider) ModelForTier(tier string) string {
	return tier
}

func (m *mockStreamingProvider) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: tier}, nil
}

func (m *mockStreamingProvider) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	return &provider.Result{Success: true, Model: tier}, nil
}

func (m *mockStreamingProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: tier}, nil
}

func (m *mockStreamingProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

func (m *mockStreamingProvider) IsValidationPassed(result *provider.Result) bool {
	return true
}

func (m *mockStreamingProvider) IsScopeTooLarge(result *provider.Result) (bool, string) {
	return false, ""
}
