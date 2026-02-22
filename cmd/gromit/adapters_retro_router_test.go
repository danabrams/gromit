package main

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/retro"
)

func TestRetroRouterAdapter_ConformsToProviderRunner(t *testing.T) {
	var _ retro.ProviderRunner = (*retroRouterAdapter)(nil)
}

func TestRetroRouterAdapterRun_DelegatesToRouterSelectedProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	wantPrompt := "retro prompt"
	wantTier := provider.TierHigh
	wantPhase := "retro"
	wantResult := &provider.Result{Success: true, ExitCode: 0, Output: "retro-ok"}

	var gotPhase, gotTier string
	var runCalled bool

	mockProvider := &reviewProviderStub{
		RunFn: func(runCtx context.Context, prompt string, tier string) (*provider.Result, error) {
			if runCtx != ctx {
				t.Fatalf("ctx mismatch")
			}
			if prompt != wantPrompt {
				t.Fatalf("prompt = %q, want %q", prompt, wantPrompt)
			}
			gotTier = tier
			runCalled = true
			return wantResult, nil
		},
	}
	mockRouter := &reviewRouterStub{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			gotPhase = phase
			gotTier = tier
			return mockProvider, "selected-model"
		},
	}
	adapter := &retroRouterAdapter{
		Router: mockRouter,
		Phase:  wantPhase,
	}

	got, err := adapter.Run(ctx, wantPrompt, wantTier)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got != wantResult {
		t.Fatalf("Run() result = %#v, want %#v", got, wantResult)
	}
	if gotPhase != wantPhase {
		t.Fatalf("phase = %q, want %q", gotPhase, wantPhase)
	}
	if gotTier != wantTier {
		t.Fatalf("tier = %q, want %q", gotTier, wantTier)
	}
	if !runCalled {
		t.Fatal("expected provider Run to be called")
	}
}

func TestRetroRouterAdapterStreamRun_DelegatesToRouterSelectedProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	wantPrompt := "retro stream prompt"
	wantTier := provider.TierMedium
	wantPhase := "retro"
	wantResult := &provider.Result{Success: true, ExitCode: 0, Output: "stream-ok"}
	output := io.Discard

	handlerCalled := false
	handler := func(line []byte) {
		handlerCalled = true
	}
	toolCalled := false
	toolHandler := func(call provider.ToolEvent) {
		toolCalled = true
	}

	var gotPhase, gotTier string
	var streamRunCalled bool

	mockProvider := &reviewProviderStub{
		StreamRunFn: func(
			runCtx context.Context,
			prompt string,
			tier string,
			gotOutput io.Writer,
			gotHandler provider.EventHandler,
			gotToolHandler provider.ToolCallHandler,
		) (*provider.Result, error) {
			if runCtx != ctx {
				t.Fatalf("ctx mismatch")
			}
			if prompt != wantPrompt {
				t.Fatalf("prompt = %q, want %q", prompt, wantPrompt)
			}
			if tier != wantTier {
				t.Fatalf("tier = %q, want %q", tier, wantTier)
			}
			if gotOutput != output {
				t.Fatal("output writer mismatch")
			}
			if gotHandler == nil {
				t.Fatal("event handler should be forwarded")
			}
			if gotToolHandler == nil {
				t.Fatal("tool handler should be forwarded")
			}
			gotHandler([]byte("event"))
			gotToolHandler(provider.ToolEvent{})
			streamRunCalled = true
			return wantResult, nil
		},
	}
	mockRouter := &reviewRouterStub{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			gotPhase = phase
			gotTier = tier
			return mockProvider, "selected-model"
		},
	}
	adapter := &retroRouterAdapter{
		Router: mockRouter,
		Phase:  wantPhase,
	}

	got, err := adapter.StreamRun(ctx, wantPrompt, wantTier, output, handler, toolHandler)
	if err != nil {
		t.Fatalf("StreamRun() error = %v", err)
	}
	if got != wantResult {
		t.Fatalf("StreamRun() result = %#v, want %#v", got, wantResult)
	}
	if gotPhase != wantPhase {
		t.Fatalf("phase = %q, want %q", gotPhase, wantPhase)
	}
	if gotTier != wantTier {
		t.Fatalf("tier = %q, want %q", gotTier, wantTier)
	}
	if !streamRunCalled {
		t.Fatal("expected provider StreamRun to be called")
	}
	if !handlerCalled {
		t.Fatal("expected event handler to be callable when forwarded")
	}
	if !toolCalled {
		t.Fatal("expected tool handler to be callable when forwarded")
	}
}
