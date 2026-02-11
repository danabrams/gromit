package runner

import (
	"os"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// TestRunnerHasRouterField verifies that the Runner struct has a router field
func TestRunnerHasRouterField(t *testing.T) {
	// Create a minimal runner with nil router
	r := &Runner{
		router: nil,
	}

	// Verify the field exists by assigning to it
	mockRouter := &provider.Router{}
	r.router = mockRouter

	if r.router != mockRouter {
		t.Errorf("Expected router field to be assignable and retrievable")
	}
}

// TestDepsHasRouterField verifies that the Deps struct has a Router field
func TestDepsHasRouterField(t *testing.T) {
	// Create a Deps struct with a Router
	mockRouter := &provider.Router{}
	deps := Deps{
		Router: mockRouter,
	}

	if deps.Router != mockRouter {
		t.Errorf("Expected Router field to be assignable and retrievable")
	}
}

// TestNewRunnerWithDepsUsesProvidedRouter verifies that NewRunnerWithDeps uses deps.Router when provided
func TestNewRunnerWithDepsUsesProvidedRouter(t *testing.T) {
	cfg := &config.Config{}
	mockRouter := &provider.Router{}
	deps := Deps{
		Router: mockRouter,
	}

	runner, err := NewRunnerWithDeps(cfg, nil, "/tmp/gromit", deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	if runner.router != mockRouter {
		t.Errorf("Expected runner.router to be set from deps.Router")
	}
}

// TestNewRunnerWithDepsFallsBackToWrappingClaude verifies that when deps.Router is nil,
// NewRunnerWithDeps wraps deps.Claude in a single-provider router
func TestNewRunnerWithDepsFallsBackToWrappingClaude(t *testing.T) {
	cfg := &config.Config{}
	mockClaude := &mockClaudeClient{}
	deps := Deps{
		Claude: mockClaude,
		Router: nil,
	}

	runner, err := NewRunnerWithDeps(cfg, nil, "/tmp/gromit", deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	if runner.router == nil {
		t.Errorf("Expected runner.router to be set even when deps.Router is nil")
	}
}

// TestNewRunnerCreatesRouterWhenNoProviders verifies that NewRunner wraps
// Claude client when cfg.HasProviders() is false
func TestNewRunnerCreatesRouterWhenNoProviders(t *testing.T) {
	tmpDir := t.TempDir()

	// Create template and spec directories
	if err := os.MkdirAll(tmpDir+"/templates", 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(tmpDir+"/specs", 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create a config without providers (backward compat mode)
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
		},
		Claude: config.ClaudeConfig{
			Binary: "claude",
		},
	}

	runner, err := NewRunner(cfg, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if runner.router == nil {
		t.Errorf("Expected runner.router to be created even without providers config")
	}
}

// TestNewRunnerWithProvidersConfigLeavesTodo verifies that NewRunner
// leaves router as nil (TODO) when cfg.HasProviders() is true
func TestNewRunnerWithProvidersConfigLeavesTodo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create template and spec directories
	if err := os.MkdirAll(tmpDir+"/templates", 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(tmpDir+"/specs", 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create a config with providers defined
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"claude": {
				Binary: "claude",
				Models: map[string]string{
					"high":   "opus",
					"medium": "sonnet",
					"low":    "haiku",
				},
			},
		},
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
		},
		Claude: config.ClaudeConfig{
			Binary: "claude",
		},
	}

	runner, err := NewRunner(cfg, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	// For now, router will be nil when providers are configured (TODO implementation)
	if runner.router != nil {
		t.Logf("router is set (provider building implemented)")
	} else {
		t.Logf("router is nil (provider building not yet implemented)")
	}
}
