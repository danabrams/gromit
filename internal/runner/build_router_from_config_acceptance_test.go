//go:build acceptance

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func routerAcceptanceConfig(t *testing.T) *config.Config {
	t.Helper()
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	cfg := &config.Config{
		Claude: config.ClaudeConfig{Binary: "claude", Timeout: 300},
		Paths:  config.PathsConfig{Templates: templatesDir},
		Providers: map[string]config.ProviderDef{
			"claude": {
				Binary: "claude",
				Models: map[string]string{"high": "opus", "medium": "sonnet", "low": "haiku"},
			},
			"openai": {
				Binary: "codex",
				Models: map[string]string{"high": "o3", "medium": "gpt-4o", "low": "gpt-4o-mini"},
			},
		},
		Routing: config.RoutingConfig{
			PhasePreferences: map[string]string{"build": "claude", "validate": "any"},
			Ratio:            map[string]int{"claude": 60, "openai": 40},
			Fallback:         config.FallbackConfig{Enabled: true, Cooldown: "30m"},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

func TestRunnerAcceptanceSurfaceOnly_RouterSelection(t *testing.T) {
	runner, err := NewRunner(routerAcceptanceConfig(t), os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if runner == nil {
		t.Fatal("expected non-nil runner")
	}
	if runner.router == nil {
		t.Fatal("expected router wiring for provider config")
	}
}

func TestRunnerAcceptanceReducedCountRouterPrimary(t *testing.T) {
	src := readRunnerTestFile(t, "build_router_from_config_acceptance_test.go")
	if count := strings.Count(src, "\nfunc Test"); count > 3 {
		t.Fatalf("build_router_from_config_acceptance_test.go contains %d tests; expected <= 3", count)
	}
}
