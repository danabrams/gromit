package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/provider"
)

func TestRetroRouterAdapterRun_RetriesOnUsageLimitResultWithoutError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	wantPrompt := "retro prompt"
	wantTier := provider.TierMedium
	wantPhase := "retro"
	wantResult := &provider.Result{Success: true, ExitCode: 0, Output: "retro-ok"}

	firstProviderRunCalls := 0
	secondProviderRunCalls := 0
	selectCalls := 0
	markedUnavailable := ""

	firstProvider := &reviewProviderStub{
		NameFn: func() string { return "first" },
		RunFn: func(runCtx context.Context, prompt string, tier string) (*provider.Result, error) {
			firstProviderRunCalls++
			if runCtx != ctx {
				t.Fatalf("ctx mismatch")
			}
			if prompt != wantPrompt {
				t.Fatalf("prompt = %q, want %q", prompt, wantPrompt)
			}
			if tier != wantTier {
				t.Fatalf("tier = %q, want %q", tier, wantTier)
			}
			return &provider.Result{Success: false, Output: "usage limit exceeded"}, nil
		},
		IsUsageLimitErrorFn: func(result *provider.Result, err error) bool {
			return err == nil && result != nil && !result.Success
		},
	}
	secondProvider := &reviewProviderStub{
		NameFn: func() string { return "second" },
		RunFn: func(runCtx context.Context, prompt string, tier string) (*provider.Result, error) {
			secondProviderRunCalls++
			if runCtx != ctx {
				t.Fatalf("ctx mismatch")
			}
			if prompt != wantPrompt {
				t.Fatalf("prompt = %q, want %q", prompt, wantPrompt)
			}
			if tier != wantTier {
				t.Fatalf("tier = %q, want %q", tier, wantTier)
			}
			return wantResult, nil
		},
	}
	mockRouter := &reviewRouterStub{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			if phase != wantPhase {
				t.Fatalf("phase = %q, want %q", phase, wantPhase)
			}
			if tier != wantTier {
				t.Fatalf("tier = %q, want %q", tier, wantTier)
			}
			selectCalls++
			if selectCalls == 1 {
				return firstProvider, "selected-first-model"
			}
			return secondProvider, "selected-second-model"
		},
		MarkUnavailableFn: func(name string) {
			markedUnavailable = name
		},
	}
	adapter := &retroRouterAdapter{
		Router: mockRouter,
		Phase:  wantPhase,
	}

	got, err := adapter.Run(ctx, wantPrompt, wantTier)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got != wantResult {
		t.Fatalf("Run() result = %#v, want %#v", got, wantResult)
	}
	if firstProviderRunCalls != 1 {
		t.Fatalf("first provider Run calls = %d, want 1", firstProviderRunCalls)
	}
	if secondProviderRunCalls != 1 {
		t.Fatalf("second provider Run calls = %d, want 1", secondProviderRunCalls)
	}
	if selectCalls != 2 {
		t.Fatalf("router Select calls = %d, want 2", selectCalls)
	}
	if markedUnavailable != "first" {
		t.Fatalf("marked unavailable provider = %q, want %q", markedUnavailable, "first")
	}
}

func TestRetroRouterAdapterRun_ReturnsProviderExhaustedErrorAfterUsageLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	wantPrompt := "retro prompt"
	wantTier := provider.TierLow
	wantPhase := "retro"
	usageErr := context.DeadlineExceeded

	runCalls := 0
	selectCalls := 0
	markedUnavailable := ""

	firstProvider := &reviewProviderStub{
		NameFn: func() string { return "first" },
		RunFn: func(runCtx context.Context, prompt string, tier string) (*provider.Result, error) {
			runCalls++
			if runCtx != ctx {
				t.Fatalf("ctx mismatch")
			}
			if prompt != wantPrompt {
				t.Fatalf("prompt = %q, want %q", prompt, wantPrompt)
			}
			if tier != wantTier {
				t.Fatalf("tier = %q, want %q", tier, wantTier)
			}
			return nil, usageErr
		},
		IsUsageLimitErrorFn: func(result *provider.Result, err error) bool {
			return result == nil && err == usageErr
		},
	}

	mockRouter := &reviewRouterStub{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			if phase != wantPhase {
				t.Fatalf("phase = %q, want %q", phase, wantPhase)
			}
			if tier != wantTier {
				t.Fatalf("tier = %q, want %q", tier, wantTier)
			}
			selectCalls++
			if selectCalls == 1 {
				return firstProvider, "selected-first-model"
			}
			return nil, ""
		},
		MarkUnavailableFn: func(name string) {
			markedUnavailable = name
		},
	}
	adapter := &retroRouterAdapter{
		Router: mockRouter,
		Phase:  wantPhase,
	}

	got, err := adapter.Run(ctx, wantPrompt, wantTier)
	if err == nil {
		t.Fatal("Run() error = nil, want provider exhausted error")
	}
	if got != nil {
		t.Fatalf("Run() result = %#v, want nil when providers are exhausted", got)
	}
	if err.Error() != `no providers available for phase "retro" and tier "low"` {
		t.Fatalf("Run() error = %q, want %q", err.Error(), `no providers available for phase "retro" and tier "low"`)
	}
	if runCalls != 1 {
		t.Fatalf("first provider Run calls = %d, want 1", runCalls)
	}
	if selectCalls != 2 {
		t.Fatalf("router Select calls = %d, want 2", selectCalls)
	}
	if markedUnavailable != "first" {
		t.Fatalf("marked unavailable provider = %q, want %q", markedUnavailable, "first")
	}
}

func TestRetroRouterAdapterStreamRun_RetriesOnUsageLimitResultWithoutError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	wantPrompt := "retro stream prompt"
	wantTier := provider.TierMedium
	wantPhase := "retro"
	wantResult := &provider.Result{Success: true, ExitCode: 0, Output: "retro-stream-ok"}

	firstProviderStreamCalls := 0
	secondProviderStreamCalls := 0
	selectCalls := 0
	markedUnavailable := ""

	firstProvider := &reviewProviderStub{
		NameFn: func() string { return "first" },
		StreamRunFn: func(runCtx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			firstProviderStreamCalls++
			if runCtx != ctx {
				t.Fatalf("ctx mismatch")
			}
			if prompt != wantPrompt {
				t.Fatalf("prompt = %q, want %q", prompt, wantPrompt)
			}
			if tier != wantTier {
				t.Fatalf("tier = %q, want %q", tier, wantTier)
			}
			return &provider.Result{Success: false, Output: "usage limit exceeded"}, nil
		},
		IsUsageLimitErrorFn: func(result *provider.Result, err error) bool {
			return err == nil && result != nil && !result.Success
		},
	}
	secondProvider := &reviewProviderStub{
		NameFn: func() string { return "second" },
		StreamRunFn: func(runCtx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			secondProviderStreamCalls++
			if runCtx != ctx {
				t.Fatalf("ctx mismatch")
			}
			if prompt != wantPrompt {
				t.Fatalf("prompt = %q, want %q", prompt, wantPrompt)
			}
			if tier != wantTier {
				t.Fatalf("tier = %q, want %q", tier, wantTier)
			}
			return wantResult, nil
		},
	}

	mockRouter := &reviewRouterStub{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			if phase != wantPhase {
				t.Fatalf("phase = %q, want %q", phase, wantPhase)
			}
			if tier != wantTier {
				t.Fatalf("tier = %q, want %q", tier, wantTier)
			}
			selectCalls++
			if selectCalls == 1 {
				return firstProvider, "selected-first-model"
			}
			return secondProvider, "selected-second-model"
		},
		MarkUnavailableFn: func(name string) {
			markedUnavailable = name
		},
	}
	adapter := &retroRouterAdapter{
		Router: mockRouter,
		Phase:  wantPhase,
	}

	got, err := adapter.StreamRun(ctx, wantPrompt, wantTier, io.Discard, nil, nil)
	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}
	if got != wantResult {
		t.Fatalf("StreamRun() result = %#v, want %#v", got, wantResult)
	}
	if firstProviderStreamCalls != 1 {
		t.Fatalf("first provider StreamRun calls = %d, want 1", firstProviderStreamCalls)
	}
	if secondProviderStreamCalls != 1 {
		t.Fatalf("second provider StreamRun calls = %d, want 1", secondProviderStreamCalls)
	}
	if selectCalls != 2 {
		t.Fatalf("router Select calls = %d, want 2", selectCalls)
	}
	if markedUnavailable != "first" {
		t.Fatalf("marked unavailable provider = %q, want %q", markedUnavailable, "first")
	}
}

func TestRetroRouterAdapterRun_LogsWhenSelectFails(t *testing.T) {
	wantPhase := "retro"
	wantTier := provider.TierMedium
	selectCalls := 0
	mockRouter := &reviewRouterStub{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			selectCalls++
			return nil, ""
		},
	}
	adapter := &retroRouterAdapter{
		Router: mockRouter,
		Phase:  wantPhase,
	}

	var err error
	output := captureStderr(t, func() {
		_, err = adapter.Run(context.Background(), "prompt", wantTier)
	})
	if err == nil {
		t.Fatal("Run() error = nil, want failure when no providers are available")
	}
	if selectCalls != 1 {
		t.Fatalf("router Select calls = %d, want 1", selectCalls)
	}
	if !strings.Contains(output, "router.Select failed") {
		t.Fatalf("stderr output %q does not mention router.Select failure", output)
	}
	if !strings.Contains(output, "phase \""+wantPhase+"\"") {
		t.Fatalf("stderr output %q does not contain phase %q", output, wantPhase)
	}
	if !strings.Contains(output, "tier \""+wantTier+"\"") {
		t.Fatalf("stderr output %q does not contain tier %q", output, wantTier)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	r.Close()
	return buf.String()
}

func TestProviderRouterClientAdapterRun_LogsWhenFallbackSelectFails(t *testing.T) {
	usageErr := context.DeadlineExceeded
	mockProvider := &reviewProviderStub{
		NameFn: func() string { return "primary" },
		RunFn: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return nil, usageErr
		},
		IsUsageLimitErrorFn: func(result *provider.Result, err error) bool {
			return result == nil && err == usageErr
		},
	}

	selectCalls := 0
	router := &reviewRouterStub{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			selectCalls++
			if phase != reviewSessionCommand {
				t.Fatalf("phase = %q, want %q", phase, reviewSessionCommand)
			}
			if selectCalls == 1 {
				return mockProvider, "legacy-model"
			}
			return nil, ""
		},
		MarkUnavailableFn: func(name string) {
			if name != mockProvider.Name() {
				t.Fatalf("MarkUnavailable called with %q, want %q", name, mockProvider.Name())
			}
		},
	}

	adapter := &llmRouterClientAdapter{
		Router:  router,
		Timeout: 1 * time.Second,
		Phase:   reviewSessionCommand,
	}

	output := captureStderr(t, func() {
		_, _ = adapter.Run("prompt", "haiku")
	})
	if selectCalls != 2 {
		t.Fatalf("router Select calls = %d, want 2", selectCalls)
	}
	if !strings.Contains(output, "router.Select failed") {
		t.Fatalf("stderr output %q does not mention router.Select failure", output)
	}
	if !strings.Contains(output, "phase \""+reviewSessionCommand+"\"") {
		t.Fatalf("stderr output %q does not contain phase %q", output, reviewSessionCommand)
	}
	if !strings.Contains(output, "tier \""+provider.TierFromLegacyModel("haiku")+"\"") {
		t.Fatalf("stderr output %q does not contain tier mapping", output)
	}
}

func TestRetroRouterAdapterStreamRun_ReturnsProviderExhaustedErrorAfterUsageLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	wantPrompt := "retro stream prompt"
	wantTier := provider.TierLow
	wantPhase := "retro"
	usageErr := context.DeadlineExceeded

	streamRunCalls := 0
	selectCalls := 0
	markedUnavailable := ""

	firstProvider := &reviewProviderStub{
		NameFn: func() string { return "first" },
		StreamRunFn: func(runCtx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			streamRunCalls++
			if runCtx != ctx {
				t.Fatalf("ctx mismatch")
			}
			if prompt != wantPrompt {
				t.Fatalf("prompt = %q, want %q", prompt, wantPrompt)
			}
			if tier != wantTier {
				t.Fatalf("tier = %q, want %q", tier, wantTier)
			}
			return nil, usageErr
		},
		IsUsageLimitErrorFn: func(result *provider.Result, err error) bool {
			return result == nil && err == usageErr
		},
	}

	mockRouter := &reviewRouterStub{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			if phase != wantPhase {
				t.Fatalf("phase = %q, want %q", phase, wantPhase)
			}
			if tier != wantTier {
				t.Fatalf("tier = %q, want %q", tier, wantTier)
			}
			selectCalls++
			if selectCalls == 1 {
				return firstProvider, "selected-first-model"
			}
			return nil, ""
		},
		MarkUnavailableFn: func(name string) {
			markedUnavailable = name
		},
	}
	adapter := &retroRouterAdapter{
		Router: mockRouter,
		Phase:  wantPhase,
	}

	got, err := adapter.StreamRun(ctx, wantPrompt, wantTier, io.Discard, nil, nil)
	if err == nil {
		t.Fatal("StreamRun() error = nil, want provider exhausted error")
	}
	if got != nil {
		t.Fatalf("StreamRun() result = %#v, want nil when providers are exhausted", got)
	}
	if err.Error() != `no providers available for phase "retro" and tier "low"` {
		t.Fatalf("StreamRun() error = %q, want %q", err.Error(), `no providers available for phase "retro" and tier "low"`)
	}
	if streamRunCalls != 1 {
		t.Fatalf("first provider StreamRun calls = %d, want 1", streamRunCalls)
	}
	if selectCalls != 2 {
		t.Fatalf("router Select calls = %d, want 2", selectCalls)
	}
	if markedUnavailable != "first" {
		t.Fatalf("marked unavailable provider = %q, want %q", markedUnavailable, "first")
	}
}

func TestNewAgentResolver_ReturnsAgentResolver(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	resolver := agent.NewResolver(cfg)

	if resolver == nil {
		t.Fatal("agent.NewResolver returned nil")
	}

	// Verify it implements pipeline.AgentResolver
	var _ pipeline.AgentResolver = resolver
}
