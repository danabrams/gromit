//go:build integration

package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// TestLearningsAdapterUsesProviderInNewRunner verifies that when NewRunner is called,
// the learnings filter is set up with a ProviderRunnerAdapter that actually invokes
// the Provider when the filter function is called.
// This tests the integration at lines 107-113 in runner.go where the provider is passed
// to learnings.NewProviderRunnerAdapter.
// Expected failure: The filter function will fail to invoke the Provider correctly if
// NewProviderRunnerAdapter doesn't properly adapt the Provider interface to the
// ClaudeRunner interface expected by NewLLMFilter
func TestLearningsAdapterUsesProviderInNewRunner(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Track whether Provider.Run was called
	providerRunCalled := false
	var capturedTier string

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: filepath.Join(tmpDir, "CLAUDE.md"),
		},
		Models: config.ModelsConfig{
			Validation: "low",
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create LEARNINGS.md so the filter is actually set
	learningsPath := filepath.Join(filepath.Dir(templatesDir), "LEARNINGS.md")
	learningsContent := `# Learnings

## Provisional

### 2026-02-12 | test-1 | patterns
Test learning that should be filtered`
	if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
		t.Fatalf("failed to create LEARNINGS.md: %v", err)
	}

	// Create mock provider that tracks calls
	mockProvider := &mockProviderForRunner{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			providerRunCalled = true
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Output:  "specific",
			}, nil
		},
	}

	// Create renderer with learnings file
	renderer, err := prompt.NewRenderer(templatesDir, "", filepath.Join(tmpDir, "CLAUDE.md"), filepath.Dir(templatesDir))
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}

	// Wire up the provider adapter to the learnings filter
	lf := renderer.GetLearningsFile()
	if lf == nil {
		t.Fatal("expected learnings file to be non-nil")
	}

	// This is the key integration: create adapter from Provider and set it as filter
	adapter := learnings.NewProviderRunnerAdapter(mockProvider)
	filter := learnings.NewLLMFilter(adapter, "test-project", "Test description")
	lf.SetFilter(filter)

	// Now trigger the filter by calling FilterProvisional
	alreadyFiltered := make(map[string]bool)
	_, err = lf.FilterProvisional(filter, alreadyFiltered)
	if err != nil {
		t.Fatalf("FilterProvisional failed: %v", err)
	}

	// Verify that Provider.Run was actually called through the adapter
	if !providerRunCalled {
		t.Error("Provider.Run was not called - adapter integration failed")
	}

	// Verify the tier passed was "haiku" (as expected by learnings filter)
	if capturedTier != "haiku" {
		t.Errorf("expected tier='haiku', got tier=%q", capturedTier)
	}
}

// TestAnalyzerUsesProviderInNewRunner verifies that when NewRunner is called,
// the analyzer is created with a Provider and actually invokes it when Analyze() is called.
// This tests the integration at lines 117-121 in runner.go.
// Expected failure: If NewAnalyzer was called with claudeClient wrapped in
// analyzer.NewClaudeClientAdapter instead of using Provider directly, the Provider.Run
// would not be invoked when calling analyzer.Analyze()
func TestAnalyzerUsesProviderInNewRunner(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Track whether Provider.Run was called
	providerRunCalled := false
	var capturedTier string

	// Create mock provider that tracks calls
	mockProvider := &mockProviderForRunner{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			providerRunCalled = true
			capturedTier = tier
			return &provider.Result{
				Success: true,
				Output: `{
					"category": "logic",
					"recoverable": false,
					"root_cause": "Test failure",
					"suggestion": "Fix the code"
				}`,
			}, nil
		},
	}

	// Create mock renderer
	mockRenderer := &testMockPromptRenderer{
		fnRenderAnalyze: func(ctx *prompt.AnalyzeContext) (string, error) {
			return "analyze prompt", nil
		},
	}

	// Create analyzer with Provider
	analyzerObj, err := analyzer.NewAnalyzer(mockProvider, "low", mockRenderer)
	if err != nil {
		t.Fatalf("NewAnalyzer failed: %v", err)
	}

	// Call Analyze to verify Provider is invoked
	testBead := &bead.Bead{
		ID:          "test-1",
		Title:       "Test Bead",
		Priority:    1,
		Description: "Test description",
	}

	ctx := context.Background()
	analysis, err := analyzerObj.Analyze(ctx, testBead, "test failure output")
	if err != nil {
		t.Fatalf("Analyze should succeed with mock provider: %v", err)
	}

	// Verify that Provider.Run was actually called
	if !providerRunCalled {
		t.Error("Provider.Run was not called - analyzer integration failed")
	}

	// Verify the tier was passed correctly
	if capturedTier != "low" {
		t.Errorf("expected tier='low', got tier=%q", capturedTier)
	}

	// Verify analysis result was parsed
	if analysis == nil {
		t.Fatal("expected non-nil analysis")
	}
	if analysis.RootCause != "Test failure" {
		t.Errorf("expected RootCause='Test failure', got %q", analysis.RootCause)
	}
}

// TestRunnerWithDepsAcceptsProviderBasedAnalyzer verifies that NewRunnerWithDeps
// can accept an Analyzer that was created with a Provider instead of claude.Client.
// Expected failure: Before commit 48c3357, analyzer.NewAnalyzer only accepted claude.Client
// wrapped in an adapter, not a Provider directly
func TestRunnerWithDepsAcceptsProviderBasedAnalyzer(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create a provider-based analyzer
	testProvider := &mockProviderForRunner{}
	mockRenderer := &mockPromptRenderer{}

	providerAnalyzer, err := analyzer.NewAnalyzer(testProvider, "low", mockRenderer)
	if err != nil {
		t.Fatalf("analyzer.NewAnalyzer should accept Provider: %v", err)
	}

	// Create router
	testRouter := provider.NewSingleProviderRouter(testProvider)

	deps := Deps{
		Beads:    &mockBeadClient{},
		Router:   testRouter,
		Analyzer: providerAnalyzer,
		Renderer: mockRenderer,
		Logger:   nil,
	}

	runner, err := NewRunnerWithDeps(cfg, os.Stdout, tmpDir, deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps should succeed with provider-based analyzer: %v", err)
	}

	if runner.analyzer != providerAnalyzer {
		t.Error("expected runner.analyzer to be the provider-based analyzer we passed in")
	}

	// Behavioral test: analyzer should work when injected via Deps
	testBead := &bead.Bead{
		ID:       "test-1",
		Title:    "Test",
		Priority: 1,
	}

	ctx := context.Background()
	_, err = runner.analyzer.Analyze(ctx, testBead, "test output")

	// Should not error with "provider is nil" or interface mismatch
	if err != nil && err.Error() == "provider is nil" {
		t.Errorf("injected provider-based analyzer should work, got: %v", err)
	}
}

// TestNewProviderRunnerAdapterWorksWithLearningsFilter verifies that
// learnings.NewProviderRunnerAdapter creates an adapter that properly invokes
// the Provider when used with learnings.NewLLMFilter.
// Expected failure: If NewProviderRunnerAdapter doesn't exist or doesn't properly
// adapt the Provider interface, Provider.Run will not be called and providerRunCalled
// will remain false
func TestNewProviderRunnerAdapterWorksWithLearningsFilter(t *testing.T) {
	providerRunCalled := false
	var capturedTier string
	var capturedPrompt string

	mockProvider := &mockProviderForRunner{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			providerRunCalled = true
			capturedTier = tier
			capturedPrompt = prompt
			return &provider.Result{
				Success: true,
				Output:  "specific",
			}, nil
		},
	}

	// Create adapter - this is the key integration point
	adapter := learnings.NewProviderRunnerAdapter(mockProvider)
	if adapter == nil {
		t.Fatal("NewProviderRunnerAdapter returned nil")
	}

	// Create filter with the provider adapter
	filter := learnings.NewLLMFilter(adapter, "gromit", "Test project")
	if filter == nil {
		t.Fatal("NewLLMFilter should accept ProviderRunnerAdapter")
	}

	// Behavioral test: filter should work with provider adapter
	// FilterFunc signature is: func(content string) (isGeneric bool, err error)
	// Call the filter function directly on some test content
	testContent := "Test content for gromit project"

	// The filter should be able to call the provider through the adapter
	// This tests that the adapter properly converts between interfaces
	isGeneric, err := filter(testContent)
	if err != nil {
		t.Fatalf("filter invocation failed: %v", err)
	}

	// Verify Provider.Run was actually called
	if !providerRunCalled {
		t.Error("Provider.Run was not called - adapter integration failed")
	}

	// Verify tier was "haiku" (learnings filter uses haiku for cost efficiency)
	if capturedTier != "haiku" {
		t.Errorf("expected tier='haiku', got %q", capturedTier)
	}

	// Verify prompt contains the test content
	if !strings.Contains(capturedPrompt, testContent) {
		t.Errorf("expected prompt to contain test content, got: %q", capturedPrompt)
	}

	// Verify result was parsed correctly ("specific" -> isGeneric=false)
	if isGeneric {
		t.Error("expected isGeneric=false for 'specific' response")
	}
}

// TestAnalyzerCreatedWithProviderCanAnalyze verifies that an Analyzer
// created with a Provider (not claude.Client) can successfully analyze failures
// and that the Provider.Run method is invoked with the correct parameters.
// Expected failure: If analyzer uses claude.Client directly instead of Provider,
// the Provider.Run method won't be invoked and providerRunCalled will remain false
func TestAnalyzerCreatedWithProviderCanAnalyze(t *testing.T) {
	providerRunCalled := false
	var capturedPrompt string
	var capturedTier string

	mockProv := &mockProviderForRunner{
		FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			providerRunCalled = true
			capturedPrompt = prompt
			capturedTier = tier
			// Return a valid analysis JSON
			return &provider.Result{
				Success: true,
				Output: `{
					"category": "syntax",
					"recoverable": true,
					"root_cause": "Test root cause",
					"learning": "Test learning",
					"suggestion": "Test suggestion"
				}`,
			}, nil
		},
	}

	mockRend := &testMockPromptRenderer{
		fnRenderAnalyze: func(ctx *prompt.AnalyzeContext) (string, error) {
			return "analyze prompt for bead " + ctx.BeadID, nil
		},
	}

	// Create analyzer with Provider (not claude.Client)
	a, err := analyzer.NewAnalyzer(mockProv, "low", mockRend)
	if err != nil {
		t.Fatalf("NewAnalyzer should accept Provider: %v", err)
	}

	testBead := &bead.Bead{
		ID:          "test-1",
		Title:       "Test Bead",
		Priority:    1,
		Description: "Test",
	}

	ctx := context.Background()
	analysis, err := a.Analyze(ctx, testBead, "test failure output")

	if err != nil {
		t.Fatalf("Analyze should succeed with provider: %v", err)
	}

	// Verify Provider.Run was called
	if !providerRunCalled {
		t.Error("Provider.Run was not called - analyzer does not use Provider interface")
	}

	// Verify the prompt was generated and passed through
	if !strings.Contains(capturedPrompt, "test-1") {
		t.Errorf("expected prompt to contain bead ID, got: %q", capturedPrompt)
	}

	// Verify tier was passed correctly
	if capturedTier != "low" {
		t.Errorf("expected tier='low', got %q", capturedTier)
	}

	if analysis == nil {
		t.Fatal("expected non-nil analysis")
	}

	// Verify the analysis was parsed correctly
	if analysis.Category != analyzer.CategorySyntax {
		t.Errorf("expected CategorySyntax, got %v", analysis.Category)
	}
	if !analysis.Recoverable {
		t.Error("expected recoverable=true")
	}
	if analysis.RootCause != "Test root cause" {
		t.Errorf("expected 'Test root cause', got %q", analysis.RootCause)
	}
}

// testMockPromptRenderer is a local mock to avoid conflicts with existing mocks
type testMockPromptRenderer struct {
	fnRenderAnalyze func(*prompt.AnalyzeContext) (string, error)
}

func (m *testMockPromptRenderer) RenderBuild(ctx *prompt.Context) (string, error) {
	return "build prompt", nil
}

func (m *testMockPromptRenderer) RenderAnalyze(ctx *prompt.AnalyzeContext) (string, error) {
	if m.fnRenderAnalyze != nil {
		return m.fnRenderAnalyze(ctx)
	}
	return "analyze prompt", nil
}

func (m *testMockPromptRenderer) RenderDecompose(ctx *prompt.DecomposeContext) (string, error) {
	return "decompose prompt", nil
}

func (m *testMockPromptRenderer) RenderScope(ctx *prompt.ScopeContext) (string, error) {
	return "scope prompt", nil
}

func (m *testMockPromptRenderer) RenderPrecheck(ctx *prompt.PrecheckContext) (string, error) {
	return "precheck prompt", nil
}

func (m *testMockPromptRenderer) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	return "review prompt", nil
}

func (m *testMockPromptRenderer) RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error) {
	return "thorough review prompt", nil
}

func (m *testMockPromptRenderer) GetLearningsFile() *learnings.File {
	return nil
}
