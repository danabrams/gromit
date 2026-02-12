//go:build acceptance

package analyzer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// TestNewAnalyzerAcceptsProvider verifies that NewAnalyzer can accept a Provider
// instead of only claude.Client
// Expected failure: NewAnalyzer does not accept Provider parameter yet
func TestNewAnalyzerAcceptsProvider(t *testing.T) {
	mockProvider := &mockProvider{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return &provider.Result{Success: true, Output: `{"category":"syntax","recoverable":true,"root_cause":"test"}`}, nil
		},
	}
	mockRenderer := &mockRenderer{}

	analyzer, err := NewAnalyzer(mockProvider, "haiku", mockRenderer)

	if err != nil {
		t.Fatalf("NewAnalyzer with provider should succeed, got error: %v", err)
	}
	if analyzer == nil {
		t.Fatal("NewAnalyzer should return non-nil analyzer")
	}
}

// TestAnalyzerAnalyzeWithProvider verifies that Analyzer.Analyze works correctly
// when initialized with a Provider instead of claude.Client
// Expected failure: Analyzer.Analyze does not work with Provider yet
func TestAnalyzerAnalyzeWithProvider(t *testing.T) {
	mockProvider := &mockProvider{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success:  true,
				Output:   `{"category":"logic","recoverable":true,"root_cause":"test root cause","suggestion":"test suggestion"}`,
				ExitCode: 0,
				Duration: time.Second,
				Model:    "haiku",
			}, nil
		},
	}
	mockRenderer := &mockRenderer{
		FnRenderAnalyze: func(ctx *prompt.AnalyzeContext) (string, error) {
			return "analyze prompt", nil
		},
	}

	analyzer, err := NewAnalyzer(mockProvider, "haiku", mockRenderer)
	if err != nil {
		t.Fatalf("NewAnalyzer failed: %v", err)
	}

	b := &bead.Bead{
		ID:    "test-123",
		Title: "Test bead",
	}

	analysis, err := analyzer.Analyze(context.Background(), b, "test failure output")

	if err != nil {
		t.Fatalf("Analyze should succeed with provider, got error: %v", err)
	}
	if analysis == nil {
		t.Fatal("expected non-nil analysis")
	}
	if analysis.Category != CategoryLogic {
		t.Errorf("expected category=logic, got %v", analysis.Category)
	}
	if analysis.RootCause != "test root cause" {
		t.Errorf("expected root_cause='test root cause', got %q", analysis.RootCause)
	}
}

// TestAnalyzerUsesProviderTierParameter verifies that Analyzer calls Provider.Run
// with the tier parameter (not a model name)
// Expected failure: Analyzer does not call Provider.Run with tier parameter yet
func TestAnalyzerUsesProviderTierParameter(t *testing.T) {
	var capturedTier string
	mockProvider := &mockProvider{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Output:  `{"category":"syntax","recoverable":true,"root_cause":"test","suggestion":"test"}`,
			}, nil
		},
	}
	mockRenderer := &mockRenderer{
		FnRenderAnalyze: func(ctx *prompt.AnalyzeContext) (string, error) {
			return "analyze prompt", nil
		},
	}

	analyzer, err := NewAnalyzer(mockProvider, "sonnet", mockRenderer)
	if err != nil {
		t.Fatalf("NewAnalyzer failed: %v", err)
	}

	b := &bead.Bead{ID: "test-123", Title: "Test"}
	_, err = analyzer.Analyze(context.Background(), b, "failure output")

	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if capturedTier != "sonnet" {
		t.Errorf("expected Provider.Run to be called with tier='sonnet', got %q", capturedTier)
	}
}

// TestNewAnalyzerRejectsNilProvider verifies that NewAnalyzer returns an error
// when given a nil Provider
// Expected failure: NewAnalyzer does not check for nil Provider yet
func TestNewAnalyzerRejectsNilProvider(t *testing.T) {
	mockRenderer := &mockRenderer{}

	analyzer, err := NewAnalyzer(nil, "haiku", mockRenderer)

	if err == nil {
		t.Fatal("NewAnalyzer should return error for nil provider")
	}
	if analyzer != nil {
		t.Error("NewAnalyzer should return nil analyzer when provider is nil")
	}
	if !strings.Contains(err.Error(), "provider") && !strings.Contains(err.Error(), "nil") {
		t.Errorf("error should mention provider or nil, got: %v", err)
	}
}

// TestAnalyzerAnalyzeWithProviderError verifies that Analyzer.Analyze properly
// propagates errors from Provider.Run
// Expected failure: Analyzer does not handle Provider errors correctly yet
func TestAnalyzerAnalyzeWithProviderError(t *testing.T) {
	mockProvider := &mockProvider{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return nil, errors.New("provider run failed")
		},
	}
	mockRenderer := &mockRenderer{
		FnRenderAnalyze: func(ctx *prompt.AnalyzeContext) (string, error) {
			return "analyze prompt", nil
		},
	}

	analyzer, err := NewAnalyzer(mockProvider, "haiku", mockRenderer)
	if err != nil {
		t.Fatalf("NewAnalyzer failed: %v", err)
	}

	b := &bead.Bead{ID: "test-123", Title: "Test"}
	analysis, err := analyzer.Analyze(context.Background(), b, "failure output")

	if err == nil {
		t.Fatal("Analyze should return error when Provider.Run fails")
	}
	if analysis != nil {
		t.Error("Analyze should return nil analysis on error")
	}
}

// mockProvider implements a mock Provider for testing
type mockProvider struct {
	FnRun            func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
	FnStreamRun      func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
	FnRunValidation  func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error)
	FnName           func() string
	FnModelForTier   func(tier string) string
	FnIsUsageLimit   func(result *provider.Result, err error) bool
}

func (m *mockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if m.FnRun != nil {
		return m.FnRun(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Output: "mock output"}, nil
}

func (m *mockProvider) StreamRun(ctx context.Context, prompt string, tier string, output interface{}, handler interface{}, onToolCall interface{}) (*provider.Result, error) {
	if m.FnStreamRun != nil {
		return m.FnStreamRun(ctx, prompt, tier)
	}
	return &provider.Result{Success: true}, nil
}

func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	if m.FnRunValidation != nil {
		return m.FnRunValidation(ctx, commands, tier, workDir)
	}
	return &provider.Result{Success: true}, nil
}

func (m *mockProvider) Name() string {
	if m.FnName != nil {
		return m.FnName()
	}
	return "mock"
}

func (m *mockProvider) ModelForTier(tier string) string {
	if m.FnModelForTier != nil {
		return m.FnModelForTier(tier)
	}
	return tier
}

func (m *mockProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	if m.FnIsUsageLimit != nil {
		return m.FnIsUsageLimit(result, err)
	}
	return false
}

// mockRenderer implements a mock PromptRenderer for testing
type mockRenderer struct {
	FnRenderAnalyze func(ctx *prompt.AnalyzeContext) (string, error)
}

func (m *mockRenderer) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
	return &prompt.Context{}, nil
}

func (m *mockRenderer) RenderBuild(ctx *prompt.Context) (string, error) {
	return "build prompt", nil
}

func (m *mockRenderer) RenderAnalyze(ctx *prompt.AnalyzeContext) (string, error) {
	if m.FnRenderAnalyze != nil {
		return m.FnRenderAnalyze(ctx)
	}
	return "analyze prompt", nil
}

func (m *mockRenderer) RenderLearn(ctx *prompt.LearnContext) (string, error) {
	return "learn prompt", nil
}

func (m *mockRenderer) RenderDecompose(ctx *prompt.DecomposeContext) (string, error) {
	return "decompose prompt", nil
}

func (m *mockRenderer) RenderScope(ctx *prompt.ScopeContext) (string, error) {
	return "scope prompt", nil
}

func (m *mockRenderer) RenderPrecheck(ctx *prompt.PrecheckContext) (string, error) {
	return "precheck prompt", nil
}

func (m *mockRenderer) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	return "review prompt", nil
}

func (m *mockRenderer) RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error) {
	return "thorough review prompt", nil
}

func (m *mockRenderer) RenderAcceptanceTests(ctx *prompt.Context) (string, error) {
	return "acceptance tests prompt", nil
}

func (m *mockRenderer) RenderATDDBuild(ctx *prompt.Context) (string, error) {
	return "atdd build prompt", nil
}

func (m *mockRenderer) RenderTDDBuild(ctx *prompt.Context) (string, error) {
	return "tdd build prompt", nil
}

func (m *mockRenderer) RenderRefactor(ctx *prompt.Context) (string, error) {
	return "refactor prompt", nil
}

func (m *mockRenderer) LoadSpec(name string) (string, error) {
	return "spec content", nil
}

func (m *mockRenderer) LoadClaudeMD() (string, error) {
	return "claude md content", nil
}

func (m *mockRenderer) LoadRules() (string, error) {
	return "rules content", nil
}

func (m *mockRenderer) GetLearningsFile() interface{} {
	return nil
}
