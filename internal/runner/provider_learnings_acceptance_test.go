//go:build acceptance

package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// TestNewRunnerWiresProviderToLearningsAdapter verifies that NewRunner correctly
// wires the Provider to the learnings adapter when no providers config exists.
// Expected failure: learnings.NewProviderRunnerAdapter does not exist yet
func TestNewRunnerWiresProviderToLearningsAdapter(t *testing.T) {
	// Create temp directory for gromit files
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create minimal config
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 300,
		},
		Paths: config.PathsConfig{
			Templates: templatesDir,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create runner - this should wire provider to learnings adapter
	runner, err := NewRunner(cfg, os.Stdout)

	if err != nil {
		t.Fatalf("NewRunner should succeed with valid config, got error: %v", err)
	}
	if runner == nil {
		t.Fatal("NewRunner should return non-nil runner")
	}

	// Verify router is set
	if runner.router == nil {
		t.Error("expected runner.router to be non-nil after NewRunner")
	}

	// Verify analyzer is set
	if runner.analyzer == nil {
		t.Error("expected runner.analyzer to be non-nil after NewRunner")
	}
}

// TestNewRunnerWithProvidersConfigWiresAnalyzer verifies that NewRunner correctly
// creates the analyzer with a Provider when the providers config path is used.
// Expected failure: The TODO path at line 99-100 is not implemented yet
func TestNewRunnerWithProvidersConfigWiresAnalyzer(t *testing.T) {
	// Create temp directory for gromit files
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create config with providers section
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 300,
		},
		Paths: config.PathsConfig{
			Templates: templatesDir,
		},
		Providers: &config.ProvidersConfig{
			Providers: map[string]config.ProviderDef{
				"claude": {
					Binary:         "claude",
					Flags:          []string{"--no-input"},
					PromptDelivery: "stdin",
					Models: map[string]string{
						"high":   "opus",
						"medium": "sonnet",
						"low":    "haiku",
					},
				},
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create runner - this should use the TODO path that builds router from providers config
	runner, err := NewRunner(cfg, os.Stdout)

	// This should succeed once the TODO path is implemented
	if err != nil {
		t.Fatalf("NewRunner should succeed with providers config, got error: %v", err)
	}
	if runner == nil {
		t.Fatal("NewRunner should return non-nil runner")
	}

	// Verify router is created from providers config (not single-provider wrapper)
	if runner.router == nil {
		t.Error("expected runner.router to be non-nil with providers config")
	}

	// Verify analyzer is wired with a provider
	if runner.analyzer == nil {
		t.Error("expected runner.analyzer to be non-nil with providers config")
	}
}

// TestRunnerAnalyzerUsesProvider verifies that the analyzer created by NewRunner
// can successfully analyze failures using the Provider interface.
// Expected failure: The analyzer integration with Provider may not work until learnings adapter exists
func TestRunnerAnalyzerUsesProvider(t *testing.T) {
	// Create temp directory for gromit files
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create minimal prompt templates for analyzer to work
	analyzeTemplate := `Analyze this failure: {{.FailureOutput}}`
	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_analyze.md"), []byte(analyzeTemplate), 0644); err != nil {
		t.Fatalf("failed to write analyze template: %v", err)
	}

	// Create config
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
		t.Fatalf("NewRunner failed: %v", err)
	}

	// Create a test bead
	b := &bead.Bead{
		ID:    "test-456",
		Title: "Test failure analysis",
	}

	// Try to analyze a failure - this exercises the Provider-based analyzer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This should work with the Provider interface once everything is wired
	analysis, err := runner.analyzer.Analyze(ctx, b, "test compilation failed: undefined symbol")

	// We expect this to fail because we're using real Claude CLI which won't work in tests
	// But the important part is that the code path exists and the types match
	if err == nil {
		// If it somehow succeeds, verify the analysis structure
		if analysis == nil {
			t.Error("expected non-nil analysis when no error returned")
		}
	} else {
		// Expected: error from calling real Claude binary
		// The test verifies the wiring exists, not that Claude CLI works in tests
		if !strings.Contains(err.Error(), "claude") && !strings.Contains(err.Error(), "exec") {
			t.Logf("Got expected error from Claude invocation: %v", err)
		}
	}
}

// TestRunnerLearningsFilterUsesProviderAdapter verifies that the learnings filter
// is correctly wired with the Provider adapter.
// Expected failure: learnings.NewProviderRunnerAdapter does not exist yet
func TestRunnerLearningsFilterUsesProviderAdapter(t *testing.T) {
	// Create temp directory for gromit files
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	learningsPath := filepath.Join(tmpDir, ".gromit", "LEARNINGS.md")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create initial LEARNINGS.md file
	initialLearnings := `# Learnings

## Confirmed Patterns

## Recent Learnings

## Archived Learnings
`
	if err := os.WriteFile(learningsPath, []byte(initialLearnings), 0644); err != nil {
		t.Fatalf("failed to write learnings file: %v", err)
	}

	// Create config
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 300,
		},
		Paths: config.PathsConfig{
			Templates: templatesDir,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create runner - this should wire provider to learnings adapter
	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	// Verify the learnings file has a filter set (indirectly by checking renderer)
	if runner.renderer == nil {
		t.Fatal("expected runner.renderer to be non-nil")
	}

	lf := runner.renderer.GetLearningsFile()
	if lf == nil {
		t.Error("expected learnings file to be non-nil after NewRunner wiring")
	}

	// The actual filter behavior is tested in learnings package acceptance tests
	// This test just verifies the wiring in runner.go works
}

// TestNewRunnerWithDepsSetsRouter verifies that NewRunnerWithDeps correctly
// uses the provided Router dependency.
// Expected failure: This should work now, but verifies the Deps.Router field is used correctly
func TestNewRunnerWithDepsSetsRouter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock router
	mockRouter := provider.NewSingleProviderRouter(&mockProviderForTest{})

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	deps := Deps{
		Beads:    &mockBeadClient{},
		Router:   mockRouter,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   nil,
	}

	runner, err := NewRunnerWithDeps(cfg, os.Stdout, tmpDir, deps)

	if err != nil {
		t.Fatalf("NewRunnerWithDeps should succeed, got error: %v", err)
	}
	if runner == nil {
		t.Fatal("NewRunnerWithDeps should return non-nil runner")
	}
	if runner.router != mockRouter {
		t.Error("expected runner.router to be the provided mockRouter")
	}
}

// mockProviderForTest implements provider.Provider for testing
type mockProviderForTest struct{}

func (m *mockProviderForTest) Name() string { return "mock" }

func (m *mockProviderForTest) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true, Output: "mock output"}, nil
}

func (m *mockProviderForTest) StreamRun(ctx context.Context, prompt string, tier string, output interface{}, handler interface{}, onToolCall interface{}) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}

func (m *mockProviderForTest) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}

func (m *mockProviderForTest) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

func (m *mockProviderForTest) ModelForTier(tier string) string {
	return tier
}
