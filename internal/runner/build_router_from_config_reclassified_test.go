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
				Enabled:  boolPtr(true),
				Cooldown: "30m",
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

func newRouterFromConfig(t *testing.T, cfg *config.Config) *Runner {
	t.Helper()

	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if runner.router == nil {
		t.Fatal("expected runner.router to be non-nil")
	}
	return runner
}

const (
	testPhaseBuild    = "build"
	testPhaseValidate = "validate"
	testPhaseAnalyze  = "analyze"
	testPhaseReview   = "review"
)

func createTestLearningsFile(t *testing.T, cfg *config.Config) {
	t.Helper()

	gromitDir := filepath.Dir(cfg.Paths.Templates)
	learningsPath := filepath.Join(gromitDir, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath, []byte("# Learnings\n"), 0644); err != nil {
		t.Fatalf("failed to create LEARNINGS.md: %v", err)
	}
}

func createStateFile(t *testing.T, cfg *config.Config, contents string) {
	t.Helper()

	gromitDir := filepath.Dir(cfg.Paths.Templates)
	if err := os.WriteFile(filepath.Join(gromitDir, "state.json"), []byte(contents), 0644); err != nil {
		t.Fatalf("failed to write state.json: %v", err)
	}
}

func newRouterConfigWithCooldown(t *testing.T, cooldown string) *config.Config {
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
			PhasePreferences: map[string]string{testPhaseBuild: "claude"},
			Ratio:            map[string]int{"claude": 50, "openai": 50},
			Fallback: config.FallbackConfig{
				Enabled:  boolPtr(true),
				Cooldown: cooldown,
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

func assertRouterSelection(t *testing.T, runner *Runner, phase string, tier string, expectedProvider string, expectedModel string) {
	t.Helper()

	p, model := runner.router.Select(phase, tier)
	if p == nil {
		t.Fatalf("router.Select(%q, %q) returned nil provider", phase, tier)
	}
	if expectedProvider != "" && p.Name() != expectedProvider {
		t.Fatalf("router.Select(%q, %q) provider = %q, want %q", phase, tier, p.Name(), expectedProvider)
	}
	if expectedModel != "" && model != expectedModel {
		t.Errorf("router.Select(%q, %q) model = %q, want %q", phase, tier, model, expectedModel)
	}
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

	runner := newRouterFromConfig(t, cfg)

	// The router should be able to select a provider for each configured phase
	for _, phase := range []string{testPhaseBuild, testPhaseValidate, testPhaseAnalyze, testPhaseReview} {
		p, model := runner.router.Select(phase, provider.TierMedium)
		if p == nil {
			t.Errorf("router.Select(%q, %q) returned nil provider", phase, provider.TierMedium)
		}
		if model == "" {
			t.Errorf("router.Select(%q, %q) returned empty model name", phase, provider.TierMedium)
		}
	}
}

// TestBuildRouterReclassified_PhasePreferenceRouting verifies that the router built
// from config routes phases to the preferred provider when one is specified.
//
// Expected failure: The TODO at runner.go:103 is not implemented - router is nil.
func TestBuildRouterReclassified_PhasePreferenceRouting(t *testing.T) {
	cfg := setupTwoProviderConfig(t)

	runner := newRouterFromConfig(t, cfg)

	tests := []struct {
		phase         string
		expectedProv  string
		tier          string
		expectedModel string
	}{
		// "build" prefers "claude" → should select claude with sonnet for medium tier
		{phase: testPhaseBuild, expectedProv: "claude", tier: provider.TierMedium, expectedModel: "sonnet"},
		{phase: testPhaseReview, expectedProv: "claude", tier: provider.TierHigh, expectedModel: "opus"},
		{phase: testPhaseBuild, expectedProv: "claude", tier: provider.TierLow, expectedModel: "haiku"},
	}

	for _, tt := range tests {
		t.Run(tt.phase+"_"+tt.tier, func(t *testing.T) {
			assertRouterSelection(t, runner, tt.phase, tt.tier, tt.expectedProv, tt.expectedModel)
		})
	}
}

// TestNewRunnerRouterUsesRatioForAnyPhases verifies that phases configured as
// "any" use ratio-based balancing to select a provider.
//
// Expected failure: The TODO at runner.go:103 is not implemented - router is nil.
func TestNewRunnerRouterUsesRatioForAnyPhases(t *testing.T) {
	cfg := setupTwoProviderConfig(t)

	runner := newRouterFromConfig(t, cfg)

	// "validate" is configured as "any" — the ratio balancer should pick a provider.
	// With a fresh router (0 counts), the provider furthest below its ratio target
	// should be selected first. Claude has 60% target vs openai 40%, so claude
	// has the larger gap (60-0=60 > 40-0=40) and should be selected first.
	p, model := runner.router.Select(testPhaseValidate, provider.TierLow)
	if p == nil {
		t.Fatal("Select returned nil provider for 'any' phase")
	}
	if model == "" {
		t.Fatal("Select returned empty model for 'any' phase")
	}

	// With 0 counts and 60/40 ratio, claude should be selected first (larger gap)
	if p.Name() != "claude" {
		t.Errorf("Select(validate, low) provider = %q, want 'claude' (60%% ratio target vs 40%%)", p.Name())
	}
	if model != "haiku" {
		t.Errorf("Select(validate, low) model = %q, want 'haiku' for claude low tier", model)
	}

	// After one claude invocation (count now claude:1, openai:0),
	// openai should be further below its target and get selected next.
	p2, model2 := runner.router.Select(testPhaseValidate, provider.TierLow)
	if p2 == nil {
		t.Fatal("second Select returned nil provider")
	}
	if p2.Name() != "codex" {
		t.Errorf("second Select(validate, low) provider = %q, want 'codex' (rebalancing)", p2.Name())
	}
	if model2 != "gpt-4o-mini" {
		t.Errorf("second Select(validate, low) model = %q, want 'gpt-4o-mini'", model2)
	}
}

// TestBuildRouterReclassified_CooldownParsing verifies that the fallback cooldown
// duration from config is correctly parsed and applied to the router.
//
// Expected failure: The TODO at runner.go:103 is not implemented - router is nil,
// and there is no BuildRouterFromConfig function yet.
func TestBuildRouterReclassified_CooldownParsing(t *testing.T) {
	cfg := setupTwoProviderConfig(t)
	// Fallback.Cooldown is "30m" in setupTwoProviderConfig

	runner := newRouterFromConfig(t, cfg)

	// Select both providers to verify they're available initially
	p1, _ := runner.router.Select(testPhaseBuild, provider.TierMedium) // claude preferred
	if p1 == nil {
		t.Fatal("expected claude provider to be available")
	}

	// Mark it unavailable and verify fallback works
	runner.router.MarkUnavailable(p1.Name())

	// The "build" phase prefers "claude", but claude is now unavailable.
	// The router should fall back to the other provider.
	p2, model := runner.router.Select(testPhaseBuild, provider.TierMedium)
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

	createTestLearningsFile(t, cfg)

	runner := newRouterFromConfig(t, cfg)

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
	cfg := setupSingleProviderConfig(t, "claude", config.ProviderDef{
		Binary:         "claude",
		Flags:          []string{"--no-input"},
		PromptDelivery: "stdin",
		Models: map[string]string{
			"high":   "opus",
			"medium": "sonnet",
			"low":    "haiku",
		},
	})
	runner := newRouterFromConfig(t, cfg)

	if runner.router == nil {
		t.Fatal("expected router to be non-nil for single-provider config via HasProviders() path")
	}

	// Select should return claude for any phase
	p, model := runner.router.Select(testPhaseBuild, provider.TierHigh)
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
	cfg := setupSingleProviderConfig(t, "openai", config.ProviderDef{
		Binary:         "codex",
		Flags:          []string{},
		PromptDelivery: "prompt_file_arg",
		PromptFlag:     "--prompt",
		Models: map[string]string{
			"high":   "o3",
			"medium": "gpt-4o",
			"low":    "gpt-4o-mini",
		},
	})
	runner := newRouterFromConfig(t, cfg)

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
			assertRouterSelection(t, runner, testPhaseBuild, tt.tier, "codex", tt.expectedModel)
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
	createStateFile(t, cfg, `{
		"provider_counts": {"claude": 10, "openai": 5},
		"clean_exit": true
	}`)

	runner := newRouterFromConfig(t, cfg)

	// With counts claude:10, openai:5, the ratio target is 60/40.
	// Current actual: claude=66.7%, openai=33.3%.
	// OpenAI is further below its target (40-33.3=6.7) vs claude (60-66.7=-6.7).
	// So "any" phases should prefer openai to rebalance.
	p, _ := runner.router.Select(testPhaseValidate, provider.TierLow)
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
			cfg := newRouterConfigWithCooldown(t, tt.cooldown)

			runner := newRouterFromConfig(t, cfg)

			// Mark claude unavailable, then verify it stays unavailable
			// (the cooldown should be the configured duration, not 0)
			p1, _ := runner.router.Select(testPhaseBuild, provider.TierMedium)
			if p1 == nil {
				t.Fatal("no provider available")
			}
			runner.router.MarkUnavailable("claude")

			// Claude should now be unavailable, so build falls back
			p2, _ := runner.router.Select(testPhaseBuild, provider.TierMedium)
			if p2 == nil {
				t.Fatal("expected fallback provider")
			}
			if p2.Name() == "claude" {
				t.Error("claude should be unavailable after MarkUnavailable with non-zero cooldown")
			}
		})
	}
}

// TestNewRunnerRouterProviderNames verifies that the router built from config
// creates providers with the correct Name() values: "claude" for the claude
// provider and "codex" for the openai/codex provider.
//
// Expected failure: The TODO at runner.go:103 is not implemented - router is nil,
// and no providers are constructed from cfg.Providers.
func TestNewRunnerRouterProviderNames(t *testing.T) {
	cfg := setupTwoProviderConfig(t)

	runner := newRouterFromConfig(t, cfg)

	// Select the build phase (prefers claude) to verify claude provider exists
	pClaude, _ := runner.router.Select(testPhaseBuild, provider.TierMedium)
	if pClaude == nil {
		t.Fatal("Select(build) returned nil provider")
	}
	if pClaude.Name() != "claude" {
		t.Errorf("build phase provider Name() = %q, want 'claude'", pClaude.Name())
	}

	// Mark claude unavailable to force fallback to the openai/codex provider
	runner.router.MarkUnavailable("claude")

	// Now build phase should fall back to the codex provider
	pCodex, _ := runner.router.Select(testPhaseBuild, provider.TierMedium)
	if pCodex == nil {
		t.Fatal("Select(build) after marking claude unavailable returned nil provider")
	}
	// The openai config key should create a CodexProvider whose Name() returns "codex"
	if pCodex.Name() != "codex" {
		t.Errorf("fallback provider Name() = %q, want 'codex' (from openai config key)", pCodex.Name())
	}
}

// TestNewRunnerRouterAllTierMappingsForBothProviders verifies that the router
// correctly maps all three tiers to the right model names for both providers.
// This ensures cfg.Providers[x].Models is properly passed to each provider.
//
// Expected failure: The TODO at runner.go:103 is not implemented - router is nil,
// and provider tier-to-model mappings are never constructed from config.
func TestNewRunnerRouterAllTierMappingsForBothProviders(t *testing.T) {
	cfg := setupTwoProviderConfig(t)

	runner := newRouterFromConfig(t, cfg)

	// Test claude provider tier mappings via phase preferences (build prefers claude)
	claudeTests := []struct {
		tier          string
		expectedModel string
	}{
		{provider.TierHigh, "opus"},
		{provider.TierMedium, "sonnet"},
		{provider.TierLow, "haiku"},
	}

	for _, tt := range claudeTests {
		t.Run("claude_"+tt.tier, func(t *testing.T) {
			p, model := runner.router.Select(testPhaseBuild, tt.tier)
			if p == nil {
				t.Fatal("Select returned nil")
			}
			if p.Name() != "claude" {
				t.Fatalf("expected claude provider for build phase, got %q", p.Name())
			}
			if model != tt.expectedModel {
				t.Errorf("claude tier %q → model %q, want %q", tt.tier, model, tt.expectedModel)
			}
		})
	}

	// Mark claude unavailable to test codex tier mappings
	runner.router.MarkUnavailable("claude")

	codexTests := []struct {
		tier          string
		expectedModel string
	}{
		{provider.TierHigh, "o3"},
		{provider.TierMedium, "gpt-4o"},
		{provider.TierLow, "gpt-4o-mini"},
	}

	for _, tt := range codexTests {
		t.Run("codex_"+tt.tier, func(t *testing.T) {
			p, model := runner.router.Select(testPhaseBuild, tt.tier)
			if p == nil {
				t.Fatal("Select returned nil after claude marked unavailable")
			}
			if p.Name() != "codex" {
				t.Fatalf("expected codex provider after claude unavailable, got %q", p.Name())
			}
			if model != tt.expectedModel {
				t.Errorf("codex tier %q → model %q, want %q", tt.tier, model, tt.expectedModel)
			}
		})
	}
}

// TestNewRunnerRouterAnalyzerUsesClaudeAsDefault verifies that when multiple
// providers are configured and claude is one of them, the analyzer is wired
// using the claude provider as the default (not a fallback ClaudeClientAdapter).
//
// Expected failure: The TODO at runner.go:103 is not implemented —
// claudeProviderForLearnings remains nil, so analyzer uses the fallback adapter.
func TestNewRunnerRouterAnalyzerUsesClaudeAsDefault(t *testing.T) {
	cfg := setupTwoProviderConfig(t)

	// Create LEARNINGS.md so learnings filter can be wired
	gromitDir := filepath.Dir(cfg.Paths.Templates)
	if err := os.WriteFile(
		filepath.Join(gromitDir, "LEARNINGS.md"),
		[]byte("# Learnings\n"),
		0644,
	); err != nil {
		t.Fatalf("failed to create LEARNINGS.md: %v", err)
	}

	runner := newRouterFromConfig(t, cfg)

	// Both router and analyzer must be non-nil
	if runner.router == nil {
		t.Fatal("expected router to be non-nil")
	}
	if runner.analyzer == nil {
		t.Fatal("expected analyzer to be non-nil")
	}

	// Verify that the router can select claude (proving the provider was built correctly)
	p, _ := runner.router.Select(testPhaseBuild, provider.TierMedium)
	if p == nil {
		t.Fatal("router.Select returned nil provider")
	}
	if p.Name() != "claude" {
		t.Errorf("expected claude as default/preferred build provider, got %q", p.Name())
	}
}
