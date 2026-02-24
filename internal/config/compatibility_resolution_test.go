package config

import "testing"

func TestResolveCompatibilityContextLegacyFallbackDefaults(t *testing.T) {
	var cfg Config

	resolved := cfg.ResolveCompatibilityContext()

	if resolved.Profile.Value != "go" {
		t.Fatalf("Profile.Value = %q, want %q", resolved.Profile.Value, "go")
	}
	if resolved.Profile.Source != CompatibilitySourceLegacyFallback {
		t.Fatalf("Profile.Source = %q, want %q", resolved.Profile.Source, CompatibilitySourceLegacyFallback)
	}
	if resolved.TrackerBackend.Value != "bd" {
		t.Fatalf("TrackerBackend.Value = %q, want %q", resolved.TrackerBackend.Value, "bd")
	}
	if resolved.TrackerBackend.Source != CompatibilitySourceLegacyFallback {
		t.Fatalf("TrackerBackend.Source = %q, want %q", resolved.TrackerBackend.Source, CompatibilitySourceLegacyFallback)
	}
	if resolved.MethodologyAdapter.Value != "go" {
		t.Fatalf("MethodologyAdapter.Value = %q, want %q", resolved.MethodologyAdapter.Value, "go")
	}
	if resolved.MethodologyAdapter.Source != CompatibilitySourceLegacyFallback {
		t.Fatalf("MethodologyAdapter.Source = %q, want %q", resolved.MethodologyAdapter.Source, CompatibilitySourceLegacyFallback)
	}
}
