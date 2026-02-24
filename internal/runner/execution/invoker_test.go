package execution

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// --- Mock types for narrow interfaces ---

// mockRouter implements the narrow Router interface for the execution package.
type mockRouter struct {
	selectFn          func(phase, tier string) (Provider, string)
	markUnavailableFn func(name string)
	recordOutcomeFn   func(providerName, failureCategory string)
	markCalls         []string
	recordCalls       []recordCall
}

type recordCall struct {
	providerName    string
	failureCategory string
}

func (m *mockRouter) Select(phase, tier string) (Provider, string) {
	if m.selectFn != nil {
		return m.selectFn(phase, tier)
	}
	return nil, ""
}

func (m *mockRouter) MarkUnavailable(name string) {
	m.markCalls = append(m.markCalls, name)
	if m.markUnavailableFn != nil {
		m.markUnavailableFn(name)
	}
}

func (m *mockRouter) RecordOutcome(providerName, failureCategory string) {
	m.recordCalls = append(m.recordCalls, recordCall{
		providerName:    providerName,
		failureCategory: failureCategory,
	})
	if m.recordOutcomeFn != nil {
		m.recordOutcomeFn(providerName, failureCategory)
	}
}

// mockProvider implements the narrow Provider interface for the execution package.
type mockProvider struct {
	name           string
	streamRunFn    func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	isUsageLimitFn func(result *provider.Result, err error) bool
	cacheAdapter   provider.CacheAdapter
}

func (m *mockProvider) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock-provider"
}

func (m *mockProvider) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	if m.isUsageLimitFn != nil {
		return m.isUsageLimitFn(result, err)
	}
	return false
}

func (m *mockProvider) ModelForTier(tier string) string {
	return "test-model"
}

func (m *mockProvider) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProvider) IsValidationPassed(result *provider.Result) bool {
	return result.Success
}

func (m *mockProvider) IsScopeTooLarge(result *provider.Result) (bool, string) {
	return false, ""
}

func (m *mockProvider) CacheAdapter() provider.CacheAdapter {
	if m.cacheAdapter == nil {
		return provider.NewNoopCacheAdapter()
	}
	return m.cacheAdapter
}

type mockCacheAdapter struct {
	lookupFn     func(ctx context.Context, req provider.CacheLookupRequest) (*provider.CacheEntry, bool, error)
	writeFn      func(ctx context.Context, req provider.CacheWriteRequest) error
	invalidateFn func(ctx context.Context, req provider.CacheInvalidateRequest) error
}

func (m *mockCacheAdapter) Lookup(ctx context.Context, req provider.CacheLookupRequest) (*provider.CacheEntry, bool, error) {
	if m.lookupFn != nil {
		return m.lookupFn(ctx, req)
	}
	return nil, false, nil
}

func (m *mockCacheAdapter) Write(ctx context.Context, req provider.CacheWriteRequest) error {
	if m.writeFn != nil {
		return m.writeFn(ctx, req)
	}
	return nil
}

func (m *mockCacheAdapter) Invalidate(ctx context.Context, req provider.CacheInvalidateRequest) error {
	if m.invalidateFn != nil {
		return m.invalidateFn(ctx, req)
	}
	return nil
}

// --- Helper ---

func newTestBeadContext() *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Tier:        provider.TierMedium,
		BuildPrompt: "test prompt",
		Result:      &runtypes.IterationResult{},
	}
}

func readStreamLogLines(t *testing.T, sl *logger.StreamLogger) []string {
	t.Helper()
	if sl == nil {
		t.Fatal("stream logger is nil")
	}
	if err := sl.Close(); err != nil {
		t.Fatalf("closing stream logger: %v", err)
	}
	content, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatalf("reading stream log: %v", err)
	}
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func lineIndex(lines []string, substring string) int {
	for i, line := range lines {
		if strings.Contains(line, substring) {
			return i
		}
	}
	return -1
}

// --- Invoker.Execute tests ---

func TestInvokerExecute_ReturnsInvocationResult(t *testing.T) {
	// Tests that Execute returns a properly populated InvocationResult
	// with Claude result data, model name, and provider name.
	mp := &mockProvider{
		name: "test-claude",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  "build complete",
				Model:   "claude-sonnet",
			}, nil
		},
	}

	selectCount := 0
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			selectCount++
			return mp, "claude-sonnet"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	result, err := invoker.Execute(context.Background(), bc, "test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify InvocationResult contains expected data
	if result.ModelName != "claude-sonnet" {
		t.Errorf("ModelName = %q, want %q", result.ModelName, "claude-sonnet")
	}
	if result.ProviderName != "test-claude" {
		t.Errorf("ProviderName = %q, want %q", result.ProviderName, "test-claude")
	}
	if !result.Result.Success {
		t.Error("Result.Success = false, want true")
	}
	if result.Result.Output != "build complete" {
		t.Errorf("Result.Output = %q, want %q", result.Result.Output, "build complete")
	}
	if result.StallFired {
		t.Error("StallFired = true, want false for successful invocation")
	}
	if selectCount != 1 {
		t.Errorf("router.Select called %d times, want 1", selectCount)
	}
}

func TestInvokerExecute_PropagatesModelAndProviderToBeadContext(t *testing.T) {
	// Tests that Execute updates bc.Model, bc.Result.Model, and bc.BuildProvider
	// with the router-selected values.
	mp := &mockProvider{name: "anthropic"}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "opus-4"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	bc.Tier = provider.TierHigh

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bc.Model != "opus-4" {
		t.Errorf("bc.Model = %q, want %q", bc.Model, "opus-4")
	}
	if bc.Result.Model != "opus-4" {
		t.Errorf("bc.Result.Model = %q, want %q", bc.Result.Model, "opus-4")
	}
	if bc.BuildProvider != "anthropic" {
		t.Errorf("bc.BuildProvider = %q, want %q", bc.BuildProvider, "anthropic")
	}
}

func TestInvokerExecute_NilRouterReturnsError(t *testing.T) {
	invoker := NewInvoker(nil, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected error for nil router, got nil")
	}
}

func TestInvokerExecute_NoProviderAvailableReturnsError(t *testing.T) {
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return nil, ""
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected error when no providers available, got nil")
	}
}

func TestInvokerExecute_UsageLimitTriggersProviderFallback(t *testing.T) {
	// When the primary provider returns a usage limit error, Execute should
	// mark it unavailable and retry with a fallback provider.
	primaryCalled := false
	fallbackCalled := false

	primary := &mockProvider{
		name: "provider-a",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			primaryCalled = true
			return nil, fmt.Errorf("usage limit exceeded")
		},
		isUsageLimitFn: func(result *provider.Result, err error) bool {
			return true
		},
	}

	fallback := &mockProvider{
		name: "provider-b",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			fallbackCalled = true
			return &provider.Result{Success: true, Model: "fallback-model"}, nil
		},
	}

	callCount := 0
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			callCount++
			if callCount == 1 {
				return primary, "primary-model"
			}
			return fallback, "fallback-model"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	result, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error after fallback: %v", err)
	}

	if !primaryCalled {
		t.Error("primary provider should have been called")
	}
	if !fallbackCalled {
		t.Error("fallback provider should have been called")
	}
	if len(mr.markCalls) != 1 || mr.markCalls[0] != "provider-a" {
		t.Errorf("MarkUnavailable calls = %v, want [provider-a]", mr.markCalls)
	}
	if result.ModelName != "fallback-model" {
		t.Errorf("ModelName = %q, want %q after fallback", result.ModelName, "fallback-model")
	}
	if len(mr.recordCalls) != 1 {
		t.Fatalf("RecordOutcome call count = %d, want 1", len(mr.recordCalls))
	}
	if mr.recordCalls[0].providerName != "provider-b" {
		t.Fatalf("RecordOutcome provider = %q, want %q", mr.recordCalls[0].providerName, "provider-b")
	}
	if mr.recordCalls[0].failureCategory != "" {
		t.Fatalf("RecordOutcome failure category = %q, want empty", mr.recordCalls[0].failureCategory)
	}
}

func TestInvokerExecute_CacheLookupHitUsesCachedPromptBeforeInvocation(t *testing.T) {
	callOrder := []string{}
	var promptUsed string
	cache := &mockCacheAdapter{
		lookupFn: func(ctx context.Context, req provider.CacheLookupRequest) (*provider.CacheEntry, bool, error) {
			callOrder = append(callOrder, "lookup")
			return &provider.CacheEntry{Content: "cached prompt"}, true, nil
		},
	}
	mp := &mockProvider{
		name:         "provider-cache",
		cacheAdapter: cache,
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			callOrder = append(callOrder, "stream")
			promptUsed = prompt
			return &provider.Result{Success: true, Model: "model-cache"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "model-cache"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	bc.PromptCtx = &prompt.Context{
		StaticPreambleCacheClass: "render_static_build",
		StaticPreambleCacheKey:   "cache-key-1",
	}

	_, err := invoker.Execute(context.Background(), bc, "original prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if promptUsed != "cached prompt" {
		t.Fatalf("provider prompt = %q, want %q", promptUsed, "cached prompt")
	}
	if len(callOrder) != 2 || callOrder[0] != "lookup" || callOrder[1] != "stream" {
		t.Fatalf("call order = %v, want [lookup stream]", callOrder)
	}
}

func TestInvokerExecute_Integration_CacheKeyStableAcrossBuildContextIterations(t *testing.T) {
	root := t.TempDir()
	templatesDir := filepath.Join(root, "templates")
	specsDir := filepath.Join(root, "specs")
	gromitDir := filepath.Join(root, ".gromit")
	claudePath := filepath.Join(root, "CLAUDE.md")

	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("mkdir gromit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_build.md"), []byte("{{ .Rules }}\n{{ .Spec }}"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("rules"), 0o644); err != nil {
		t.Fatalf("write RULES.md: %v", err)
	}
	if err := os.WriteFile(claudePath, []byte("claude"), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, claudePath, gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}

	b := &bead.Bead{ID: "b1", Title: "task"}
	ctx1, err := renderer.BuildContext(b, nil, 1, "model-a", "build")
	if err != nil {
		t.Fatalf("BuildContext(iteration=1) error = %v", err)
	}
	if _, err := renderer.RenderBuild(ctx1); err != nil {
		t.Fatalf("RenderBuild(iteration=1) error = %v", err)
	}

	ctx2, err := renderer.BuildContext(b, nil, 2, "model-b", "build")
	if err != nil {
		t.Fatalf("BuildContext(iteration=2) error = %v", err)
	}
	if _, err := renderer.RenderBuild(ctx2); err != nil {
		t.Fatalf("RenderBuild(iteration=2) error = %v", err)
	}

	lookupKeys := []string{}
	cache := &mockCacheAdapter{
		lookupFn: func(ctx context.Context, req provider.CacheLookupRequest) (*provider.CacheEntry, bool, error) {
			lookupKeys = append(lookupKeys, req.CacheKey)
			return nil, false, nil
		},
	}
	mp := &mockProvider{
		name:         "provider-cache",
		cacheAdapter: cache,
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "model-cache"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "model-cache"
		},
	}
	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)

	bc1 := newTestBeadContext()
	bc1.PromptCtx = ctx1
	if _, err := invoker.Execute(context.Background(), bc1, "prompt"); err != nil {
		t.Fatalf("Execute(iteration=1) error = %v", err)
	}

	bc2 := newTestBeadContext()
	bc2.PromptCtx = ctx2
	if _, err := invoker.Execute(context.Background(), bc2, "prompt"); err != nil {
		t.Fatalf("Execute(iteration=2) error = %v", err)
	}

	if len(lookupKeys) != 2 {
		t.Fatalf("lookup calls = %d, want 2", len(lookupKeys))
	}
	if lookupKeys[0] != ctx1.StaticPreambleCacheKey {
		t.Fatalf("lookup key for iteration=1 = %q, want %q", lookupKeys[0], ctx1.StaticPreambleCacheKey)
	}
	if lookupKeys[1] != ctx2.StaticPreambleCacheKey {
		t.Fatalf("lookup key for iteration=2 = %q, want %q", lookupKeys[1], ctx2.StaticPreambleCacheKey)
	}
	if lookupKeys[0] != lookupKeys[1] {
		t.Fatalf("lookup keys differ across iterations: %q vs %q", lookupKeys[0], lookupKeys[1])
	}
}

func TestInvokerExecute_CacheMissWritesPromptAfterInvocation(t *testing.T) {
	callOrder := []string{}
	var writeReq provider.CacheWriteRequest
	cache := &mockCacheAdapter{
		lookupFn: func(ctx context.Context, req provider.CacheLookupRequest) (*provider.CacheEntry, bool, error) {
			callOrder = append(callOrder, "lookup")
			return nil, false, nil
		},
		writeFn: func(ctx context.Context, req provider.CacheWriteRequest) error {
			callOrder = append(callOrder, "write")
			writeReq = req
			return nil
		},
	}
	mp := &mockProvider{
		name:         "provider-cache",
		cacheAdapter: cache,
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			callOrder = append(callOrder, "stream")
			return &provider.Result{Success: true, Model: "model-cache"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "model-cache"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	bc.PromptCtx = &prompt.Context{
		StaticPreambleCacheClass: "render_static_build",
		StaticPreambleCacheKey:   "cache-key-2",
	}

	_, err := invoker.Execute(context.Background(), bc, "original prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writeReq.CacheClass != "render_static_build" {
		t.Fatalf("write cache class = %q, want %q", writeReq.CacheClass, "render_static_build")
	}
	if writeReq.CacheKey != "cache-key-2" {
		t.Fatalf("write cache key = %q, want %q", writeReq.CacheKey, "cache-key-2")
	}
	if writeReq.Content != "test prompt" {
		t.Fatalf("write content = %q, want %q", writeReq.Content, "test prompt")
	}
	if len(callOrder) != 3 || callOrder[0] != "lookup" || callOrder[1] != "stream" || callOrder[2] != "write" {
		t.Fatalf("call order = %v, want [lookup stream write]", callOrder)
	}
}

func TestInvokerExecute_VersionKeyChangeInvalidatesBeforeNextLookup(t *testing.T) {
	callOrder := []string{}
	cache := &mockCacheAdapter{
		lookupFn: func(ctx context.Context, req provider.CacheLookupRequest) (*provider.CacheEntry, bool, error) {
			callOrder = append(callOrder, "lookup")
			return nil, false, nil
		},
		writeFn: func(ctx context.Context, req provider.CacheWriteRequest) error {
			callOrder = append(callOrder, "write")
			return nil
		},
		invalidateFn: func(ctx context.Context, req provider.CacheInvalidateRequest) error {
			callOrder = append(callOrder, "invalidate")
			return nil
		},
	}
	mp := &mockProvider{
		name:         "provider-cache",
		cacheAdapter: cache,
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			callOrder = append(callOrder, "stream")
			return &provider.Result{Success: true, Model: "model-cache"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "model-cache"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil).WithCacheVersionKey("rules:v1")
	bc := newTestBeadContext()
	bc.PromptCtx = &prompt.Context{
		StaticPreambleCacheClass: "render_static_build",
		StaticPreambleCacheKey:   "cache-key-3",
	}

	if _, err := invoker.Execute(context.Background(), bc, "original prompt"); err != nil {
		t.Fatalf("first execute error: %v", err)
	}

	invoker.WithCacheVersionKey("rules:v2")
	if _, err := invoker.Execute(context.Background(), bc, "original prompt"); err != nil {
		t.Fatalf("second execute error: %v", err)
	}

	got := strings.Join(callOrder, ",")
	want := "lookup,stream,write,invalidate,lookup,stream,write"
	if got != want {
		t.Fatalf("call order = %q, want %q", got, want)
	}
}

func TestInvokerExecute_CacheLookupErrorContinuesUncachedWithoutWrite(t *testing.T) {
	callOrder := []string{}
	var promptUsed string
	writeCalls := 0
	cache := &mockCacheAdapter{
		lookupFn: func(ctx context.Context, req provider.CacheLookupRequest) (*provider.CacheEntry, bool, error) {
			callOrder = append(callOrder, "lookup")
			return nil, false, fmt.Errorf("cache backend unavailable")
		},
		writeFn: func(ctx context.Context, req provider.CacheWriteRequest) error {
			writeCalls++
			callOrder = append(callOrder, "write")
			return nil
		},
	}
	mp := &mockProvider{
		name:         "provider-cache",
		cacheAdapter: cache,
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			callOrder = append(callOrder, "stream")
			promptUsed = prompt
			return &provider.Result{Success: true, Model: "model-cache"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "model-cache"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	bc.PromptCtx = &prompt.Context{
		StaticPreambleCacheClass: "render_static_build",
		StaticPreambleCacheKey:   "cache-key-4",
	}

	if _, err := invoker.Execute(context.Background(), bc, "original prompt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if promptUsed != "test prompt" {
		t.Fatalf("provider prompt = %q, want %q", promptUsed, "test prompt")
	}
	if writeCalls != 0 {
		t.Fatalf("write calls = %d, want 0 when lookup fails", writeCalls)
	}
	got := strings.Join(callOrder, ",")
	want := "lookup,stream"
	if got != want {
		t.Fatalf("call order = %q, want %q", got, want)
	}
}

func TestInvokerExecute_Integration_CacheLookupFailureFallsBackWithoutAbort(t *testing.T) {
	cacheClass := "render_static_build"
	cacheKey := prompt.StaticPreambleCacheKey(cacheClass, map[string]string{
		"rules": "rules",
		"spec":  "spec",
	})

	streamCalled := false
	writeCalls := 0
	cache := &mockCacheAdapter{
		lookupFn: func(ctx context.Context, req provider.CacheLookupRequest) (*provider.CacheEntry, bool, error) {
			if req.CacheClass != cacheClass || req.CacheKey != cacheKey {
				t.Fatalf("lookup request = (%q,%q), want (%q,%q)", req.CacheClass, req.CacheKey, cacheClass, cacheKey)
			}
			return nil, false, fmt.Errorf("cache backend unavailable")
		},
		writeFn: func(ctx context.Context, req provider.CacheWriteRequest) error {
			writeCalls++
			return nil
		},
	}
	mp := &mockProvider{
		name:         "provider-cache",
		cacheAdapter: cache,
		streamRunFn: func(ctx context.Context, promptText, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			streamCalled = true
			return &provider.Result{Success: true, Model: "model-cache"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "model-cache"
		},
	}
	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)

	bc := newTestBeadContext()
	bc.PromptCtx = &prompt.Context{
		StaticPreambleCacheClass: cacheClass,
		StaticPreambleCacheKey:   cacheKey,
	}

	if _, err := invoker.Execute(context.Background(), bc, "prompt"); err != nil {
		t.Fatalf("Execute() error = %v, want nil when cache lookup fails", err)
	}
	if !streamCalled {
		t.Fatal("expected provider StreamRun to execute after cache lookup failure")
	}
	if writeCalls != 0 {
		t.Fatalf("cache write calls = %d, want 0 when lookup fails", writeCalls)
	}
}

func TestInvokerExecute_EscalatedInvocationUpdatesEscalatedTo(t *testing.T) {
	// When bc.Result.Escalated is true and EscalatedTo is set, Execute
	// should update EscalatedTo with the concrete model name from the router.
	mp := &mockProvider{name: "provider-x"}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "opus-latest"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	bc.Result.Escalated = true
	bc.Result.EscalatedTo = "placeholder" // will be overwritten

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bc.Result.EscalatedTo != "opus-latest" {
		t.Errorf("bc.Result.EscalatedTo = %q, want %q", bc.Result.EscalatedTo, "opus-latest")
	}
}

func TestInvokerExecute_CapturesDiagnosticDataFromStreamStats(t *testing.T) {
	// Execute should populate bc.Result diagnostic fields from the StreamStats
	// DiagnosticSnapshot after the invocation completes.
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "test-model"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "test-model"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After execution, diagnostic fields should be populated (at minimum, set to zero values
	// since no events were recorded). The key behavior is that DiagnosticSnapshot is called
	// and its values are propagated to bc.Result.
	// StallCount should be 0 since no stall occurred
	if bc.Result.StallCount != 0 {
		t.Errorf("bc.Result.StallCount = %d, want 0", bc.Result.StallCount)
	}
	// ToolCallCount should be 0 since no tool calls were made
	if bc.Result.ToolCallCount != 0 {
		t.Errorf("bc.Result.ToolCallCount = %d, want 0", bc.Result.ToolCallCount)
	}
}

func TestInvokerExecute_PassesTierFromBeadContext(t *testing.T) {
	// Execute should use bc.Tier when calling router.Select and pass it
	// through to provider.StreamRun.
	var capturedRouterTier string
	var capturedStreamTier string

	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			capturedStreamTier = tier
			return &provider.Result{Success: true, Model: "m"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			capturedRouterTier = tier
			return mp, "m"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	bc.Tier = provider.TierHigh

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedRouterTier != provider.TierHigh {
		t.Errorf("router received tier %q, want %q", capturedRouterTier, provider.TierHigh)
	}
	if capturedStreamTier != provider.TierHigh {
		t.Errorf("StreamRun received tier %q, want %q", capturedStreamTier, provider.TierHigh)
	}
}

func TestInvokerExecute_StreamRunErrorPropagates(t *testing.T) {
	// When StreamRun returns a non-usage-limit error, it should propagate to the caller.
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return nil, fmt.Errorf("connection refused")
		},
		isUsageLimitFn: func(result *provider.Result, err error) bool {
			return false // not a usage limit error
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "m"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected error from StreamRun failure, got nil")
	}
}

func TestInvokerExecute_PassesEventHandlerWithoutStreamLogger(t *testing.T) {
	// Even when stream logger is nil, invoker should still pass a non-nil event
	// handler so providers can run in structured streaming mode.
	var handlerWasNil bool
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			handlerWasNil = handler == nil
			if handler != nil {
				handler([]byte(`{"type":"system","subtype":"init"}`))
			}
			return &provider.Result{Success: true, Model: "m"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "m"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	invResult, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invResult == nil {
		t.Fatal("expected invocation result")
	}
	if handlerWasNil {
		t.Fatal("expected non-nil event handler when stream logger is nil")
	}
}

func TestInvokerExecute_PreserveProviderStreamModePassesNilHandlers(t *testing.T) {
	t.Setenv("GROMIT_PRESERVE_PROVIDER_STREAM", "1")

	var handlerWasNil bool
	var toolHandlerWasNil bool
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			handlerWasNil = handler == nil
			toolHandlerWasNil = onToolCall == nil
			return &provider.Result{Success: true, Model: "m"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "m"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	invResult, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invResult == nil {
		t.Fatal("expected invocation result")
	}
	if !handlerWasNil {
		t.Fatal("expected nil event handler in preserve-provider-stream mode")
	}
	if !toolHandlerWasNil {
		t.Fatal("expected nil tool handler in preserve-provider-stream mode")
	}
}

func TestInvokerExecute_PreserveProviderStreamConfiguredPassesNilHandlers(t *testing.T) {
	var handlerWasNil bool
	var toolHandlerWasNil bool
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			handlerWasNil = handler == nil
			toolHandlerWasNil = onToolCall == nil
			return &provider.Result{Success: true, Model: "m"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "m"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil).WithPreserveProviderTerminalStream(true)
	bc := newTestBeadContext()
	invResult, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invResult == nil {
		t.Fatal("expected invocation result")
	}
	if !handlerWasNil {
		t.Fatal("expected nil event handler when preserve-provider-stream is configured")
	}
	if !toolHandlerWasNil {
		t.Fatal("expected nil tool handler when preserve-provider-stream is configured")
	}
}

func TestInvokerExecute_PreserveProviderStreamEnvOverrideOff(t *testing.T) {
	t.Setenv("GROMIT_PRESERVE_PROVIDER_STREAM", "0")

	var handlerWasNil bool
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			handlerWasNil = handler == nil
			return &provider.Result{Success: true, Model: "m"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "m"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil).WithPreserveProviderTerminalStream(true)
	bc := newTestBeadContext()
	invResult, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invResult == nil {
		t.Fatal("expected invocation result")
	}
	if handlerWasNil {
		t.Fatal("expected non-nil event handler when env override disables preserve mode")
	}
}

func TestInvokerExecute_EmitsStartMarker(t *testing.T) {
	logsDir := t.TempDir()
	sl, err := logger.NewStreamLogger(logsDir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}

	mp := &mockProvider{
		name: "provider-start",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "model-start"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "model-start"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, sl)
	bc := newTestBeadContext()

	_, err = invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := readStreamLogLines(t, sl)
	startIndex := lineIndex(lines, InvocationLifecycleMarkerStart)
	if startIndex == -1 {
		t.Fatalf("missing start marker %q", InvocationLifecycleMarkerStart)
	}
}

func TestInvokerExecute_EmitsLifecycleMarkersWithoutStreamEvents(t *testing.T) {
	// Ensure lifecycle markers are emitted even when no stream events are parsed.
	// This test expects start, selection, and completion markers in the stream log.
	logsDir := t.TempDir()
	sl, err := logger.NewStreamLogger(logsDir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}

	mp := &mockProvider{
		name: "provider-a",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			// Do not emit any events via handler.
			return &provider.Result{Success: true, Model: "model-a"}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "model-a"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, sl)
	bc := newTestBeadContext()
	bc.Tier = provider.TierHigh

	_, err = invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := readStreamLogLines(t, sl)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lifecycle lines, got %d", len(lines))
	}

	startIndex := lineIndex(lines, InvocationLifecycleMarkerStart)
	selectIndex := lineIndex(lines, InvocationLifecycleMarkerSelection)
	completeIndex := lineIndex(lines, InvocationLifecycleMarkerComplete)
	if startIndex == -1 {
		t.Fatalf("missing start marker %q", InvocationLifecycleMarkerStart)
	}
	if selectIndex == -1 {
		t.Fatalf("missing selection marker %q", InvocationLifecycleMarkerSelection)
	}
	if completeIndex == -1 {
		t.Fatalf("missing completion marker %q", InvocationLifecycleMarkerComplete)
	}
	if startIndex >= selectIndex || selectIndex >= completeIndex {
		t.Fatalf("expected marker order start < selection < completion, got %d < %d < %d", startIndex, selectIndex, completeIndex)
	}

	if !strings.Contains(lines[selectIndex], "provider=provider-a") {
		t.Fatalf("selection marker missing provider: %s", lines[selectIndex])
	}
	if !strings.Contains(lines[selectIndex], "model=model-a") {
		t.Fatalf("selection marker missing model: %s", lines[selectIndex])
	}
	if !strings.Contains(lines[selectIndex], "tier="+provider.TierHigh) {
		t.Fatalf("selection marker missing tier: %s", lines[selectIndex])
	}
	if !strings.Contains(lines[completeIndex], "success=true") {
		t.Fatalf("completion marker missing success=true: %s", lines[completeIndex])
	}
}

func TestInvokerExecute_EmitsFailureSummaryMarker(t *testing.T) {
	logsDir := t.TempDir()
	sl, err := logger.NewStreamLogger(logsDir)
	if err != nil {
		t.Fatalf("creating stream logger: %v", err)
	}

	mp := &mockProvider{
		name: "provider-b",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return nil, fmt.Errorf("connection refused")
		},
		isUsageLimitFn: func(result *provider.Result, err error) bool {
			return false
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "model-b"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, sl)
	bc := newTestBeadContext()

	_, err = invoker.Execute(context.Background(), bc, "prompt")
	if err == nil {
		t.Fatal("expected error from StreamRun")
	}

	lines := readStreamLogLines(t, sl)
	failureIndex := lineIndex(lines, InvocationLifecycleMarkerFailure)
	if failureIndex == -1 {
		t.Fatalf("missing failure marker %q", InvocationLifecycleMarkerFailure)
	}
	if !strings.Contains(lines[failureIndex], "error=connection refused") {
		t.Fatalf("failure marker missing error summary: %s", lines[failureIndex])
	}
}

func TestInvocationResult_ContainsStreamStats(t *testing.T) {
	// InvocationResult should include StreamStats for the caller to inspect.
	mp := &mockProvider{}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "m"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	result, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Stats == nil {
		t.Fatal("InvocationResult.Stats should not be nil")
	}

	// Stats should be a real StreamStats with valid data
	toolCalls, _, elapsed := result.Stats.Snapshot()
	if toolCalls != 0 {
		t.Errorf("Stats.ToolCalls = %d, want 0 for no-op invocation", toolCalls)
	}
	if elapsed < 0 {
		t.Error("Stats.Elapsed should be non-negative")
	}
}

func TestInvokerExecute_ExposesProviderResult(t *testing.T) {
	expected := &provider.Result{
		Success:      true,
		Output:       "provider output",
		ExitCode:     7,
		Model:        "test-model",
		CostUSD:      2.34,
		InputTokens:  11,
		OutputTokens: 22,
	}
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return expected, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "test-model"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	result, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ProviderResult == nil {
		t.Fatal("InvocationResult.ProviderResult should not be nil")
	}
	if result.ProviderResult != expected {
		t.Fatalf("InvocationResult.ProviderResult = %+v, want %+v", result.ProviderResult, expected)
	}
}

func TestInvokerExecute_RecordsFailureCategoryWithRouter(t *testing.T) {
	mp := &mockProvider{
		name: "provider-z",
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success:         false,
				FailureCategory: provider.FailureCategoryTransportDisconnect,
			}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "model-z"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	_, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mr.recordCalls) != 1 {
		t.Fatalf("RecordOutcome call count = %d, want 1", len(mr.recordCalls))
	}
	if mr.recordCalls[0].providerName != "provider-z" {
		t.Fatalf("RecordOutcome provider = %q, want %q", mr.recordCalls[0].providerName, "provider-z")
	}
	if mr.recordCalls[0].failureCategory != provider.FailureCategoryTransportDisconnect {
		t.Fatalf("RecordOutcome failure category = %q, want %q", mr.recordCalls[0].failureCategory, provider.FailureCategoryTransportDisconnect)
	}
}

func TestInvokerExecute_MergesProviderUsageIntoStatsWhenStreamHasNoResultEvent(t *testing.T) {
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			// Emit a non-result event so stream stats record activity but no usage.
			if handler != nil {
				handler([]byte(`{"type":"system","subtype":"init"}`))
			}
			return &provider.Result{
				Success:      true,
				Model:        "test-model",
				CostUSD:      1.23,
				InputTokens:  4321,
				OutputTokens: 210,
			}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "test-model"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()
	invResult, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invResult == nil || invResult.Stats == nil {
		t.Fatal("expected invocation stats")
	}

	cost, in, out := invResult.Stats.CostData()
	if cost != 1.23 {
		t.Errorf("CostData cost = %v, want 1.23", cost)
	}
	if in != 4321 {
		t.Errorf("CostData input tokens = %d, want 4321", in)
	}
	if out != 210 {
		t.Errorf("CostData output tokens = %d, want 210", out)
	}
}

func TestInvocationResult_ConvertsClaudeResult(t *testing.T) {
	// The Result field in InvocationResult should be a *claude.Result,
	// converted from the provider.Result returned by StreamRun.
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			return &provider.Result{
				Success:  true,
				Output:   "output text",
				ExitCode: 0,
				Duration: 5 * time.Second,
				Model:    "opus-4",
			}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "opus-4"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil)
	bc := newTestBeadContext()

	invResult, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cr := invResult.Result
	if cr == nil {
		t.Fatal("InvocationResult.Result should not be nil")
	}
	if !cr.Success {
		t.Error("claude.Result.Success = false, want true")
	}
	if cr.Output != "output text" {
		t.Errorf("claude.Result.Output = %q, want %q", cr.Output, "output text")
	}
	if cr.ExitCode != 0 {
		t.Errorf("claude.Result.ExitCode = %d, want 0", cr.ExitCode)
	}
	if cr.Duration != 5*time.Second {
		t.Errorf("claude.Result.Duration = %v, want 5s", cr.Duration)
	}
}

// TestInvokerExecute_PreserveStreamModeStillPropagatesCostFromProviderResult verifies
// that when preserve_provider_stream mode is active (nil event handler), cost data
// reported by the provider result flows through MergeCostData into invResult.Stats.
// This is the regression test for the haiku $0 cost bug: in preserve mode the stream
// event handler is nil so ParseAndLogEvent never fires, meaning cost must come
// solely from providerResult.CostUSD via MergeCostData.
func TestInvokerExecute_PreserveStreamModeStillPropagatesCostFromProviderResult(t *testing.T) {
	mp := &mockProvider{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			// In preserve mode the handler is nil — no stream events are parsed.
			// Cost must come solely from the returned provider.Result.
			if handler != nil {
				t.Error("expected nil event handler in preserve-stream mode")
			}
			return &provider.Result{
				Success:      true,
				Model:        "claude-haiku-4-5",
				CostUSD:      0.0432,
				InputTokens:  8500,
				OutputTokens: 420,
			}, nil
		},
	}
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return mp, "claude-haiku-4-5"
		},
	}

	invoker := NewInvoker(mr, &bytes.Buffer{}, nil).WithPreserveProviderTerminalStream(true)
	bc := newTestBeadContext()
	invResult, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invResult == nil || invResult.Stats == nil {
		t.Fatal("expected non-nil invocation result with stats")
	}

	cost, inputTokens, outputTokens := invResult.Stats.CostData()
	if cost != 0.0432 {
		t.Errorf("CostData cost = %v, want 0.0432 (preserve mode must propagate provider result cost via MergeCostData)", cost)
	}
	if inputTokens != 8500 {
		t.Errorf("CostData input tokens = %d, want 8500", inputTokens)
	}
	if outputTokens != 420 {
		t.Errorf("CostData output tokens = %d, want 420", outputTokens)
	}
}

func TestNewInvoker_AcceptsNarrowInterfaces(t *testing.T) {
	// Verify that NewInvoker accepts the narrow Router interface (not *provider.Router),
	// enabling mock injection without importing the provider package's concrete types.
	var buf bytes.Buffer
	mr := &mockRouter{
		selectFn: func(phase, tier string) (Provider, string) {
			return &mockProvider{}, "m"
		},
	}

	// This must compile with the narrow Router interface, not *provider.Router
	invoker := NewInvoker(mr, &buf, nil)
	bc := newTestBeadContext()

	result, err := invoker.Execute(context.Background(), bc, "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}
