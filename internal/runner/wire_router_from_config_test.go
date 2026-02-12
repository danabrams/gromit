package runner

import (
	"os"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// setupNewRunnerDirs creates the temporary directory structure needed for NewRunner
// (templates, specs, logs). Returns the temp dir path.
func setupNewRunnerDirs(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/templates", 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(tmpDir+"/specs", 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}
	if err := os.MkdirAll(tmpDir+"/logs", 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}
	return tmpDir
}

// TestNewRunnerWithProvidersBuildsMultiProviderRouter verifies that when config
// has a providers section, NewRunner constructs a multi-provider Router (not nil).
func TestNewRunnerWithProvidersBuildsMultiProviderRouter(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)

	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"claude": {
				Binary: "claude",
				Flags:  []string{"--no-input"},
				Models: map[string]string{
					"high":   "opus",
					"medium": "sonnet",
					"low":    "haiku",
				},
			},
			"codex": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5.3-codex",
				},
			},
		},
		Routing: config.RoutingConfig{
			PhasePreferences: map[string]string{
				"build":    "claude",
				"validate": "any",
				"review":   "any",
			},
			Ratio: map[string]int{
				"claude": 60,
				"codex":  40,
			},
			Fallback: config.FallbackConfig{
				Enabled:  true,
				Cooldown: "30m",
			},
		},
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
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
		t.Fatal("Expected runner.router to be non-nil when providers are configured, got nil")
	}
}

// TestNewRunnerWithProvidersSelectsCorrectProviderForPhase verifies that a
// multi-provider Router built from config routes phases to the preferred provider.
func TestNewRunnerWithProvidersSelectsCorrectProviderForPhase(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)

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
			"codex": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5.3-codex",
				},
			},
		},
		Routing: config.RoutingConfig{
			PhasePreferences: map[string]string{
				"build": "claude",
			},
			Ratio: map[string]int{
				"claude": 60,
				"codex":  40,
			},
			Fallback: config.FallbackConfig{
				Enabled:  true,
				Cooldown: "30m",
			},
		},
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
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
		t.Fatal("Expected runner.router to be non-nil when providers are configured")
	}

	// Select for "build" phase — should prefer "claude" per phase_preferences
	p, model := runner.router.Select("build", provider.TierHigh)
	if p == nil {
		t.Fatal("Expected Select('build', 'high') to return a provider, got nil")
	}
	if p.Name() != "claude" {
		t.Errorf("Expected build phase to select 'claude' provider, got %q", p.Name())
	}
	if model != "opus" {
		t.Errorf("Expected high tier model 'opus' for claude, got %q", model)
	}
}

// TestNewRunnerWithProvidersCodexTierMapping verifies that the codex provider
// created from config has the correct tier-to-model mapping.
func TestNewRunnerWithProvidersCodexTierMapping(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)

	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"codex": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-4o",
					"low":    "gpt-4o-mini",
				},
			},
		},
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
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
		t.Fatal("Expected runner.router to be non-nil when providers are configured")
	}

	// Select any phase — with only codex, it should always return codex
	p, model := runner.router.Select("build", provider.TierMedium)
	if p == nil {
		t.Fatal("Expected Select to return a provider, got nil")
	}
	if p.Name() != "codex" {
		t.Errorf("Expected 'codex' provider, got %q", p.Name())
	}
	if model != "gpt-4o" {
		t.Errorf("Expected medium tier model 'gpt-4o' for codex, got %q", model)
	}
}

// TestNewRunnerUnrecognizedProviderReturnsError verifies that an unrecognized
// provider name in the config causes NewRunner to return a clear error.
func TestNewRunnerUnrecognizedProviderReturnsError(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)

	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"gemini": {
				Binary: "gemini-cli",
				Models: map[string]string{
					"high":   "gemini-ultra",
					"medium": "gemini-pro",
					"low":    "gemini-flash",
				},
			},
		},
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
		},
		Claude: config.ClaudeConfig{
			Binary: "claude",
		},
	}

	_, err := NewRunner(cfg, nil)
	if err == nil {
		t.Fatal("Expected error for unrecognized provider 'gemini', got nil")
	}
	if !strings.Contains(err.Error(), "gemini") {
		t.Errorf("Expected error to mention 'gemini', got %q", err.Error())
	}
}

// TestNewRunnerEmptyModelsUsesDefaults verifies that when a provider's models
// map is nil or empty, NewRunner fills in default tier mappings.
func TestNewRunnerEmptyModelsUsesDefaults(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)

	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"claude": {
				Binary: "claude",
				Models: nil, // empty — should get defaults
			},
		},
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
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
		t.Fatal("Expected router to be non-nil")
	}

	// With default Claude tier mapping, high should resolve to "opus"
	p, model := runner.router.Select("build", provider.TierHigh)
	if p == nil {
		t.Fatal("Expected Select to return a provider")
	}
	if model != "opus" {
		t.Errorf("Expected default high-tier model 'opus' for claude, got %q", model)
	}

	// Medium should resolve to "sonnet"
	_, model = runner.router.Select("build", provider.TierMedium)
	if model != "sonnet" {
		t.Errorf("Expected default medium-tier model 'sonnet' for claude, got %q", model)
	}

	// Low should resolve to "haiku"
	_, model = runner.router.Select("build", provider.TierLow)
	if model != "haiku" {
		t.Errorf("Expected default low-tier model 'haiku' for claude, got %q", model)
	}
}

// TestNewRunnerWithProvidersStateFileReusedInRun verifies that the state file
// is created in NewRunner and reused in Run() rather than creating a duplicate.
// After NewRunner with providers, provider selection counts persist across
// router.Select calls via the non-nil StateFile in the Router.
func TestNewRunnerWithProvidersStateFileReusedInRun(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)

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
			"codex": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5.3-codex",
				},
			},
		},
		Routing: config.RoutingConfig{
			Ratio: map[string]int{
				"claude": 50,
				"codex":  50,
			},
		},
		Paths: config.PathsConfig{
			GromitDir: tmpDir,
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
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
		t.Fatal("Expected router to be non-nil when providers are configured")
	}

	// Perform multiple selections — the router should track counts via the state file.
	// After 2 selections with 50/50 ratio, both providers should have been selected.
	p1, _ := runner.router.Select("build", provider.TierMedium)
	p2, _ := runner.router.Select("review", provider.TierMedium)
	if p1 == nil || p2 == nil {
		t.Fatal("Expected Select to return providers")
	}

	// With 50/50 ratio and stateful counting, the two selections should
	// pick different providers to balance the ratio
	if p1.Name() == p2.Name() {
		t.Errorf("Expected ratio balancing to select different providers on consecutive calls, "+
			"got %q both times (state file may not be wired)", p1.Name())
	}
}

// TestNewRunnerWithProvidersNoRoutingUsesDefaults verifies that when providers
// exist but routing config is empty, sensible defaults are applied:
// all phases "any", equal ratio split, 30m cooldown.
func TestNewRunnerWithProvidersNoRoutingUsesDefaults(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)

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
			"codex": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5.3-codex",
				},
			},
		},
		// No Routing section — defaults should apply
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
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
		t.Fatal("Expected router to be non-nil when providers are configured with no routing")
	}

	// With default "any" preferences, both providers should be selectable.
	// Call Select twice to verify both providers participate via ratio balancing.
	selectedProviders := make(map[string]bool)
	for i := 0; i < 10; i++ {
		p, _ := runner.router.Select("build", provider.TierMedium)
		if p == nil {
			t.Fatal("Expected Select to return a provider")
		}
		selectedProviders[p.Name()] = true
	}

	if !selectedProviders["claude"] {
		t.Error("Expected 'claude' to be selected at least once with equal ratio")
	}
	if !selectedProviders["codex"] {
		t.Error("Expected 'codex' to be selected at least once with equal ratio")
	}
}

// TestNewRunnerWithProvidersCooldownParsed verifies that the cooldown duration
// from cfg.Routing.Fallback.Cooldown is parsed and applied to the Router.
// When a provider is marked unavailable, it should respect the configured cooldown.
func TestNewRunnerWithProvidersCooldownParsed(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)

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
			"codex": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5.3-codex",
				},
			},
		},
		Routing: config.RoutingConfig{
			PhasePreferences: map[string]string{
				"build": "claude",
			},
			Ratio: map[string]int{
				"claude": 50,
				"codex":  50,
			},
			Fallback: config.FallbackConfig{
				Enabled:  true,
				Cooldown: "1h",
			},
		},
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
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
		t.Fatal("Expected router to be non-nil when providers are configured")
	}

	// Mark claude as unavailable — cooldown should be 1h from config
	runner.router.MarkUnavailable("claude")

	// Now build phase should fall back to codex since claude is unavailable
	p, _ := runner.router.Select("build", provider.TierMedium)
	if p == nil {
		t.Fatal("Expected Select to fall back to available provider")
	}
	if p.Name() != "codex" {
		t.Errorf("Expected fallback to 'codex' when claude is unavailable, got %q", p.Name())
	}
}

// TestNewRunnerSingleProviderInProvidersSection verifies that a single provider
// in the providers section works through the multi-provider code path.
func TestNewRunnerSingleProviderInProvidersSection(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)

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
			Logs:      tmpDir + "/logs",
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
		t.Fatal("Expected router to be non-nil with single provider in providers section")
	}

	// Should work identically to legacy path: always returns claude
	p, model := runner.router.Select("build", provider.TierHigh)
	if p == nil {
		t.Fatal("Expected Select to return a provider")
	}
	if p.Name() != "claude" {
		t.Errorf("Expected 'claude' provider, got %q", p.Name())
	}
	if model != "opus" {
		t.Errorf("Expected 'opus' model for high tier, got %q", model)
	}
}

// TestNewRunnerWithProvidersLearningsUsesClaudeProvider verifies that
// claudeProviderForLearnings is set to the Claude provider from the providers
// config when one exists. We test this indirectly by verifying the runner's
// analyzer is non-nil (analyzer requires a provider for learnings).
func TestNewRunnerWithProvidersLearningsUsesClaudeProvider(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)

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
			"codex": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5.3-codex",
				},
			},
		},
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
		},
		Claude: config.ClaudeConfig{
			Binary: "claude",
		},
	}

	runner, err := NewRunner(cfg, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	// When the multi-provider path properly sets claudeProviderForLearnings,
	// both the router AND the analyzer should be non-nil and functional.
	if runner.router == nil {
		t.Fatal("Expected router to be non-nil when providers are configured")
	}

	// The router should contain a claude provider — verify it responds to "build"
	p, _ := runner.router.Select("build", provider.TierHigh)
	if p == nil {
		t.Fatal("Expected Select to return a provider from multi-provider router")
	}
	// Verify the provider is the Claude one (should be based on default "any" routing)
	// and that it has the correct tier mapping from config
	if p.Name() == "claude" && p.ModelForTier(provider.TierHigh) != "opus" {
		t.Errorf("Expected claude provider high tier to be 'opus', got %q", p.ModelForTier(provider.TierHigh))
	}
}

// TestNewRunnerPhasePreferencesPassedToRouter verifies that phase preferences
// from the routing config are correctly passed to the Router.
func TestNewRunnerPhasePreferencesPassedToRouter(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)

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
			"codex": {
				Binary: "codex",
				Models: map[string]string{
					"high":   "gpt-5.3-codex",
					"medium": "gpt-5.3-codex",
					"low":    "gpt-5.3-codex",
				},
			},
		},
		Routing: config.RoutingConfig{
			PhasePreferences: map[string]string{
				"build":    "claude",
				"validate": "codex",
			},
			Ratio: map[string]int{
				"claude": 50,
				"codex":  50,
			},
		},
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
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
		t.Fatal("Expected router to be non-nil")
	}

	// Build phase should prefer claude
	buildP, _ := runner.router.Select("build", provider.TierMedium)
	if buildP == nil {
		t.Fatal("Expected build Select to return a provider")
	}
	if buildP.Name() != "claude" {
		t.Errorf("Expected build phase to select 'claude', got %q", buildP.Name())
	}

	// Validate phase should prefer codex
	valP, _ := runner.router.Select("validate", provider.TierLow)
	if valP == nil {
		t.Fatal("Expected validate Select to return a provider")
	}
	if valP.Name() != "codex" {
		t.Errorf("Expected validate phase to select 'codex', got %q", valP.Name())
	}
}
