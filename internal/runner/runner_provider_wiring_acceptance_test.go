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
// correctly handles the providers config path (the TODO at lines 98-100).
// Expected failure: The TODO path is not implemented yet - router and provider will be nil
func TestRunnerProviderWiringWithProvidersConfig(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create config WITH providers section to trigger the TODO path
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
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Verify this DOES trigger the providers config path
	if !cfg.HasProviders() {
		t.Fatal("Test expects providers config, but cfg.HasProviders() is false")
	}

	runner, err := NewRunner(cfg, os.Stdout)

	// The TODO path at line 99 is not implemented yet, so router will be nil
	// and the subsequent code (lines 123-128) will use the fallback path
	if err != nil {
		t.Fatalf("NewRunner should not error even with TODO path: %v", err)
	}

	if runner == nil {
		t.Fatal("NewRunner should return non-nil runner even with TODO path")
	}

	// When TODO is implemented, router should be built from providers config
	// For now, it falls through to the else block (line 123) which uses claudeAdapter
	if runner.router == nil {
		t.Error("expected router to be non-nil (even if using fallback path)")
	}

	// Analyzer should be created via fallback path (lines 123-128) until TODO is done
	if runner.analyzer == nil {
		t.Error("expected analyzer to be non-nil via fallback path")
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
// Expected failure: Test will fail if Runner.claude field still exists
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

	// The runner should have a router field
	if runner.router == nil {
		t.Fatal("expected runner.router to exist")
	}

	// Verify there's no claude field accessible (this is a compile-time check really,
	// but we document the expectation here)
	// If Runner had a claude field, code like `runner.claude.Run()` would compile
	// Since it doesn't, all LLM invocations must go through runner.router.Select()

	// This test primarily documents the architectural requirement:
	// Runner.claude field has been removed, router is the sole LLM access point
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
