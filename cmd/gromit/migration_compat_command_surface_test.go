package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestDebugCompatibilityDiagnostics_CommandSurfaceSupportsLegacyAndExplicitConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		fixture        string
		wantSubstrings []string
		wantAbsent     []string
	}{
		{
			name:    "legacy fixture surfaces fallback diagnostics",
			fixture: "legacy_command_surface.yaml",
			wantSubstrings: []string{
				"Profile:  go (source: legacy_fallback)",
				"Backend:  bd (source: legacy_fallback)",
				"Adapter:  go (source: legacy_fallback)",
				config.CompatibilityDeprecationMarkerLegacyHardcodedDefaults,
				config.CompatibilityDeprecationMarkerLegacyTrackerBackendFallback,
				config.CompatibilityStrictDefaultCutoverDate,
			},
		},
		{
			name:    "explicit fixture surfaces explicit diagnostics",
			fixture: "new_command_surface.yaml",
			wantSubstrings: []string{
				"Profile:  go (source: explicit)",
				"Backend:  bd (source: explicit)",
				"Adapter:  go (source: explicit)",
			},
			wantAbsent: []string{
				config.CompatibilityDeprecationMarkerLegacyHardcodedDefaults,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repoRoot, err := getRepoRoot()
			if err != nil {
				t.Fatalf("getRepoRoot error = %v", err)
			}
			cfgPath := filepath.Join(repoRoot, "test", "fixtures", "migration", tc.fixture)
			cfg, err := config.Load(cfgPath)
			if err != nil {
				t.Fatalf("Load(%q) error = %v", cfgPath, err)
			}

			got := formatDebugCompatibilityDiagnostics(cfg)
			for _, want := range tc.wantSubstrings {
				if !strings.Contains(got, want) {
					t.Fatalf("diagnostics missing %q\nfull output:\n%s", want, got)
				}
			}
			for _, unwanted := range tc.wantAbsent {
				if strings.Contains(got, unwanted) {
					t.Fatalf("diagnostics unexpectedly included %q\nfull output:\n%s", unwanted, got)
				}
			}
		})
	}
}
