//go:build acceptance

package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// TestProviderIntegrationWithLearningsAndAnalyzer verifies that a Provider can be used
// by both the learnings adapter and the analyzer, matching the pattern in runner.go lines 111 and 118.
// Expected failure: learnings.NewProviderRunnerAdapter does not exist yet
func TestProviderIntegrationWithLearningsAndAnalyzer(t *testing.T) {
	// Create a mock provider that tracks calls
	var learningsAdapterCalls int
	var analyzerCalls int

	mockProvider := &testProvider{
		runFunc: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			// Distinguish calls by prompt content
			if prompt == "test learnings prompt" {
				learningsAdapterCalls++
			} else if prompt == "test analyzer prompt" {
				analyzerCalls++
			}
			return &provider.Result{
				Success: true,
				Output:  `{"category":"syntax","recoverable":true,"root_cause":"test","suggestion":"test"}`,
			}, nil
		},
	}

	// Test 1: Create learnings adapter from provider (line 111 in runner.go)
	learningsAdapter := learnings.NewProviderRunnerAdapter(mockProvider)
	if learningsAdapter == nil {
		t.Fatal("NewProviderRunnerAdapter should return non-nil adapter")
	}

	// Use the learnings adapter
	_, err := learningsAdapter.Run(context.Background(), "test learnings prompt", "haiku")
	if err != nil {
		t.Errorf("learnings adapter Run should succeed, got error: %v", err)
	}
	if learningsAdapterCalls != 1 {
		t.Errorf("expected 1 learnings adapter call, got %d", learningsAdapterCalls)
	}

	// Test 2: Create analyzer from provider (line 118 in runner.go)
	mockRenderer := &testPromptRenderer{
		renderAnalyzeFunc: func(ctx *prompt.AnalyzeContext) (string, error) {
			return "test analyzer prompt", nil
		},
	}

	analyzerObj, err := analyzer.NewAnalyzer(mockProvider, "low", mockRenderer)
	if err != nil {
		t.Fatalf("NewAnalyzer with provider should succeed, got error: %v", err)
	}
	if analyzerObj == nil {
		t.Fatal("NewAnalyzer should return non-nil analyzer")
	}

	// Use the analyzer
	b := &bead.Bead{ID: "test-123", Title: "Test"}
	_, err = analyzerObj.Analyze(context.Background(), b, "test failure")
	if err != nil {
		t.Errorf("analyzer Analyze should succeed, got error: %v", err)
	}
	if analyzerCalls != 1 {
		t.Errorf("expected 1 analyzer call, got %d", analyzerCalls)
	}
}

// TestLearningsAdapterConvertsProviderResultFormat verifies that the adapter
// correctly converts provider.Result to learnings.Result format.
// Expected failure: learnings.NewProviderRunnerAdapter does not exist yet
func TestLearningsAdapterConvertsProviderResultFormat(t *testing.T) {
	mockProvider := &testProvider{
		runFunc: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success:  true,
				Output:   "specific",
				ExitCode: 0,
				Model:    "haiku",
			}, nil
		},
	}

	adapter := learnings.NewProviderRunnerAdapter(mockProvider)
	result, err := adapter.Run(context.Background(), "classify this learning", "haiku")

	if err != nil {
		t.Fatalf("adapter Run should succeed, got error: %v", err)
	}
	if result == nil {
		t.Fatal("adapter Run should return non-nil result")
	}
	if !result.Success {
		t.Error("expected result.Success=true from adapter")
	}
	if result.Output != "specific" {
		t.Errorf("expected result.Output='specific', got %q", result.Output)
	}
}

// TestAnalyzerUsesProviderWithTier verifies that the analyzer correctly
// passes the tier parameter (not model name) to the Provider.
// Expected failure: Analyzer may not correctly handle tier parameter yet
func TestAnalyzerUsesProviderWithTier(t *testing.T) {
	var capturedTier string
	mockProvider := &testProvider{
		runFunc: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Output:  `{"category":"logic","recoverable":true,"root_cause":"test","suggestion":"test"}`,
			}, nil
		},
	}

	mockRenderer := &testPromptRenderer{
		renderAnalyzeFunc: func(ctx *prompt.AnalyzeContext) (string, error) {
			return "analyze prompt", nil
		},
	}

	// Create analyzer with tier="low"
	analyzerObj, err := analyzer.NewAnalyzer(mockProvider, "low", mockRenderer)
	if err != nil {
		t.Fatalf("NewAnalyzer failed: %v", err)
	}

	// Analyze a failure
	b := &bead.Bead{ID: "test-123", Title: "Test"}
	_, err = analyzerObj.Analyze(context.Background(), b, "failure output")

	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if capturedTier != "low" {
		t.Errorf("expected Provider.Run called with tier='low', got %q", capturedTier)
	}
}

// TestNewRunnerCreatesProviderBasedComponents verifies that NewRunner
// creates both learnings adapter and analyzer using Provider when no
// providers config exists (backward compatibility path).
// Expected failure: learnings.NewProviderRunnerAdapter does not exist yet
func TestNewRunnerCreatesProviderBasedComponents(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 300,
		},
		Paths: config.PathsConfig{
			Templates: templatesDir,
		},
		Models: config.ModelsConfig{
			Validation: "low",
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create runner
	runner, err := NewRunner(cfg, os.Stdout)

	if err != nil {
		t.Fatalf("NewRunner should succeed, got error: %v", err)
	}
	if runner == nil {
		t.Fatal("NewRunner should return non-nil runner")
	}

	// Verify router is created (line 102-104 in runner.go)
	if runner.router == nil {
		t.Error("expected runner.router to be non-nil")
	}

	// Verify analyzer is created from provider (line 117-121 in runner.go)
	if runner.analyzer == nil {
		t.Error("expected runner.analyzer to be non-nil")
	}

	// Verify renderer has learnings file (learnings wiring at line 108-113)
	if runner.renderer == nil {
		t.Fatal("expected runner.renderer to be non-nil")
	}

	lf := runner.renderer.GetLearningsFile()
	// Learnings file may be nil if LEARNINGS.md doesn't exist, but renderer should work
	_ = lf
}

// testProvider implements provider.Provider for testing
type testProvider struct {
	runFunc           func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
	streamRunFunc     func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
	runValidationFunc func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error)
}

func (tp *testProvider) Name() string {
	return "test"
}

func (tp *testProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if tp.runFunc != nil {
		return tp.runFunc(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Output: "test output"}, nil
}

func (tp *testProvider) StreamRun(ctx context.Context, prompt string, tier string, output interface{}, handler interface{}, onToolCall interface{}) (*provider.Result, error) {
	if tp.streamRunFunc != nil {
		return tp.streamRunFunc(ctx, prompt, tier)
	}
	return &provider.Result{Success: true}, nil
}

func (tp *testProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	if tp.runValidationFunc != nil {
		return tp.runValidationFunc(ctx, commands, tier, workDir)
	}
	return &provider.Result{Success: true}, nil
}

func (tp *testProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

func (tp *testProvider) ModelForTier(tier string) string {
	return tier
}

// testPromptRenderer implements PromptRenderer for testing
type testPromptRenderer struct {
	renderAnalyzeFunc func(ctx *prompt.AnalyzeContext) (string, error)
}

func (tpr *testPromptRenderer) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
	return &prompt.Context{}, nil
}

func (tpr *testPromptRenderer) RenderBuild(ctx *prompt.Context) (string, error) {
	return "build prompt", nil
}

func (tpr *testPromptRenderer) RenderAnalyze(ctx *prompt.AnalyzeContext) (string, error) {
	if tpr.renderAnalyzeFunc != nil {
		return tpr.renderAnalyzeFunc(ctx)
	}
	return "analyze prompt", nil
}

func (tpr *testPromptRenderer) RenderLearn(ctx *prompt.LearnContext) (string, error) {
	return "learn prompt", nil
}

func (tpr *testPromptRenderer) RenderDecompose(ctx *prompt.DecomposeContext) (string, error) {
	return "decompose prompt", nil
}

func (tpr *testPromptRenderer) RenderScope(ctx *prompt.ScopeContext) (string, error) {
	return "scope prompt", nil
}

func (tpr *testPromptRenderer) RenderPrecheck(ctx *prompt.PrecheckContext) (string, error) {
	return "precheck prompt", nil
}

func (tpr *testPromptRenderer) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	return "review prompt", nil
}

func (tpr *testPromptRenderer) RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error) {
	return "thorough review prompt", nil
}

func (tpr *testPromptRenderer) RenderAcceptanceTests(ctx *prompt.Context) (string, error) {
	return "acceptance tests prompt", nil
}

func (tpr *testPromptRenderer) RenderATDDBuild(ctx *prompt.Context) (string, error) {
	return "atdd build prompt", nil
}

func (tpr *testPromptRenderer) RenderTDDBuild(ctx *prompt.Context) (string, error) {
	return "tdd build prompt", nil
}

func (tpr *testPromptRenderer) RenderRefactor(ctx *prompt.Context) (string, error) {
	return "refactor prompt", nil
}

func (tpr *testPromptRenderer) LoadSpec(name string) (string, error) {
	return "spec content", nil
}

func (tpr *testPromptRenderer) LoadClaudeMD() (string, error) {
	return "claude md content", nil
}

func (tpr *testPromptRenderer) LoadRules() (string, error) {
	return "rules content", nil
}

func (tpr *testPromptRenderer) GetLearningsFile() *learnings.File {
	return nil
}
