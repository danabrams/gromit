package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// TestBuildRouterReclassified_PhasePreferenceRouting verifies that when
// NewRunner builds a multi-provider Router from config, phase preferences
// are respected so each phase selects the configured preferred provider.
// This is the unit-level reclassification of the acceptance-level phase
// preference routing check.
func TestBuildRouterReclassified_PhasePreferenceRouting(t *testing.T) {
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
		Claude: config.ClaudeConfig{Binary: "claude"},
	}

	r, err := NewRunner(cfg, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if r.router == nil {
		t.Fatal("expected router to be non-nil when providers are configured")
	}

	// Build phase should route to claude per phase preferences.
	buildProvider, _ := r.router.Select("build", provider.TierMedium)
	if buildProvider == nil {
		t.Fatal("Select('build', medium) returned nil provider")
	}
	if buildProvider.Name() != "claude" {
		t.Errorf("build phase: want provider %q, got %q", "claude", buildProvider.Name())
	}

	// Validate phase should route to codex per phase preferences.
	valProvider, _ := r.router.Select("validate", provider.TierLow)
	if valProvider == nil {
		t.Fatal("Select('validate', low) returned nil provider")
	}
	if valProvider.Name() != "codex" {
		t.Errorf("validate phase: want provider %q, got %q", "codex", valProvider.Name())
	}
}

// TestBuildRouterReclassified_CooldownParsing verifies that the cooldown
// duration from cfg.Routing.Fallback.Cooldown is parsed and applied to the
// Router so that when a provider is marked unavailable, fallback to another
// provider occurs. This is the unit-level reclassification of the acceptance-
// level cooldown parsing check.
func TestBuildRouterReclassified_CooldownParsing(t *testing.T) {
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
				Enabled:  boolPtr(true),
				Cooldown: "1h",
			},
		},
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
		},
		Claude: config.ClaudeConfig{Binary: "claude"},
	}

	r, err := NewRunner(cfg, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if r.router == nil {
		t.Fatal("expected router to be non-nil when providers are configured")
	}

	// Mark claude as unavailable; the router should fall back to codex.
	r.router.MarkUnavailable("claude")

	p, _ := r.router.Select("build", provider.TierMedium)
	if p == nil {
		t.Fatal("Select after MarkUnavailable returned nil; expected fallback provider")
	}
	if p.Name() != "codex" {
		t.Errorf("expected fallback to codex when claude is unavailable, got %q", p.Name())
	}
}
