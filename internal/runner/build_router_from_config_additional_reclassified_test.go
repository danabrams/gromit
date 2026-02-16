
package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// setupSingleProviderConfig creates a config with one provider and basic routing.
func setupSingleProviderConfig(t *testing.T, name string, pDef config.ProviderDef) *config.Config {
	t.Helper()
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}
	cfg := &config.Config{
		Claude:    config.ClaudeConfig{Binary: "claude", Timeout: 300},
		Paths:     config.PathsConfig{Templates: templatesDir},
		Models:    config.ModelsConfig{Validation: "low"},
		Providers: map[string]config.ProviderDef{name: pDef},
		Routing:   config.RoutingConfig{Ratio: map[string]int{name: 100}},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

// TestNewRunnerCodexOnlyConfigWiresRouterAndAnalyzer verifies that when only
// a non-claude provider (codex) is configured, both router and analyzer are created.
// Expected failure: The TODO at runner.go:103 is not implemented — router is nil.
func TestNewRunnerCodexOnlyConfigWiresRouterAndAnalyzer(t *testing.T) {
	cfg := setupSingleProviderConfig(t, "openai", config.ProviderDef{
		Binary: "codex", PromptDelivery: "prompt_file_arg", PromptFlag: "--prompt",
		Models: map[string]string{"high": "o3", "medium": "gpt-4o", "low": "gpt-4o-mini"},
	})

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if runner.router == nil {
		t.Fatal("expected router to be non-nil for codex-only config")
	}
	if runner.analyzer == nil {
		t.Fatal("expected analyzer to be non-nil with codex-only config")
	}

	p, model := runner.router.Select("build", provider.TierHigh)
	if p == nil {
		t.Fatal("Select returned nil for codex-only config")
	}
	if p.Name() != "codex" {
		t.Errorf("expected codex provider, got %q", p.Name())
	}
	if model != "o3" {
		t.Errorf("expected model 'o3' for high tier, got %q", model)
	}
}

// TestNewRunnerNoPhasePreferencesUsesRatioOnly verifies that when providers
// are configured but no phase preferences exist, all phases route via ratio.
// Expected failure: The TODO at runner.go:103 is not implemented — router is nil.
func TestNewRunnerNoPhasePreferencesUsesRatioOnly(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	cfg := &config.Config{
		Claude: config.ClaudeConfig{Binary: "claude", Timeout: 300},
		Paths:  config.PathsConfig{Templates: templatesDir},
		Models: config.ModelsConfig{Validation: "low"},
		Providers: map[string]config.ProviderDef{
			"claude": {Binary: "claude", Models: map[string]string{
				"high": "opus", "medium": "sonnet", "low": "haiku",
			}},
			"openai": {Binary: "codex", PromptFlag: "--prompt", Models: map[string]string{
				"high": "o3", "medium": "gpt-4o", "low": "gpt-4o-mini",
			}},
		},
		Routing: config.RoutingConfig{
			Ratio: map[string]int{"claude": 70, "openai": 30},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if runner.router == nil {
		t.Fatal("expected router to be non-nil with empty phase preferences")
	}

	// With 70/30 ratio and 0 counts, claude has larger gap and should be first
	p, _ := runner.router.Select("build", provider.TierMedium)
	if p == nil {
		t.Fatal("Select returned nil")
	}
	if p.Name() != "claude" {
		t.Errorf("expected claude (70%% ratio) first, got %q", p.Name())
	}
}

// TestNewRunnerTwoProviderConfigBothReachable verifies the router has both
// providers registered and reachable via phase selection.
// Expected failure: The TODO at runner.go:103 is not implemented — router is nil.
func TestNewRunnerTwoProviderConfigBothReachable(t *testing.T) {
	cfg := setupTwoProviderConfig(t)
	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if runner.router == nil {
		t.Fatal("expected router to be non-nil")
	}

	seen := make(map[string]bool)
	for _, phase := range []string{"build", "validate", "analyze", "review"} {
		p, _ := runner.router.Select(phase, provider.TierMedium)
		if p != nil {
			seen[p.Name()] = true
		}
	}
	if !seen["claude"] {
		t.Error("claude provider never selected — not registered in router")
	}
	if !seen["codex"] {
		t.Error("codex provider never selected — not registered in router")
	}
}
