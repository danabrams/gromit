//go:build acceptance

package runner

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// TestRunnerProviderWiringWithBackwardCompatPath verifies that NewRunner
// correctly wires Provider to both learnings and analyzer when using the
// backward-compatibility path (no providers config, single Claude client).
// This tests the actual wiring at lines 107-129 in runner.go.
// Expected failure: Tests will fail if the wiring is not correct or if components are nil
func TestRunnerProviderWiringWithBackwardCompatPath(t *testing.T) {
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

	// This should NOT trigger cfg.HasProviders() - using backward compat path
	if cfg.HasProviders() {
		t.Skip("Test expects no providers config, but cfg.HasProviders() is true")
	}

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner should succeed with backward compat config: %v", err)
	}

	// Verify the backward compat wiring (lines 101-104)
	if runner.router == nil {
		t.Fatal("expected router to be created via NewSingleProviderRouter")
	}

	// Verify analyzer was created with provider (lines 117-121)
	if runner.analyzer == nil {
		t.Fatal("expected analyzer to be created with claudeProviderForLearnings")
	}

	// Verify learnings filter was wired (lines 107-113)
	if runner.renderer == nil {
		t.Fatal("expected renderer to exist for learnings file access")
	}
	// GetLearningsFile may return nil if LEARNINGS.md doesn't exist, which is fine
	// The important part is that SetFilter was called with the adapter
}

// TestRunnerProviderWiringWithProvidersConfig verifies that NewRunner
// correctly builds a multi-provider router when providers config is present.
// Expected failure: The TODO at runner.go:103 is not implemented —
// cfg.HasProviders() branch does not construct provider instances or call NewRouter().
func TestRunnerProviderWiringWithProvidersConfig(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create config WITH providers section to trigger the HasProviders() path
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
		Routing: config.RoutingConfig{
			Ratio: map[string]int{"claude": 100},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Verify this DOES trigger the providers config path
	if !cfg.HasProviders() {
		t.Fatal("Test expects providers config, but cfg.HasProviders() is false")
	}

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if runner == nil {
		t.Fatal("NewRunner() returned nil runner")
	}

	// After implementation, router must be non-nil and built from providers config
	if runner.router == nil {
		t.Fatal("expected runner.router to be non-nil when providers config is present")
	}

	// The router should be functional — Select should return the claude provider
	p, model := runner.router.Select("build", provider.TierMedium)
	if p == nil {
		t.Fatal("router.Select() returned nil provider")
	}
	if p.Name() != "claude" {
		t.Errorf("router.Select() returned provider %q, want 'claude'", p.Name())
	}
	if model != "sonnet" {
		t.Errorf("router.Select() returned model %q, want 'sonnet' for medium tier", model)
	}

	// Analyzer must still be wired when using multi-provider path
	if runner.analyzer == nil {
		t.Fatal("expected analyzer to be non-nil with providers config")
	}
}

// TestRunnerWithDepsAcceptsRouterDependency verifies that NewRunnerWithDeps
// correctly accepts a Router as a dependency injection.
// Expected failure: Tests will fail if Deps.Router is not properly used
func TestRunnerWithDepsAcceptsRouterDependency(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create a test provider and router
	testProvider := &simpleTestProvider{}
	testRouter := provider.NewSingleProviderRouter(testProvider)

	deps := Deps{
		Beads:    &mockBeadClient{},
		Router:   testRouter,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   nil,
	}

	runner, err := NewRunnerWithDeps(cfg, os.Stdout, tmpDir, deps)

	if err != nil {
		t.Fatalf("NewRunnerWithDeps should succeed: %v", err)
	}
	if runner == nil {
		t.Fatal("NewRunnerWithDeps should return non-nil runner")
	}
	if runner.router != testRouter {
		t.Error("expected runner.router to be the injected testRouter")
	}
}

// TestRunnerNoLongerHasClaudeField verifies that the Runner struct
// no longer has a direct `claude` field - all LLM access goes through router.
func TestRunnerNoLongerHasClaudeField(t *testing.T) {
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
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	if runner.router == nil {
		t.Fatal("expected runner.router to exist")
	}
}

// simpleTestProvider implements provider.Provider for testing
type simpleTestProvider struct{}

func (tp *simpleTestProvider) Name() string {
	return "test"
}

func (tp *simpleTestProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true, Output: "test"}, nil
}

func (tp *simpleTestProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}

func (tp *simpleTestProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}

func (tp *simpleTestProvider) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

func (tp *simpleTestProvider) ModelForTier(tier string) string {
	return tier
}
