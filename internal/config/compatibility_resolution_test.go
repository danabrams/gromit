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

func TestResolveCompatibilityContextUsesProfileDefaultsWhenProfileExplicit(t *testing.T) {
	cfg := Config{
		Project: ProjectConfig{
			Profile: "go",
		},
	}

	resolved := cfg.ResolveCompatibilityContext()

	if resolved.Profile.Source != CompatibilitySourceExplicit {
		t.Fatalf("Profile.Source = %q, want %q", resolved.Profile.Source, CompatibilitySourceExplicit)
	}
	if resolved.TrackerBackend.Value != "bd" {
		t.Fatalf("TrackerBackend.Value = %q, want %q", resolved.TrackerBackend.Value, "bd")
	}
	if resolved.TrackerBackend.Source != CompatibilitySourceProfileDefault {
		t.Fatalf("TrackerBackend.Source = %q, want %q", resolved.TrackerBackend.Source, CompatibilitySourceProfileDefault)
	}
	if resolved.MethodologyAdapter.Value != "go" {
		t.Fatalf("MethodologyAdapter.Value = %q, want %q", resolved.MethodologyAdapter.Value, "go")
	}
	if resolved.MethodologyAdapter.Source != CompatibilitySourceProfileDefault {
		t.Fatalf("MethodologyAdapter.Source = %q, want %q", resolved.MethodologyAdapter.Source, CompatibilitySourceProfileDefault)
	}
}

func TestResolveCompatibilityContextExplicitSelectorsOverrideProfileDefaults(t *testing.T) {
	cfg := Config{
		Project: ProjectConfig{
			Profile: "go",
		},
		Tracker: TrackerConfig{
			Backend: "linear",
		},
		Methodology: MethodologyConfig{
			Adapter: "python",
		},
	}

	resolved := cfg.ResolveCompatibilityContext()

	if resolved.TrackerBackend.Value != "linear" {
		t.Fatalf("TrackerBackend.Value = %q, want %q", resolved.TrackerBackend.Value, "linear")
	}
	if resolved.TrackerBackend.Source != CompatibilitySourceExplicit {
		t.Fatalf("TrackerBackend.Source = %q, want %q", resolved.TrackerBackend.Source, CompatibilitySourceExplicit)
	}
	if resolved.MethodologyAdapter.Value != "python" {
		t.Fatalf("MethodologyAdapter.Value = %q, want %q", resolved.MethodologyAdapter.Value, "python")
	}
	if resolved.MethodologyAdapter.Source != CompatibilitySourceExplicit {
		t.Fatalf("MethodologyAdapter.Source = %q, want %q", resolved.MethodologyAdapter.Source, CompatibilitySourceExplicit)
	}
}
