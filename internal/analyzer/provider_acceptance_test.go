//go:build acceptance

package analyzer

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// TestAnalyzerWithProvider verifies that Analyzer works with Provider interface,
// accepts tier parameter correctly, and properly parses analysis results.
func TestAnalyzerWithProvider(t *testing.T) {
	var capturedTier string
	mockProvider := &mockProvider{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			capturedTier = tier
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

	analyzer, err := NewAnalyzer(mockProvider, "medium", mockRenderer)
	if err != nil {
		t.Fatalf("NewAnalyzer failed: %v", err)
	}
	if analyzer == nil {
		t.Fatal("NewAnalyzer should return non-nil analyzer")
	}

	b := &bead.Bead{ID: "test-123", Title: "Test bead"}
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
	if capturedTier != "medium" {
		t.Errorf("expected Provider.Run to be called with tier='medium', got %q", capturedTier)
	}
}

func TestMockRendererBuildContextAcceptsPhase(t *testing.T) {
	m := &mockRenderer{}
	_, err := m.BuildContext(&bead.Bead{ID: "test-123"}, nil, 1, "haiku", "build")
	if err != nil {
		t.Fatalf("BuildContext should succeed, got error: %v", err)
	}
}

// mockProvider implements a mock Provider for testing
type mockProvider struct {
	FnRun           func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
	FnStreamRun     func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
	FnRunValidation func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error)
	FnName          func() string
	FnModelForTier  func(tier string) string
	FnIsUsageLimit  func(result *provider.Result, err error) bool
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

func (m *mockRenderer) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string, phase string) (*prompt.Context, error) {
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
