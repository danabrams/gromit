package config

import (
	"strings"
	"testing"
)

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

func TestValidateCompatibilityRejectsInvalidExplicitValuesOnly(t *testing.T) {
	t.Run("legacy empty config is valid", func(t *testing.T) {
		var cfg Config
		cfg.SetDefaults()
		cfg.NormalizeNilFields()
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	testCases := []struct {
		name        string
		cfg         Config
		wantErrPart string
	}{
		{
			name: "invalid project profile",
			cfg: Config{
				Project: ProjectConfig{
					Profile: "bad-profile",
				},
			},
			wantErrPart: "project.profile",
		},
		{
			name: "invalid tracker backend",
			cfg: Config{
				Tracker: TrackerConfig{
					Backend: "bad-backend",
				},
			},
			wantErrPart: "tracker.backend",
		},
		{
			name: "invalid methodology adapter",
			cfg: Config{
				Methodology: MethodologyConfig{
					Adapter: "bad-adapter",
				},
			},
			wantErrPart: "methodology.adapter",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.cfg.SetDefaults()
			tc.cfg.NormalizeNilFields()
			err := tc.cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tc.wantErrPart) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tc.wantErrPart)
			}
		})
	}
}

func TestResolvedMethodologyAdapterAccessorUsesLegacyFallback(t *testing.T) {
	var cfg Config

	resolved := cfg.ResolvedMethodologyAdapter()
	if resolved.Value != "go" {
		t.Fatalf("ResolvedMethodologyAdapter().Value = %q, want %q", resolved.Value, "go")
	}
	if resolved.Source != CompatibilitySourceLegacyFallback {
		t.Fatalf("ResolvedMethodologyAdapter().Source = %q, want %q", resolved.Source, CompatibilitySourceLegacyFallback)
	}
}

func TestResolvedTrackerBackendAccessorUsesProfileDefaultSource(t *testing.T) {
	cfg := Config{
		Project: ProjectConfig{
			Profile: "go",
		},
	}

	resolved := cfg.ResolvedTrackerBackend()
	if resolved.Value != "bd" {
		t.Fatalf("ResolvedTrackerBackend().Value = %q, want %q", resolved.Value, "bd")
	}
	if resolved.Source != CompatibilitySourceProfileDefault {
		t.Fatalf("ResolvedTrackerBackend().Source = %q, want %q", resolved.Source, CompatibilitySourceProfileDefault)
	}
}

func TestResolvedProfileAccessorUsesExplicitSource(t *testing.T) {
	cfg := Config{
		Project: ProjectConfig{
			Profile: "go",
		},
	}

	resolved := cfg.ResolvedProfile()
	if resolved.Value != "go" {
		t.Fatalf("ResolvedProfile().Value = %q, want %q", resolved.Value, "go")
	}
	if resolved.Source != CompatibilitySourceExplicit {
		t.Fatalf("ResolvedProfile().Source = %q, want %q", resolved.Source, CompatibilitySourceExplicit)
	}
}

func TestResolveCompatibilityContext_DeprecationMarkerTracksLegacyAssumptions(t *testing.T) {
	var legacy Config

	legacyResolved := legacy.ResolveCompatibilityContext()
	if legacyResolved.Profile.DeprecationMarker != CompatibilityDeprecationMarkerLegacyHardcodedDefaults {
		t.Fatalf("legacy Profile.DeprecationMarker = %q, want %q", legacyResolved.Profile.DeprecationMarker, CompatibilityDeprecationMarkerLegacyHardcodedDefaults)
	}
	if legacyResolved.TrackerBackend.DeprecationMarker != CompatibilityDeprecationMarkerLegacyHardcodedDefaults {
		t.Fatalf("legacy TrackerBackend.DeprecationMarker = %q, want %q", legacyResolved.TrackerBackend.DeprecationMarker, CompatibilityDeprecationMarkerLegacyHardcodedDefaults)
	}
	if legacyResolved.MethodologyAdapter.DeprecationMarker != CompatibilityDeprecationMarkerLegacyHardcodedDefaults {
		t.Fatalf("legacy MethodologyAdapter.DeprecationMarker = %q, want %q", legacyResolved.MethodologyAdapter.DeprecationMarker, CompatibilityDeprecationMarkerLegacyHardcodedDefaults)
	}

	explicit := Config{
		Project: ProjectConfig{Profile: "go"},
		Tracker: TrackerConfig{Backend: "bd"},
		Methodology: MethodologyConfig{
			Adapter: "go",
		},
	}
	explicitResolved := explicit.ResolveCompatibilityContext()
	if explicitResolved.Profile.DeprecationMarker != "" {
		t.Fatalf("explicit Profile.DeprecationMarker = %q, want empty", explicitResolved.Profile.DeprecationMarker)
	}
	if explicitResolved.TrackerBackend.DeprecationMarker != "" {
		t.Fatalf("explicit TrackerBackend.DeprecationMarker = %q, want empty", explicitResolved.TrackerBackend.DeprecationMarker)
	}
	if explicitResolved.MethodologyAdapter.DeprecationMarker != "" {
		t.Fatalf("explicit MethodologyAdapter.DeprecationMarker = %q, want empty", explicitResolved.MethodologyAdapter.DeprecationMarker)
	}
}
