package main

import (
	"bytes"
	"context"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/retro"
)

func TestRetroRouterAdapter_ConformsToProviderRunner(t *testing.T) {
	var _ retro.ProviderRunner = (*retroRouterAdapter)(nil)
}

func TestRetroRouterAdapterCompileTimeCheckLivesInProduction(t *testing.T) {
	const sentinel = "var _ retro.ProviderRunner = (*retroRouterAdapter)(nil)"

	prod, err := os.ReadFile("cmd/gromit/adapters.go")
	if err != nil {
		t.Fatalf("reading production adapters file: %v", err)
	}
	if !strings.Contains(string(prod), sentinel) {
		t.Fatalf("expected compile-time check in production file %q", sentinel)
	}
}

func TestRetroRouterAdapterCompileTimeCheckIsPackageLevel(t *testing.T) {
	const sentinel = "var _ retro.ProviderRunner = (*retroRouterAdapter)(nil)"

	fset := token.NewFileSet()
	prod, err := parser.ParseFile(fset, "cmd/gromit/adapters.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing production adapters file: %v", err)
	}

	for _, decl := range prod.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		var buf bytes.Buffer
		if err := format.Node(&buf, fset, genDecl); err != nil {
			t.Fatalf("formatting declaration: %v", err)
		}
		if strings.Contains(buf.String(), sentinel) {
			return
		}
	}

	t.Fatalf("expected package-level compile-time assertion %q in production file", sentinel)
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

func TestRetroRouterAdapterRun_RetriesWithFallbackProviderOnUsageLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	wantPrompt := "retro prompt"
	wantTier := provider.TierMedium
	wantPhase := "retro"
	wantResult := &provider.Result{Success: true, ExitCode: 0, Output: "retro-ok"}
	usageErr := io.EOF

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
			return nil, usageErr
		},
		IsUsageLimitErrorFn: func(result *provider.Result, err error) bool {
			return result == nil && err == usageErr
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

func TestRetroRouterAdapterStreamRun_RetriesWithFallbackProviderOnUsageLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	wantPrompt := "retro stream prompt"
	wantTier := provider.TierMedium
	wantPhase := "retro"
	wantResult := &provider.Result{Success: true, ExitCode: 0, Output: "stream-ok"}
	output := io.Discard
	usageErr := io.EOF

	handler := func([]byte) {}
	toolHandler := func(provider.ToolEvent) {}

	firstProviderRunCalls := 0
	secondProviderRunCalls := 0
	selectCalls := 0
	markedUnavailable := ""

	firstProvider := &reviewProviderStub{
		NameFn: func() string { return "first" },
		StreamRunFn: func(
			runCtx context.Context,
			prompt string,
			tier string,
			gotOutput io.Writer,
			gotHandler provider.EventHandler,
			gotToolHandler provider.ToolCallHandler,
		) (*provider.Result, error) {
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
			if gotOutput != output {
				t.Fatal("output writer mismatch")
			}
			if gotHandler == nil {
				t.Fatal("event handler should be forwarded")
			}
			if gotToolHandler == nil {
				t.Fatal("tool handler should be forwarded")
			}
			return nil, usageErr
		},
		IsUsageLimitErrorFn: func(result *provider.Result, err error) bool {
			return result == nil && err == usageErr
		},
	}
	secondProvider := &reviewProviderStub{
		NameFn: func() string { return "second" },
		StreamRunFn: func(
			runCtx context.Context,
			prompt string,
			tier string,
			gotOutput io.Writer,
			gotHandler provider.EventHandler,
			gotToolHandler provider.ToolCallHandler,
		) (*provider.Result, error) {
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
			if gotOutput != output {
				t.Fatal("output writer mismatch")
			}
			if gotHandler == nil {
				t.Fatal("event handler should be forwarded")
			}
			if gotToolHandler == nil {
				t.Fatal("tool handler should be forwarded")
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

	got, err := adapter.StreamRun(ctx, wantPrompt, wantTier, output, handler, toolHandler)
	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}
	if got != wantResult {
		t.Fatalf("StreamRun() result = %#v, want %#v", got, wantResult)
	}
	if firstProviderRunCalls != 1 {
		t.Fatalf("first provider StreamRun calls = %d, want 1", firstProviderRunCalls)
	}
	if secondProviderRunCalls != 1 {
		t.Fatalf("second provider StreamRun calls = %d, want 1", secondProviderRunCalls)
	}
	if selectCalls != 2 {
		t.Fatalf("router Select calls = %d, want 2", selectCalls)
	}
	if markedUnavailable != "first" {
		t.Fatalf("marked unavailable provider = %q, want %q", markedUnavailable, "first")
	}
}
