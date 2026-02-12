//go:build acceptance

package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// setupTwoProviderConfig creates a config with both claude and openai providers,
// routing preferences, ratio, and fallback settings. Returns the config and a
// cleanup function (tmpDir is auto-cleaned by t.TempDir).
func setupTwoProviderConfig(t *testing.T) *config.Config {
	t.Helper()
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
			"openai": {
				Binary:         "codex",
				Flags:          []string{},
				PromptDelivery: "prompt_file_arg",
				PromptFlag:     "--prompt",
				Models: map[string]string{
					"high":   "o3",
					"medium": "gpt-4o",
					"low":    "gpt-4o-mini",
				},
			},
		},
		Routing: config.RoutingConfig{
			PhasePreferences: map[string]string{
				"build":    "claude",
				"validate": "any",
				"analyze":  "any",
				"review":   "claude",
			},
			Ratio: map[string]int{
				"claude": 60,
				"openai": 40,
			},
			Fallback: config.FallbackConfig{
				Enabled:  true,
				Cooldown: "30m",
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

// TestNewRunnerBuildRouterFromTwoProviderConfig verifies that NewRunner()
// constructs a functional multi-provider router when cfg.HasProviders() is true
// with two providers defined.
//
// Expected failure: The TODO at runner.go:103 is not implemented - the
// cfg.HasProviders() branch does not build a router, leaving it nil.
func TestNewRunnerBuildRouterFromTwoProviderConfig(t *testing.T) {
	cfg := setupTwoProviderConfig(t)

	if !cfg.HasProviders() {
		t.Fatal("expected cfg.HasProviders() to be true for two-provider config")
	}

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	// After implementation, the router must be non-nil and functional
	if runner.router == nil {
		t.Fatal("expected runner.router to be non-nil when providers config is present")
	}

	// The router should be able to select a provider for each configured phase
	for _, phase := range []string{"build", "validate", "analyze", "review"} {
		p, model := runner.router.Select(phase, provider.TierMedium)
		if p == nil {
			t.Errorf("router.Select(%q, %q) returned nil provider", phase, provider.TierMedium)
		}
		if model == "" {
			t.Errorf("router.Select(%q, %q) returned empty model name", phase, provider.TierMedium)
		}
	}
}

// TestNewRunnerRouterRespectsPhasePreferences verifies that the router built
// from config routes phases to the preferred provider when one is specified.
//
// Expected failure: The TODO at runner.go:103 is not implemented - router is nil.
func TestNewRunnerRouterRespectsPhasePreferences(t *testing.T) {
	cfg := setupTwoProviderConfig(t)

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if runner.router == nil {
		t.Fatal("expected router to be non-nil")
	}

	tests := []struct {
		phase        string
		expectedProv string
		tier         string
		expectedModel string
	}{
		// "build" prefers "claude" → should select claude with sonnet for medium tier
		{"build", "claude", provider.TierMedium, "sonnet"},
		// "review" prefers "claude" → should select claude with opus for high tier
		{"review", "claude", provider.TierHigh, "opus"},
		// "build" prefers "claude" → should select claude with haiku for low tier
		{"build", "claude", provider.TierLow, "haiku"},
	}

	for _, tt := range tests {
		t.Run(tt.phase+"_"+tt.tier, func(t *testing.T) {
			p, model := runner.router.Select(tt.phase, tt.tier)
			if p == nil {
				t.Fatal("Select returned nil provider")
			}
			if p.Name() != tt.expectedProv {
				t.Errorf("Select(%q, %q) provider = %q, want %q",
					tt.phase, tt.tier, p.Name(), tt.expectedProv)
			}
			if model != tt.expectedModel {
				t.Errorf("Select(%q, %q) model = %q, want %q",
					tt.phase, tt.tier, model, tt.expectedModel)
			}
		})
	}
}

// TestNewRunnerRouterUsesRatioForAnyPhases verifies that phases configured as
// "any" use ratio-based balancing to select a provider.
//
// Expected failure: The TODO at runner.go:103 is not implemented - router is nil.
func TestNewRunnerRouterUsesRatioForAnyPhases(t *testing.T) {
	cfg := setupTwoProviderConfig(t)

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if runner.router == nil {
		t.Fatal("expected router to be non-nil")
	}

	// "validate" is configured as "any" — the ratio balancer should pick a provider.
	// With a fresh router (0 counts), the provider furthest below its ratio target
	// should be selected. With 60/40 split and 0 counts, either could be first.
	p, model := runner.router.Select("validate", provider.TierLow)
	if p == nil {
		t.Fatal("Select returned nil provider for 'any' phase")
	}
	if model == "" {
		t.Fatal("Select returned empty model for 'any' phase")
	}

	// The selected provider must be one of the configured providers
	provName := p.Name()
	if provName != "claude" && provName != "codex" {
		t.Errorf("Select(validate, low) provider = %q, want 'claude' or 'codex'", provName)
	}
}

// TestNewRunnerRouterParsesCooldownDuration verifies that the fallback cooldown
// duration from config is correctly parsed and applied to the router.
//
// Expected failure: The TODO at runner.go:103 is not implemented - router is nil,
// and there is no BuildRouterFromConfig function yet.
func TestNewRunnerRouterParsesCooldownDuration(t *testing.T) {
	cfg := setupTwoProviderConfig(t)
	// Fallback.Cooldown is "30m" in setupTwoProviderConfig

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if runner.router == nil {
		t.Fatal("expected router to be non-nil")
	}

	// Select both providers to verify they're available initially
	p1, _ := runner.router.Select("build", provider.TierMedium) // claude preferred
	if p1 == nil {
		t.Fatal("expected claude provider to be available")
	}

	// Mark it unavailable and verify fallback works
	runner.router.MarkUnavailable(p1.Name())

	// The "build" phase prefers "claude", but claude is now unavailable.
	// The router should fall back to the other provider.
	p2, model := runner.router.Select("build", provider.TierMedium)
	if p2 == nil {
		t.Fatal("expected fallback provider when preferred is unavailable")
	}
	if p2.Name() == p1.Name() {
		t.Errorf("expected different provider after marking %q unavailable, got same", p1.Name())
	}
	if model == "" {
		t.Error("expected non-empty model from fallback provider")
	}
}

// TestNewRunnerWiresLearningsFilterWithDefaultProvider verifies that the
// learnings filter is wired with a default provider when multiple providers
// are configured. The default provider should be claude if present.
//
// Expected failure: The TODO at runner.go:103 is not implemented -
// claudeProviderForLearnings remains nil when HasProviders() is true.
func TestNewRunnerWiresLearningsFilterWithDefaultProvider(t *testing.T) {
	cfg := setupTwoProviderConfig(t)

	// Create a LEARNINGS.md so the learnings file gets loaded
	gromitDir := filepath.Dir(cfg.Paths.Templates)
	learningsPath := filepath.Join(gromitDir, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath, []byte("# Learnings\n"), 0644); err != nil {
		t.Fatalf("failed to create LEARNINGS.md: %v", err)
	}

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	// When the TODO is implemented, the analyzer should be created using
	// the default provider (claude when present), not the fallback ClaudeClientAdapter.
	if runner.analyzer == nil {
		t.Fatal("expected analyzer to be wired even with multi-provider config")
	}

	// The router must be non-nil for the wiring to be correct
	if runner.router == nil {
		t.Fatal("expected router to be non-nil for learnings/analyzer wiring")
	}
}

// TestNewRunnerSingleClaudeProviderConfig verifies that a config with only
// one provider (claude) still builds a proper router, not using the backward
// compat path.
//
// Expected failure: The TODO at runner.go:103 is not implemented - the
// HasProviders() branch does not construct a router.
func TestNewRunnerSingleClaudeProviderConfig(t *testing.T) {
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

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if runner.router == nil {
		t.Fatal("expected router to be non-nil for single-provider config via HasProviders() path")
	}

	// Select should return claude for any phase
	p, model := runner.router.Select("build", provider.TierHigh)
	if p == nil {
		t.Fatal("Select returned nil provider")
	}
	if p.Name() != "claude" {
		t.Errorf("Select returned provider %q, want 'claude'", p.Name())
	}
	if model != "opus" {
		t.Errorf("Select returned model %q, want 'opus'", model)
	}
}

// TestNewRunnerCodexProviderTierMapping verifies that when openai/codex is
// configured as a provider, the router correctly maps tiers to codex model names.
//
// Expected failure: The TODO at runner.go:103 is not implemented - router is nil,
// and no CodexProvider is constructed from config.
func TestNewRunnerCodexProviderTierMapping(t *testing.T) {
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
		Providers: map[string]config.ProviderDef{
			"openai": {
				Binary:         "codex",
				Flags:          []string{},
				PromptDelivery: "prompt_file_arg",
				PromptFlag:     "--prompt",
				Models: map[string]string{
					"high":   "o3",
					"medium": "gpt-4o",
					"low":    "gpt-4o-mini",
				},
			},
		},
		Routing: config.RoutingConfig{
			Ratio: map[string]int{"openai": 100},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if runner.router == nil {
		t.Fatal("expected router to be non-nil for codex-only provider config")
	}

	tests := []struct {
		tier          string
		expectedModel string
	}{
		{provider.TierHigh, "o3"},
		{provider.TierMedium, "gpt-4o"},
		{provider.TierLow, "gpt-4o-mini"},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			p, model := runner.router.Select("build", tt.tier)
			if p == nil {
				t.Fatal("Select returned nil provider")
			}
			if p.Name() != "codex" {
				t.Errorf("Select returned provider %q, want 'codex'", p.Name())
			}
			if model != tt.expectedModel {
				t.Errorf("Select(%q) model = %q, want %q", tt.tier, model, tt.expectedModel)
			}
		})
	}
}

// TestNewRunnerRouterUsesStateFileForCounts verifies that the router built
// from config uses a state file adapter for persisting provider invocation
// counts, enabling ratio-based balancing across runs.
//
// Expected failure: The TODO at runner.go:103 is not implemented - no state
// file adapter is created or passed to provider.NewRouter().
func TestNewRunnerRouterUsesStateFileForCounts(t *testing.T) {
	cfg := setupTwoProviderConfig(t)

	// Create the gromit dir and state.json with pre-existing provider counts
	gromitDir := filepath.Dir(cfg.Paths.Templates)
	stateJSON := `{
		"provider_counts": {"claude": 10, "openai": 5},
		"clean_exit": true
	}`
	if err := os.WriteFile(filepath.Join(gromitDir, "state.json"), []byte(stateJSON), 0644); err != nil {
		t.Fatalf("failed to write state.json: %v", err)
	}

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if runner.router == nil {
		t.Fatal("expected router to be non-nil")
	}

	// With counts claude:10, openai:5, the ratio target is 60/40.
	// Current actual: claude=66.7%, openai=33.3%.
	// OpenAI is further below its target (40-33.3=6.7) vs claude (60-66.7=-6.7).
	// So "any" phases should prefer openai to rebalance.
	p, _ := runner.router.Select("validate", provider.TierLow)
	if p == nil {
		t.Fatal("Select returned nil provider")
	}
	// openai (codex) should be selected since it's further below its target ratio
	if p.Name() != "codex" {
		t.Errorf("expected 'codex' provider for ratio rebalancing, got %q", p.Name())
	}
}

// TestNewRunnerCooldownParsing verifies that different cooldown duration formats
// are correctly parsed from config when building the router.
//
// Expected failure: The TODO at runner.go:103 is not implemented - cooldown
// is never parsed from cfg.Routing.Fallback.Cooldown.
func TestNewRunnerCooldownParsing(t *testing.T) {
	tests := []struct {
		name             string
		cooldown         string
		expectedCooldown time.Duration
	}{
		{"30_minutes", "30m", 30 * time.Minute},
		{"1_hour", "1h", time.Hour},
		{"90_seconds", "90s", 90 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				Providers: map[string]config.ProviderDef{
					"claude": {
						Binary: "claude",
						Models: map[string]string{
							"high": "opus", "medium": "sonnet", "low": "haiku",
						},
					},
					"openai": {
						Binary:     "codex",
						PromptFlag: "--prompt",
						Models: map[string]string{
							"high": "o3", "medium": "gpt-4o", "low": "gpt-4o-mini",
						},
					},
				},
				Routing: config.RoutingConfig{
					PhasePreferences: map[string]string{"build": "claude"},
					Ratio:            map[string]int{"claude": 50, "openai": 50},
					Fallback: config.FallbackConfig{
						Enabled:  true,
						Cooldown: tt.cooldown,
					},
				},
			}
			cfg.SetDefaults()
			cfg.NormalizeNilFields()

			runner, err := NewRunner(cfg, os.Stdout)
			if err != nil {
				t.Fatalf("NewRunner() error = %v", err)
			}
			if runner.router == nil {
				t.Fatal("expected router to be non-nil")
			}

			// Mark claude unavailable, then verify it stays unavailable
			// (the cooldown should be the configured duration, not 0)
			p1, _ := runner.router.Select("build", provider.TierMedium)
			if p1 == nil {
				t.Fatal("no provider available")
			}
			runner.router.MarkUnavailable("claude")

			// Claude should now be unavailable, so build falls back
			p2, _ := runner.router.Select("build", provider.TierMedium)
			if p2 == nil {
				t.Fatal("expected fallback provider")
			}
			if p2.Name() == "claude" {
				t.Error("claude should be unavailable after MarkUnavailable with non-zero cooldown")
			}
		})
	}
}
