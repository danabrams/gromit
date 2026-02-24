package config

import (
	"path/filepath"
	"testing"
)

func TestLoadCompatibilityFixtureMixedLegacyAndNewSelectors(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "test", "fixtures", "migration", "mixed_selectors.yaml")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%q) error = %v", cfgPath, err)
	}

	resolved := cfg.ResolveCompatibilityContext()
	if resolved.Profile.Value != "go" || resolved.Profile.Source != CompatibilitySourceLegacyFallback {
		t.Fatalf("profile resolved = (%q, %q), want (%q, %q)", resolved.Profile.Value, resolved.Profile.Source, "go", CompatibilitySourceLegacyFallback)
	}
	if resolved.TrackerBackend.Value != "bd" || resolved.TrackerBackend.Source != CompatibilitySourceExplicit {
		t.Fatalf("tracker backend resolved = (%q, %q), want (%q, %q)", resolved.TrackerBackend.Value, resolved.TrackerBackend.Source, "bd", CompatibilitySourceExplicit)
	}
	if resolved.MethodologyAdapter.Value != "go" || resolved.MethodologyAdapter.Source != CompatibilitySourceLegacyFallback {
		t.Fatalf("methodology adapter resolved = (%q, %q), want (%q, %q)", resolved.MethodologyAdapter.Value, resolved.MethodologyAdapter.Source, "go", CompatibilitySourceLegacyFallback)
	}
}
