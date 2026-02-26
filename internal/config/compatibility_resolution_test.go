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
	if legacyResolved.TrackerBackend.DeprecationMarker != CompatibilityDeprecationMarkerLegacyTrackerBackendFallback {
		t.Fatalf("legacy TrackerBackend.DeprecationMarker = %q, want %q", legacyResolved.TrackerBackend.DeprecationMarker, CompatibilityDeprecationMarkerLegacyTrackerBackendFallback)
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

func TestResolveCompatibilityContext_DeprecationMarkerTracksLegacyTrackerBackendFallback(t *testing.T) {
	cfg := Config{
		Project: ProjectConfig{
			Profile: "go",
		},
	}

	resolved := cfg.ResolveCompatibilityContext()
	if resolved.TrackerBackend.DeprecationMarker != CompatibilityDeprecationMarkerLegacyTrackerBackendFallback {
		t.Fatalf("TrackerBackend.DeprecationMarker = %q, want %q", resolved.TrackerBackend.DeprecationMarker, CompatibilityDeprecationMarkerLegacyTrackerBackendFallback)
	}
}

func TestResolveCompatibilityContext_FullPrecedenceMatrix(t *testing.T) {
	testCases := []struct {
		name      string
		config    Config
		wantProfs map[string]string // field -> (value,source,marker)
	}{
		{
			name: "all legacy defaults",
			config: Config{
				Project:    ProjectConfig{},
				Tracker:    TrackerConfig{},
				Methodology: MethodologyConfig{},
			},
			wantProfs: map[string]string{
				"profile.value": "go",
				"profile.source": string(CompatibilitySourceLegacyFallback),
				"profile.marker": CompatibilityDeprecationMarkerLegacyHardcodedDefaults,
				"tracker.value": "bd",
				"tracker.source": string(CompatibilitySourceLegacyFallback),
				"tracker.marker": CompatibilityDeprecationMarkerLegacyTrackerBackendFallback,
				"adapter.value": "go",
				"adapter.source": string(CompatibilitySourceLegacyFallback),
				"adapter.marker": CompatibilityDeprecationMarkerLegacyHardcodedDefaults,
			},
		},
		{
			name: "explicit profile only",
			config: Config{
				Project:    ProjectConfig{Profile: "python"},
				Tracker:    TrackerConfig{},
				Methodology: MethodologyConfig{},
			},
			wantProfs: map[string]string{
				"profile.value": "python",
				"profile.source": string(CompatibilitySourceExplicit),
				"profile.marker": "",
				"tracker.value": "bd",
				"tracker.source": string(CompatibilitySourceProfileDefault),
				"tracker.marker": CompatibilityDeprecationMarkerLegacyTrackerBackendFallback,
				"adapter.value": "go",
				"adapter.source": string(CompatibilitySourceProfileDefault),
				"adapter.marker": CompatibilityDeprecationMarkerLegacyHardcodedDefaults,
			},
		},
		{
			name: "explicit profile and tracker",
			config: Config{
				Project:    ProjectConfig{Profile: "node"},
				Tracker:    TrackerConfig{Backend: "linear"},
				Methodology: MethodologyConfig{},
			},
			wantProfs: map[string]string{
				"profile.value": "node",
				"profile.source": string(CompatibilitySourceExplicit),
				"profile.marker": "",
				"tracker.value": "linear",
				"tracker.source": string(CompatibilitySourceExplicit),
				"tracker.marker": "",
				"adapter.value": "go",
				"adapter.source": string(CompatibilitySourceProfileDefault),
				"adapter.marker": CompatibilityDeprecationMarkerLegacyHardcodedDefaults,
			},
		},
		{
			name: "all explicit",
			config: Config{
				Project:    ProjectConfig{Profile: "go"},
				Tracker:    TrackerConfig{Backend: "bd"},
				Methodology: MethodologyConfig{Adapter: "python"},
			},
			wantProfs: map[string]string{
				"profile.value": "go",
				"profile.source": string(CompatibilitySourceExplicit),
				"profile.marker": "",
				"tracker.value": "bd",
				"tracker.source": string(CompatibilitySourceExplicit),
				"tracker.marker": "",
				"adapter.value": "python",
				"adapter.source": string(CompatibilitySourceExplicit),
				"adapter.marker": "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolved := tc.config.ResolveCompatibilityContext()

			checks := []struct {
				path  string
				got   string
				want  string
			}{
				{"profile.value", resolved.Profile.Value, tc.wantProfs["profile.value"]},
				{"profile.source", string(resolved.Profile.Source), tc.wantProfs["profile.source"]},
				{"profile.marker", resolved.Profile.DeprecationMarker, tc.wantProfs["profile.marker"]},
				{"tracker.value", resolved.TrackerBackend.Value, tc.wantProfs["tracker.value"]},
				{"tracker.source", string(resolved.TrackerBackend.Source), tc.wantProfs["tracker.source"]},
				{"tracker.marker", resolved.TrackerBackend.DeprecationMarker, tc.wantProfs["tracker.marker"]},
				{"adapter.value", resolved.MethodologyAdapter.Value, tc.wantProfs["adapter.value"]},
				{"adapter.source", string(resolved.MethodologyAdapter.Source), tc.wantProfs["adapter.source"]},
				{"adapter.marker", resolved.MethodologyAdapter.DeprecationMarker, tc.wantProfs["adapter.marker"]},
			}

			for _, check := range checks {
				if check.got != check.want {
					t.Errorf("%s: got %q, want %q", check.path, check.got, check.want)
				}
			}
		})
	}
}
