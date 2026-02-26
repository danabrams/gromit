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

func TestLoadCompatibilityFixtureMixedLegacyAndNewSelectors_ProfileFallbackDocumented(t *testing.T) {
	cfgPath := filepath.Join("..", "..", "test", "fixtures", "migration", "mixed_selectors.yaml")
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%q) error = %v", cfgPath, err)
	}

	if cfg.Project.Profile != "" {
		t.Fatalf("mixed_selectors fixture should omit project.profile to rely on legacy fallback, got %q", cfg.Project.Profile)
	}

	resolved := cfg.ResolveCompatibilityContext()

	if resolved.Profile.Value != "go" || resolved.Profile.Source != CompatibilitySourceLegacyFallback {
		t.Fatalf("mixed_selectors fixture profile resolved = (%q, %q), want (%q, %q) because project.profile is missing", resolved.Profile.Value, resolved.Profile.Source, "go", CompatibilitySourceLegacyFallback)
	}
	if resolved.Profile.DeprecationMarker != CompatibilityDeprecationMarkerLegacyHardcodedDefaults {
		t.Fatalf("mixed_selectors fixture profile marker = %q, want %q to document legacy hardcoded defaults", resolved.Profile.DeprecationMarker, CompatibilityDeprecationMarkerLegacyHardcodedDefaults)
	}
	if resolved.MethodologyAdapter.Source != CompatibilitySourceLegacyFallback {
		t.Fatalf("mixed_selectors fixture methodology adapter source = %q, want %q to confirm implicit legacy go defaults", resolved.MethodologyAdapter.Source, CompatibilitySourceLegacyFallback)
	}
	if resolved.MethodologyAdapter.DeprecationMarker != CompatibilityDeprecationMarkerLegacyHardcodedDefaults {
		t.Fatalf("mixed_selectors fixture methodology adapter marker = %q, want %q", resolved.MethodologyAdapter.DeprecationMarker, CompatibilityDeprecationMarkerLegacyHardcodedDefaults)
	}
	if resolved.TrackerBackend.Source != CompatibilitySourceExplicit {
		t.Fatalf("mixed_selectors fixture tracker backend source = %q, want %q since backend is explicitly declared", resolved.TrackerBackend.Source, CompatibilitySourceExplicit)
	}
	if resolved.TrackerBackend.DeprecationMarker != "" {
		t.Fatalf("mixed_selectors fixture tracker backend marker = %q, want empty because backend is explicit in this fixture", resolved.TrackerBackend.DeprecationMarker)
	}
}
