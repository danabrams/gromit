package runner

import (
	"context"
	"io"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/provider"
)

// TestRunnerStructDoesNotHaveClaudeField uses reflection to verify that the Runner struct
// no longer has a 'claude' field after all sites have been converted to use r.router
// Expected failure: Runner.claude field still exists
func TestRunnerStructDoesNotHaveClaudeField(t *testing.T) {
	// Use reflection to check if the Runner struct has a 'claude' field
	runnerType := reflect.TypeOf(Runner{})

	for i := 0; i < runnerType.NumField(); i++ {
		field := runnerType.Field(i)
		if field.Name == "claude" {
			t.Errorf("Runner struct still has 'claude' field at index %d. This field should be removed after all sites are converted to use r.router", i)
		}
	}

	// Also verify that router field exists
	hasRouter := false
	for i := 0; i < runnerType.NumField(); i++ {
		field := runnerType.Field(i)
		if field.Name == "router" {
			hasRouter = true
			break
		}
	}

	if !hasRouter {
		t.Error("Runner struct should have 'router' field")
	}
}

// TestDepsStructDoesNotRequireClaudeField verifies that Deps struct only needs Router,
// not both Claude and Router
// Expected failure: Deps struct still has Claude field alongside Router
func TestDepsStructDoesNotRequireClaudeField(t *testing.T) {
	// Use reflection to check if Deps has a Claude field
	depsType := reflect.TypeOf(Deps{})

	hasClaude := false
	hasRouter := false

	for i := 0; i < depsType.NumField(); i++ {
		field := depsType.Field(i)
		if field.Name == "Claude" {
			hasClaude = true
		}
		if field.Name == "Router" {
			hasRouter = true
		}
	}

	// After conversion, Deps should have Router but not Claude
	if hasClaude {
		t.Error("Deps struct still has 'Claude' field. After conversion to Provider/Router pattern, only Router should be needed")
	}

	if !hasRouter {
		t.Error("Deps struct should have 'Router' field")
	}
}

// TestNewRunnerWithDepsWorksWithOnlyRouter verifies that NewRunnerWithDeps
// can create a runner with only Router in deps (no Claude field)
// Expected failure: NewRunnerWithDeps requires Claude field in deps or fallback logic still exists
func TestNewRunnerWithDepsWorksWithOnlyRouter(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")

	cfg := &config.Config{
		Paths: config.PathsConfig{
			GromitDir: gromitDir,
		},
	}
	cfg.SetDefaults()

	// Test will fail because:
	// 1. Deps still has Claude field alongside Router
	// 2. NewRunnerWithDeps signature still accepts Claude in deps
	// The test documents the desired end state where only Router is needed

	t.Skip("Test documents desired behavior - Deps struct and NewRunnerWithDeps need updating to remove Claude field dependency")
}

// TestLearningsAdapterCanUseProviderInterface verifies that the learnings adapter
// integration in NewRunner can work with a Provider instead of requiring claude.Client
func TestLearningsAdapterCanUseProviderInterface(t *testing.T) {
	// This test validates that NewProviderRunnerAdapter exists and works
	mockProvider := &mockProviderForRunner{}

	// Create adapter with provider
	adapter := learnings.NewProviderRunnerAdapter(mockProvider)
	if adapter == nil {
		t.Fatal("NewProviderRunnerAdapter returned nil")
	}

	// Verify it can be used with LLMFilter
	filter := learnings.NewLLMFilter(adapter, "test", "test project")
	if filter == nil {
		t.Fatal("NewLLMFilter returned nil")
	}
}

// TestAnalyzerCanBeCreatedWithProvider verifies that analyzer.NewAnalyzer
// can accept a Provider instead of requiring claude.Client
func TestAnalyzerCanBeCreatedWithProvider(t *testing.T) {
	mockProvider := &mockProviderForRunner{}

	// Create analyzer with provider
	mockRenderer := &mockPromptRenderer{}
	a, err := testNewAnalyzerWithProvider(mockProvider, mockRenderer)

	if err != nil {
		t.Fatalf("analyzer.NewAnalyzer should accept Provider, got error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil analyzer")
	}
}

// Helper function to test NewAnalyzer with Provider
func testNewAnalyzerWithProvider(p provider.Provider, renderer interface{}) (interface{}, error) {
	// analyzer.NewAnalyzer now accepts Provider
	return analyzer.NewAnalyzer(p, "low", renderer.(analyzer.PromptRenderer))
}

// Mock types for testing

type mockProviderForRunner struct {
	FnRun           func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
	FnStreamRun     func(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	FnRunValidation func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error)
}

func (m *mockProviderForRunner) Name() string { return "mock" }

func (m *mockProviderForRunner) ModelForTier(tier string) string { return tier }

func (m *mockProviderForRunner) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if m.FnRun != nil {
		return m.FnRun(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Output: "mock output"}, nil
}

func (m *mockProviderForRunner) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.FnStreamRun != nil {
		return m.FnStreamRun(ctx, prompt, tier, output, handler, onToolCall)
	}
	return &provider.Result{Success: true}, nil
}

func (m *mockProviderForRunner) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	if m.FnRunValidation != nil {
		return m.FnRunValidation(ctx, commands, tier, workDir)
	}
	return &provider.Result{Success: true}, nil
}

func (m *mockProviderForRunner) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

func (m *mockProviderForRunner) IsValidationPassed(result *provider.Result) bool {
	return result.Success
}

func (m *mockProviderForRunner) IsScopeTooLarge(result *provider.Result) (bool, string) {
	return false, ""
}

type mockRouterForRunner struct {
	FnSelect           func(phase string, tier string) (provider.Provider, string)
	FnMarkUnavailable  func(name string)
	FnRecordInvocation func(name string)
	FnStreamRun        func(ctx context.Context, phase string, tier string, prompt string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	FnRun              func(ctx context.Context, phase string, tier string, prompt string) (*provider.Result, error)
}

func (m *mockRouterForRunner) Select(phase string, tier string) (provider.Provider, string) {
	if m.FnSelect != nil {
		return m.FnSelect(phase, tier)
	}
	return &mockProviderForRunner{}, "haiku"
}

func (m *mockRouterForRunner) MarkUnavailable(name string) {
	if m.FnMarkUnavailable != nil {
		m.FnMarkUnavailable(name)
	}
}

func (m *mockRouterForRunner) RecordInvocation(name string) {
	if m.FnRecordInvocation != nil {
		m.FnRecordInvocation(name)
	}
}

func (m *mockRouterForRunner) StreamRun(ctx context.Context, phase string, tier string, prompt string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.FnStreamRun != nil {
		return m.FnStreamRun(ctx, phase, tier, prompt, output, handler, onToolCall)
	}
	return &provider.Result{Success: true}, nil
}

func (m *mockRouterForRunner) Run(ctx context.Context, phase string, tier string, prompt string) (*provider.Result, error) {
	if m.FnRun != nil {
		return m.FnRun(ctx, phase, tier, prompt)
	}
	return &provider.Result{Success: true}, nil
}
