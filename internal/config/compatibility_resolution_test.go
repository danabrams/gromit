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
