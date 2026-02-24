package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestDebugCompatibilityDiagnostics_CommandSurfaceSupportsLegacyAndExplicitConfig(t *testing.T) {
	tests := []struct {
		name            string
		fixture         string
		wantSubstrings  []string
	}{
		{
			name:    "legacy fixture surfaces fallback diagnostics",
			fixture: "legacy_command_surface.yaml",
			wantSubstrings: []string{
				"Profile:  go (source: legacy_fallback)",
				"Backend:  bd (source: legacy_fallback)",
				"Adapter:  go (source: legacy_fallback)",
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
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfgPath := filepath.Join("..", "..", "test", "fixtures", "migration", tc.fixture)
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
		})
	}
}
