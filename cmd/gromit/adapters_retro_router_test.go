package main

import (
	"context"
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
